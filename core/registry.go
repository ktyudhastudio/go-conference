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
	track     *webrtc.TrackLocalStaticRTP
}

// TrackRegistry maps (meetingID, userID) of a publisher to the local track that
// subscribers attach to. Subscribers may wait before the publisher connects.
type TrackRegistry struct {
	mu      sync.Mutex
	entries map[string]*entry
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

// Publish registers the track and wakes waiting subscribers.
func (r *TrackRegistry) Publish(meetingID, userID string, t *webrtc.TrackLocalStaticRTP) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e := r.getOrCreate(meetingID, userID)
	if e.track != nil { // republish -> fresh entry
		e = &entry{meetingID: meetingID, userID: userID, ready: make(chan struct{})}
		r.entries[key(meetingID, userID)] = e
	}
	e.track = t
	close(e.ready)
}

// Unpublish removes the entry if it still holds track t (publisher disconnected).
func (r *TrackRegistry) Unpublish(meetingID, userID string, t *webrtc.TrackLocalStaticRTP) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e, ok := r.entries[key(meetingID, userID)]; ok && e.track == t {
		delete(r.entries, key(meetingID, userID))
	}
}

// Wait blocks until the publisher's track is available or ctx is done.
func (r *TrackRegistry) Wait(ctx context.Context, meetingID, userID string) (*webrtc.TrackLocalStaticRTP, error) {
	r.mu.Lock()
	e := r.getOrCreate(meetingID, userID)
	r.mu.Unlock()

	select {
	case <-e.ready:
		return e.track, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
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
