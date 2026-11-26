package services

import "go-conference/core"

// PresenceService exposes meeting membership (who is in the room, camera/mic state).
type PresenceService struct {
	hub *core.PresenceHub
}

func NewPresenceService(hub *core.PresenceHub) *PresenceService {
	return &PresenceService{hub: hub}
}

// Join registers a live session for userID. Observers (admin) are not listed.
func (s *PresenceService) Join(meetingID, userID string, observer bool) (*core.Session, error) {
	return s.hub.Join(meetingID, userID, observer)
}

func (s *PresenceService) State(meetingID string) core.State {
	return s.hub.Snapshot(meetingID)
}

// ActiveMeetings lists meetings that currently have participants.
func (s *PresenceService) ActiveMeetings() []core.MeetingInfo {
	return s.hub.Meetings()
}
