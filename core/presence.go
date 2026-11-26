package core

import (
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	maxChatLen     = 1000
	maxChatHistory = 100
)

var (
	// ErrBlocked is returned by Join for a participant the admin removed.
	ErrBlocked = errors.New("participant is blocked from this meeting")
	// ErrForbidden is returned when a non-observer tries an admin action.
	ErrForbidden = errors.New("admin only")
	// ErrBadAction is returned for unknown admin actions.
	ErrBadAction = errors.New("unknown action")
)

// Connection quality levels.
const (
	QualityGood = "good"
	QualityFair = "fair"
	QualityPoor = "poor"
)

// Quality describes a participant's connection to the server (their upload).
type Quality struct {
	RTTMs       int     `json:"rttMs"`
	LossPct     float64 `json:"lossPct"`
	JitterMs    int     `json:"jitterMs"`
	BitrateKbps int     `json:"bitrateKbps"`
	Level       string  `json:"level"` // good | fair | poor
}

// Participant is a member of a meeting as seen by clients.
type Participant struct {
	UserID     string `json:"userId"`
	Camera     bool   `json:"camera"`
	Mic        bool   `json:"mic"`
	Publishing bool   `json:"publishing"` // media connection is live (subscribe-able)
	// Quality is the latest connection measurement; absent until the first sample.
	Quality *Quality `json:"quality,omitempty"`
}

// State is the room snapshot pushed to every session.
type State struct {
	Participants []Participant `json:"participants"`
	Blocked      []string      `json:"blocked"`
}

// ChatMessage is a chat line; history lives only while the meeting has sessions.
type ChatMessage struct {
	ID   int64  `json:"id"`
	From string `json:"from"`
	Text string `json:"text"`
	At   int64  `json:"at"` // unix ms
}

// Command is an admin instruction delivered to a participant's client.
type Command struct {
	Action string `json:"action"` // mute_mic | camera_off | kick
}

type member struct {
	conns   int
	camera  bool
	mic     bool
	quality *Quality
}

type meetingState struct {
	members  map[string]*member
	sessions map[*Session]struct{}
	blocked  map[string]struct{}
	history  []ChatMessage
	nextID   int64
	started  time.Time // saat peserta pertama bergabung
}

// MeetingInfo summarises an active meeting (at least one participant).
type MeetingInfo struct {
	ID           string `json:"id"`
	Participants int    `json:"participants"`
	StartedAt    int64  `json:"startedAt"` // unix ms
}

// PresenceHub tracks who is in each meeting (independent of whether they publish
// media), relays chat and admin commands, and pushes snapshots to sessions.
type PresenceHub struct {
	mu       sync.Mutex
	meetings map[string]*meetingState
	reg      *TrackRegistry
}

func NewPresenceHub(reg *TrackRegistry) *PresenceHub {
	h := &PresenceHub{meetings: map[string]*meetingState{}, reg: reg}
	reg.SetOnChange(h.Refresh)
	return h
}

// Session is one live connection (e.g. a websocket) to a meeting.
type Session struct {
	hub       *PresenceHub
	meetingID string
	userID    string
	observer  bool
	updates   chan State
	msgs      chan any // ChatMessage | Command | []ChatMessage (history)
	kicked    chan struct{}
	closeOnce sync.Once
	kickOnce  sync.Once
}

// Updates delivers state snapshots; only the latest is kept for slow readers.
func (s *Session) Updates() <-chan State { return s.updates }

// Messages delivers chat lines, history and commands in order.
func (s *Session) Messages() <-chan any { return s.msgs }

// Kicked is closed when the admin removed this participant.
func (s *Session) Kicked() <-chan struct{} { return s.kicked }

func (s *Session) push(v any) {
	select {
	case s.msgs <- v:
	default: // slow client: drop rather than block the room
	}
}

// Join registers a session. Observers (e.g. admin) receive state but are not listed.
func (h *PresenceHub) Join(meetingID, userID string, observer bool) (*Session, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	m := h.meetingLocked(meetingID)
	if _, blocked := m.blocked[userID]; blocked && !observer {
		h.cleanupLocked(meetingID)
		return nil, ErrBlocked
	}
	s := &Session{
		hub: h, meetingID: meetingID, userID: userID, observer: observer,
		updates: make(chan State, 1), msgs: make(chan any, 64), kicked: make(chan struct{}),
	}
	m.sessions[s] = struct{}{}
	if !observer {
		if len(m.members) == 0 {
			m.started = time.Now()
		}
		mem, ok := m.members[userID]
		if !ok {
			mem = &member{}
			m.members[userID] = mem
		}
		mem.conns++
	}
	if len(m.history) > 0 {
		s.push(append([]ChatMessage(nil), m.history...))
	}
	h.broadcastLocked(meetingID)
	return s, nil
}

// SetMedia records the camera/mic state reported by the participant.
func (s *Session) SetMedia(camera, mic bool) {
	if s.observer {
		return
	}
	h := s.hub
	h.mu.Lock()
	defer h.mu.Unlock()
	if m, ok := h.meetings[s.meetingID]; ok {
		if mem, ok := m.members[s.userID]; ok {
			mem.camera, mem.mic = camera, mic
			h.broadcastLocked(s.meetingID)
		}
	}
}

// SetQuality records a connection measurement for a participant (called by the media layer).
func (h *PresenceHub) SetQuality(meetingID, userID string, q Quality) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if m, ok := h.meetings[meetingID]; ok {
		if mem, ok := m.members[userID]; ok {
			mem.quality = &q
			h.broadcastLocked(meetingID)
		}
	}
}

// Chat relays a message to everyone in the meeting (admins included).
func (s *Session) Chat(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	if utf8.RuneCountInString(text) > maxChatLen {
		text = string([]rune(text)[:maxChatLen])
	}
	from := s.userID
	if s.observer {
		from = "Admin"
	}
	h := s.hub
	h.mu.Lock()
	defer h.mu.Unlock()
	m, ok := h.meetings[s.meetingID]
	if !ok {
		return
	}
	m.nextID++
	msg := ChatMessage{ID: m.nextID, From: from, Text: text, At: time.Now().UnixMilli()}
	m.history = append(m.history, msg)
	if len(m.history) > maxChatHistory {
		m.history = m.history[len(m.history)-maxChatHistory:]
	}
	for sess := range m.sessions {
		sess.push(msg)
	}
}

// AdminAction lets an observer control a participant: mute_mic, camera_off, kick, allow.
func (s *Session) AdminAction(action, target string) error {
	if !s.observer {
		return ErrForbidden
	}
	h := s.hub
	h.mu.Lock()
	defer h.mu.Unlock()
	m, ok := h.meetings[s.meetingID]
	if !ok {
		return nil
	}
	switch action {
	case "mute_mic", "camera_off":
		for sess := range m.sessions {
			if !sess.observer && sess.userID == target {
				sess.push(Command{Action: action})
			}
		}
	case "kick":
		m.blocked[target] = struct{}{}
		for sess := range m.sessions {
			if !sess.observer && sess.userID == target {
				sess.push(Command{Action: "kick"})
				sess.kickOnce.Do(func() { close(sess.kicked) })
			}
		}
		h.broadcastLocked(s.meetingID)
	case "allow":
		delete(m.blocked, target)
		h.broadcastLocked(s.meetingID)
	default:
		return ErrBadAction
	}
	return nil
}

// Close removes the session; the participant leaves once its last session closes.
func (s *Session) Close() {
	s.closeOnce.Do(func() {
		h := s.hub
		h.mu.Lock()
		defer h.mu.Unlock()
		m, ok := h.meetings[s.meetingID]
		if !ok {
			return
		}
		delete(m.sessions, s)
		if !s.observer {
			if mem, ok := m.members[s.userID]; ok {
				if mem.conns--; mem.conns <= 0 {
					delete(m.members, s.userID)
				}
			}
		}
		if h.cleanupLocked(s.meetingID) {
			return
		}
		h.broadcastLocked(s.meetingID)
	})
}

// Snapshot returns the current state of a meeting.
func (h *PresenceHub) Snapshot(meetingID string) State {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.snapshotLocked(meetingID)
}

// Meetings lists meetings that currently have participants, busiest first.
func (h *PresenceHub) Meetings() []MeetingInfo {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := []MeetingInfo{}
	for id, m := range h.meetings {
		if len(m.members) > 0 {
			out = append(out, MeetingInfo{ID: id, Participants: len(m.members), StartedAt: m.started.UnixMilli()})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Participants != out[j].Participants {
			return out[i].Participants > out[j].Participants
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// Refresh pushes a fresh snapshot (called when publishers change).
func (h *PresenceHub) Refresh(meetingID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.broadcastLocked(meetingID)
}

func (h *PresenceHub) meetingLocked(id string) *meetingState {
	m, ok := h.meetings[id]
	if !ok {
		m = &meetingState{
			members:  map[string]*member{},
			sessions: map[*Session]struct{}{},
			blocked:  map[string]struct{}{},
		}
		h.meetings[id] = m
	}
	return m
}

// cleanupLocked drops an empty meeting (history and blocklist end with it).
func (h *PresenceHub) cleanupLocked(id string) bool {
	if m, ok := h.meetings[id]; ok && len(m.sessions) == 0 {
		delete(h.meetings, id)
		return true
	}
	return false
}

func (h *PresenceHub) snapshotLocked(meetingID string) State {
	st := State{Participants: []Participant{}, Blocked: []string{}}
	if m, ok := h.meetings[meetingID]; ok {
		for id, mem := range m.members {
			publishing := h.reg.IsPublishing(meetingID, id)
			p := Participant{UserID: id, Camera: mem.camera, Mic: mem.mic, Publishing: publishing}
			if publishing { // stale numbers are hidden once the media connection ends
				p.Quality = mem.quality
			}
			st.Participants = append(st.Participants, p)
		}
		for id := range m.blocked {
			st.Blocked = append(st.Blocked, id)
		}
	}
	sort.Slice(st.Participants, func(i, j int) bool { return st.Participants[i].UserID < st.Participants[j].UserID })
	sort.Strings(st.Blocked)
	return st
}

func (h *PresenceHub) broadcastLocked(meetingID string) {
	m, ok := h.meetings[meetingID]
	if !ok {
		return
	}
	snap := h.snapshotLocked(meetingID)
	for s := range m.sessions {
		select { // replace any stale undelivered snapshot with the latest
		case <-s.updates:
		default:
		}
		select {
		case s.updates <- snap:
		default:
		}
	}
}
