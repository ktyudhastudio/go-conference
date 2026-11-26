package services

import (
	"context"
	"testing"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"

	"go-conference/config"
	"go-conference/core"
)

func newService(t *testing.T) (*SdpService, *webrtc.API) {
	t.Helper()
	api, err := core.NewWebRTCAPI()
	if err != nil {
		t.Fatal(err)
	}
	return NewSdpService(api, core.PeerConfig(config.Config{StunServer: "stun:stun.l.google.com:19302"}), core.NewTrackRegistry()), api
}

func TestNegotiateInvalidOffer(t *testing.T) {
	s, _ := newService(t)
	if _, err := s.Negotiate(context.Background(), true, "m", "a", "b", "%%%"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSubscriberCancelledWhilePublisherAbsent(t *testing.T) {
	s, api := newService(t)
	offer := clientOffer(t, api, false, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := s.Negotiate(ctx, false, "m", "sub", "ghost", offer.encoded); err == nil {
		t.Fatal("expected context error")
	}
}

// End-to-end: a pion client publishes VP8, a second client subscribes and receives RTP.
func TestPublishAndSubscribeEndToEnd(t *testing.T) {
	s, api := newService(t)

	// --- publisher ---
	track, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8}, "video", "pub")
	if err != nil {
		t.Fatal(err)
	}
	pub := clientOffer(t, api, true, track)
	defer pub.pc.Close()
	answer, err := s.Negotiate(context.Background(), true, "m", "pub", "", pub.encoded)
	if err != nil {
		t.Fatal(err)
	}
	applyAnswer(t, pub.pc, answer)

	stop := make(chan struct{})
	defer close(stop)
	go func() {
		tk := time.NewTicker(30 * time.Millisecond)
		defer tk.Stop()
		for {
			select {
			case <-stop:
				return
			case <-tk.C:
				_ = track.WriteSample(media.Sample{Data: []byte{0x10, 0x02, 0x00, 0x9d, 0x01, 0x2a, 1, 2, 3, 4}, Duration: 30 * time.Millisecond})
			}
		}
	}()

	// --- subscriber ---
	sub := clientOffer(t, api, false, nil)
	defer sub.pc.Close()
	got := make(chan struct{}, 1)
	sub.pc.OnTrack(func(tr *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		if _, _, err := tr.ReadRTP(); err == nil {
			select {
			case got <- struct{}{}:
			default:
			}
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ans, err := s.Negotiate(ctx, false, "m", "sub", "pub", sub.encoded)
	if err != nil {
		t.Fatal(err)
	}
	applyAnswer(t, sub.pc, ans)

	select {
	case <-got:
	case <-time.After(10 * time.Second):
		t.Fatal("subscriber received no RTP")
	}

	if peers := s.registry.List("m"); len(peers) != 1 || peers[0] != "pub" {
		t.Fatalf("unexpected peers: %v", peers)
	}
	if peers := s.registry.List("other"); len(peers) != 0 {
		t.Fatalf("meetings must be isolated: %v", peers)
	}
}

type offerResult struct {
	pc      *webrtc.PeerConnection
	encoded string
}

func clientOffer(t *testing.T, api *webrtc.API, send bool, track webrtc.TrackLocal) offerResult {
	t.Helper()
	pc, err := api.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatal(err)
	}
	if send {
		if _, err := pc.AddTrack(track); err != nil {
			t.Fatal(err)
		}
	} else if _, err := pc.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo,
		webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly}); err != nil {
		t.Fatal(err)
	}
	off, err := pc.CreateOffer(nil)
	if err != nil {
		t.Fatal(err)
	}
	gathered := webrtc.GatheringCompletePromise(pc)
	if err := pc.SetLocalDescription(off); err != nil {
		t.Fatal(err)
	}
	<-gathered
	enc, err := core.Encode(pc.LocalDescription())
	if err != nil {
		t.Fatal(err)
	}
	return offerResult{pc: pc, encoded: enc}
}

func applyAnswer(t *testing.T, pc *webrtc.PeerConnection, encoded string) {
	t.Helper()
	var ans webrtc.SessionDescription
	if err := core.Decode(encoded, &ans); err != nil {
		t.Fatal(err)
	}
	if err := pc.SetRemoteDescription(ans); err != nil {
		t.Fatal(err)
	}
}
