package services

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v4"

	"go-conference/core"
)

const (
	rtcpPLIInterval = 3 * time.Second
	disconnectGrace = 5 * time.Second  // tutup PC jika "disconnected" selama ini
	connectTimeout  = 20 * time.Second // tutup PC yang tak pernah terhubung
)

// watchConnection menutup PeerConnection yang mati (ICE failed/disconnected lama/tidak pernah
// connect). Tanpa ini pion tidak menutup sendiri, remote.Read menggantung, dan publisher yang
// sudah putus tetap terdaftar di registry.
func watchConnection(pc *webrtc.PeerConnection, onConnected, onClosed func()) {
	var mu sync.Mutex
	var grace *time.Timer
	cancelGrace := func() {
		if grace != nil {
			grace.Stop()
			grace = nil
		}
	}

	connectTimer := time.AfterFunc(connectTimeout, func() {
		if pc.ConnectionState() != webrtc.PeerConnectionStateConnected {
			_ = pc.Close()
		}
	})

	pc.OnConnectionStateChange(func(st webrtc.PeerConnectionState) {
		mu.Lock()
		defer mu.Unlock()
		switch st {
		case webrtc.PeerConnectionStateConnected:
			connectTimer.Stop()
			cancelGrace()
			if onConnected != nil {
				onConnected()
			}
		case webrtc.PeerConnectionStateDisconnected:
			cancelGrace()
			grace = time.AfterFunc(disconnectGrace, func() {
				if pc.ConnectionState() == webrtc.PeerConnectionStateDisconnected {
					_ = pc.Close()
				}
			})
		case webrtc.PeerConnectionStateFailed:
			cancelGrace()
			_ = pc.Close()
		case webrtc.PeerConnectionStateClosed:
			connectTimer.Stop()
			cancelGrace()
			if onClosed != nil {
				onClosed()
			}
		}
	})
}

// SdpService negotiates WebRTC sessions (SFU-style forwarding).
type SdpService struct {
	api       *webrtc.API
	config    webrtc.Configuration
	registry  *core.TrackRegistry
	onQuality func(meetingID, userID string, q core.Quality)
}

// SetQualityReporter registers where publisher connection measurements go (the presence hub).
func (s *SdpService) SetQualityReporter(fn func(meetingID, userID string, q core.Quality)) {
	s.onQuality = fn
}

func NewSdpService(api *webrtc.API, cfg webrtc.Configuration, reg *core.TrackRegistry) *SdpService {
	return &SdpService{api: api, config: cfg, registry: reg}
}

// Negotiate handles an SDP offer and returns the encoded SDP answer.
// isSender=true: userID publishes video. Otherwise the caller subscribes to peerID.
func (s *SdpService) Negotiate(ctx context.Context, isSender bool, meetingID, userID, peerID, encodedOffer string) (string, error) {
	var offer webrtc.SessionDescription
	if err := core.Decode(encodedOffer, &offer); err != nil {
		return "", errors.New("invalid sdp: " + err.Error())
	}

	pc, err := s.api.NewPeerConnection(s.config)
	if err != nil {
		return "", err
	}

	if isSender {
		err = s.setupPublisher(pc, meetingID, userID)
	} else {
		watchConnection(pc, nil, nil)
		err = s.setupSubscriber(ctx, pc, meetingID, peerID)
	}
	if err != nil {
		pc.Close()
		return "", err
	}

	if err := pc.SetRemoteDescription(offer); err != nil {
		pc.Close()
		return "", err
	}
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		pc.Close()
		return "", err
	}
	gathered := webrtc.GatheringCompletePromise(pc)
	if err := pc.SetLocalDescription(answer); err != nil {
		pc.Close()
		return "", err
	}
	<-gathered

	return core.Encode(pc.LocalDescription())
}

func (s *SdpService) setupSubscriber(ctx context.Context, pc *webrtc.PeerConnection, meetingID, peerID string) error {
	video, audio, err := s.registry.WaitTracks(ctx, meetingID, peerID)
	if err != nil {
		return err
	}
	for _, track := range []*webrtc.TrackLocalStaticRTP{video, audio} {
		if track == nil {
			continue
		}
		sender, err := pc.AddTrack(track)
		if err != nil {
			return err
		}
		// drain RTCP so interceptors keep working
		go func() {
			buf := make([]byte, 1500)
			for {
				if _, _, err := sender.Read(buf); err != nil {
					return
				}
			}
		}()
	}
	return nil
}

// setupPublisher accepts video + audio from the publisher. Local tracks are created
// up front and registered once the connection is up, so a participant is subscribable
// even while their camera is off (the client then sends placeholder media).
func (s *SdpService) setupPublisher(pc *webrtc.PeerConnection, meetingID, userID string) error {
	for _, kind := range []webrtc.RTPCodecType{webrtc.RTPCodecTypeVideo, webrtc.RTPCodecTypeAudio} {
		if _, err := pc.AddTransceiverFromKind(kind,
			webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly}); err != nil {
			return err
		}
	}

	video, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8, ClockRate: 90000}, "video", userID)
	if err != nil {
		return err
	}
	audio, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 2}, "audio", userID)
	if err != nil {
		return err
	}

	var once sync.Once // "connected" can fire again after a brief disconnect
	watchConnection(pc,
		func() {
			once.Do(func() {
				s.registry.PublishAll(meetingID, userID, video, audio)
				if s.onQuality != nil {
					monitorQuality(pc, func(q core.Quality) { s.onQuality(meetingID, userID, q) })
				}
			})
		},
		func() { s.registry.Unpublish(meetingID, userID, video) },
	)

	pc.OnTrack(func(remote *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		local := video
		if remote.Kind() == webrtc.RTPCodecTypeAudio {
			local = audio
		} else {
			// periodically request keyframes
			go func() {
				t := time.NewTicker(rtcpPLIInterval)
				defer t.Stop()
				for range t.C {
					if err := pc.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: uint32(remote.SSRC())}}); err != nil {
						return
					}
				}
			}()
		}

		buf := make([]byte, 1500)
		for {
			n, _, err := remote.Read(buf)
			if err != nil {
				return
			}
			if _, err := local.Write(buf[:n]); err != nil && !errors.Is(err, io.ErrClosedPipe) {
				return
			}
		}
	})
	return nil
}
