package services

import "go-conference/core"

// PeerService exposes who is currently publishing in a meeting.
type PeerService struct {
	registry *core.TrackRegistry
}

func NewPeerService(reg *core.TrackRegistry) *PeerService {
	return &PeerService{registry: reg}
}

func (s *PeerService) ListPublishers(meetingID string) []string {
	return s.registry.List(meetingID)
}
