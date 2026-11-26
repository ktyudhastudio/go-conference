package core

import (
	"context"
	"sort"
	"sync"

	"github.com/pion/webrtc/v4"
)

type entry struct {
	meetingID string
	userID    string
	ready     chan struct{}
	track     *webrtc.TrackLocalStaticRTP // video (penanda publish)
	audio     *webrtc.TrackLocalStaticRTP // opsional
}

// TrackRegistry maps (meetingID, userID) of a publisher to the local track that
// subscribers attach to. Subscribers may wait before the publisher connects.
type TrackRegistry struct {
	mu       sync.Mutex
	entries  map[string]*entry
	onChange func(meetingID string)
}

// SetOnChange registers a callback fired (outside the lock) after a publisher is
// added or removed. Used by the presence hub to push fresh state.
func (r *TrackRegistry) SetOnChange(fn func(meetingID string)) {
	r.mu.Lock()
	r.onChange = fn
	r.mu.Unlock()
}

func (r *TrackRegistry) notify(meetingID string) {
	r.mu.Lock()
	fn := r.onChange
	r.mu.Unlock()
	if fn != nil {
		fn(meetingID)
	}
}

func NewTrackRegistry() *TrackRegistry {
	return &TrackRegistry{entries: map[string]*entry{}}
}

func key(meetingID, userID string) string { return meetingID + "/" + userID }

func (r *TrackRegistry) getOrCreate(meetingID, userID string) *entry {
	k := key(meetingID, userID)
	e, ok := r.entries[k]
	if !ok {
		e = &entry{meetingID: meetingID, userID: userID, ready: make(chan struct{})}
		r.entries[k] = e
	}
	return e
}

// Publish registers the video track and wakes waiting subscribers.
func (r *TrackRegistry) Publish(meetingID, userID string, t *webrtc.TrackLocalStaticRTP) {
	r.PublishAll(meetingID, userID, t, nil)
}

// PublishAll registers video (+ optional audio) and wakes waiting subscribers.
func (r *TrackRegistry) PublishAll(meetingID, userID string, video, audio *webrtc.TrackLocalStaticRTP) {
	defer r.notify(meetingID)
	r.mu.Lock()
	defer r.mu.Unlock()
	t := video
	e := r.getOrCreate(meetingID, userID)
	if e.track != nil { // republish -> fresh entry
		e = &entry{meetingID: meetingID, userID: userID, ready: make(chan struct{})}
		r.entries[key(meetingID, userID)] = e
	}
	e.track = t
	e.audio = audio
	close(e.ready)
}

// Unpublish removes the entry if it still holds track t (publisher disconnected).
func (r *TrackRegistry) Unpublish(meetingID, userID string, t *webrtc.TrackLocalStaticRTP) {
	r.mu.Lock()
	removed := false
	if e, ok := r.entries[key(meetingID, userID)]; ok && e.track == t {
		delete(r.entries, key(meetingID, userID))
		removed = true
	}
	r.mu.Unlock()
	if removed {
		r.notify(meetingID)
	}
}

// Wait blocks until the publisher's track is available or ctx is done.
func (r *TrackRegistry) Wait(ctx context.Context, meetingID, userID string) (*webrtc.TrackLocalStaticRTP, error) {
	v, _, err := r.WaitTracks(ctx, meetingID, userID)
	return v, err
}

// WaitTracks is like Wait but also returns the audio track (nil if the publisher has none).
func (r *TrackRegistry) WaitTracks(ctx context.Context, meetingID, userID string) (video, audio *webrtc.TrackLocalStaticRTP, err error) {
	r.mu.Lock()
	e := r.getOrCreate(meetingID, userID)
	r.mu.Unlock()

	select {
	case <-e.ready:
		return e.track, e.audio, nil
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}
}

// IsPublishing reports whether userID has an active publisher in the meeting.
func (r *TrackRegistry) IsPublishing(meetingID, userID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.entries[key(meetingID, userID)]
	return ok && e.track != nil
}

// List returns the sorted userIDs actively publishing in a meeting.
func (r *TrackRegistry) List(meetingID string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	peers := []string{}
	for _, e := range r.entries {
		if e.meetingID == meetingID && e.track != nil {
			peers = append(peers, e.userID)
		}
	}
	sort.Strings(peers)
	return peers
}
