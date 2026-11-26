package services

import (
	"context"
	"errors"
	"io"
	"log"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v4"

	"go-conference/core"
)

const rtcpPLIInterval = 3 * time.Second

// SdpService negotiates WebRTC sessions (SFU-style forwarding).
type SdpService struct {
	api      *webrtc.API
	config   webrtc.Configuration
	registry *core.TrackRegistry
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
	track, err := s.registry.Wait(ctx, meetingID, peerID)
	if err != nil {
		return err
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
	return nil
}

func (s *SdpService) setupPublisher(pc *webrtc.PeerConnection, meetingID, userID string) error {
	if _, err := pc.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo,
		webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly}); err != nil {
		return err
	}

	pc.OnTrack(func(remote *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
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

		local, err := webrtc.NewTrackLocalStaticRTP(remote.Codec().RTPCodecCapability, "video", "pion")
		if err != nil {
			log.Println("new local track:", err)
			return
		}
		s.registry.Publish(meetingID, userID, local)
		defer s.registry.Unpublish(meetingID, userID, local)

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
