package core

import (
	"context"
	"testing"
	"time"

	"github.com/pion/webrtc/v4"
)

func newTrack(t *testing.T) *webrtc.TrackLocalStaticRTP {
	t.Helper()
	tr, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8}, "video", "pion")
	if err != nil {
		t.Fatal(err)
	}
	return tr
}

func TestRegistryPublishBeforeWait(t *testing.T) {
	r := NewTrackRegistry()
	tr := newTrack(t)
	r.Publish("m", "a", tr)
	got, err := r.Wait(context.Background(), "m", "a")
	if err != nil || got != tr {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestRegistryWaitBeforePublish(t *testing.T) {
	r := NewTrackRegistry()
	tr := newTrack(t)
	done := make(chan *webrtc.TrackLocalStaticRTP, 1)
	go func() {
		got, _ := r.Wait(context.Background(), "m", "a")
		done <- got
	}()
	time.Sleep(50 * time.Millisecond)
	r.Publish("m", "a", tr)
	select {
	case got := <-done:
		if got != tr {
			t.Fatal("wrong track")
		}
	case <-time.After(time.Second):
		t.Fatal("Wait did not wake up")
	}
}

func TestRegistryWaitContextCancel(t *testing.T) {
	r := NewTrackRegistry()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := r.Wait(ctx, "m", "nobody"); err == nil {
		t.Fatal("expected context error")
	}
}

func TestRegistryRepublishReplaces(t *testing.T) {
	r := NewTrackRegistry()
	t1, t2 := newTrack(t), newTrack(t)
	r.Publish("m", "a", t1)
	r.Publish("m", "a", t2)
	got, _ := r.Wait(context.Background(), "m", "a")
	if got != t2 {
		t.Fatal("expected latest track")
	}
}

func TestRegistryListAndUnpublish(t *testing.T) {
	r := NewTrackRegistry()
	t1 := newTrack(t)
	r.Publish("m", "b", newTrack(t))
	r.Publish("m", "a", t1)
	r.Publish("x", "z", newTrack(t))
	if got := r.List("m"); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("got %v", got)
	}
	r.Unpublish("m", "a", newTrack(t)) // different track: ignored
	if len(r.List("m")) != 2 {
		t.Fatal("unpublish with wrong track must be ignored")
	}
	r.Unpublish("m", "a", t1)
	if got := r.List("m"); len(got) != 1 || got[0] != "b" {
		t.Fatalf("got %v", got)
	}
}
