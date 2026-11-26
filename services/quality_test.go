package services

import (
	"context"
	"testing"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"

	"go-conference/core"
)

func TestGradeQuality(t *testing.T) {
	cases := []struct {
		rtt    int
		loss   float64
		jitter int
		want   string
	}{
		{30, 0, 5, core.QualityGood},
		{250, 0, 5, core.QualityFair},
		{30, 4, 5, core.QualityFair},
		{30, 0, 40, core.QualityFair},
		{500, 0, 5, core.QualityPoor},
		{30, 9, 5, core.QualityPoor},
		{30, 0, 80, core.QualityPoor},
		{250, 9, 5, core.QualityPoor}, // worst metric wins
	}
	for _, c := range cases {
		if got := gradeQuality(c.rtt, c.loss, c.jitter); got != c.want {
			t.Errorf("rtt=%d loss=%.1f jitter=%d: got %s want %s", c.rtt, c.loss, c.jitter, got, c.want)
		}
	}
}

// A connected publisher must produce quality reports.
func TestPublisherQualityIsReported(t *testing.T) {
	old := qualityInterval
	qualityInterval = 100 * time.Millisecond
	defer func() { qualityInterval = old }()

	s, api := newService(t)
	got := make(chan core.Quality, 8)
	s.SetQualityReporter(func(meetingID, userID string, q core.Quality) {
		if meetingID == "m" && userID == "pub" {
			select {
			case got <- q:
			default:
			}
		}
	})

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

	select {
	case q := <-got:
		if q.Level != core.QualityGood && q.Level != core.QualityFair && q.Level != core.QualityPoor {
			t.Fatalf("invalid level %q", q.Level)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("no quality report")
	}
}
