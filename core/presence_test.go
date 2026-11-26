package core

import (
	"testing"
	"time"
)

func join(t *testing.T, h *PresenceHub, m, u string, obs bool) *Session {
	t.Helper()
	s, err := h.Join(m, u, obs)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func recv(t *testing.T, s *Session) []Participant {
	t.Helper()
	select {
	case p := <-s.Updates():
		return p.Participants
	case <-time.After(time.Second):
		t.Fatal("no update")
		return nil
	}
}

func TestPresenceJoinLeaveAndMedia(t *testing.T) {
	reg := NewTrackRegistry()
	h := NewPresenceHub(reg)

	a := join(t, h, "m", "a", false)
	if p := recv(t, a); len(p) != 1 || p[0].UserID != "a" {
		t.Fatalf("got %v", p)
	}

	b := join(t, h, "m", "b", false)
	if p := recv(t, a); len(p) != 2 {
		t.Fatalf("a should see b: %v", p)
	}
	_ = recv(t, b)

	b.SetMedia(true, false)
	p := recv(t, a)
	if !p[1].Camera || p[1].Mic {
		t.Fatalf("media not propagated: %v", p)
	}

	b.Close()
	if p := recv(t, a); len(p) != 1 || p[0].UserID != "a" {
		t.Fatalf("b should be gone: %v", p)
	}
}

func TestPresenceObserverNotListedAndMeetingsIsolated(t *testing.T) {
	h := NewPresenceHub(NewTrackRegistry())
	obs := join(t, h, "m", "admin", true)
	defer obs.Close()
	x := join(t, h, "other", "x", false)
	defer x.Close()

	if got := h.Snapshot("m").Participants; len(got) != 0 {
		t.Fatalf("observer must not be listed: %v", got)
	}
	a := join(t, h, "m", "a", false)
	defer a.Close()
	if p := recv(t, obs); len(p) != 1 || p[0].UserID != "a" {
		t.Fatalf("observer should see a: %v", p)
	}
}

func TestPresenceTwoTabsSameUser(t *testing.T) {
	h := NewPresenceHub(NewTrackRegistry())
	a1 := join(t, h, "m", "a", false)
	a2 := join(t, h, "m", "a", false)
	a1.Close()
	if got := h.Snapshot("m").Participants; len(got) != 1 {
		t.Fatalf("still present via 2nd tab: %v", got)
	}
	a2.Close()
	if got := h.Snapshot("m").Participants; len(got) != 0 {
		t.Fatalf("expected empty: %v", got)
	}
}

func TestPresencePublishingFlagFollowsRegistry(t *testing.T) {
	reg := NewTrackRegistry()
	h := NewPresenceHub(reg)
	a := join(t, h, "m", "a", false)
	defer a.Close()
	_ = recv(t, a)

	tr := newTrack(t)
	reg.Publish("m", "a", tr)
	if p := recv(t, a); !p[0].Publishing {
		t.Fatalf("expected publishing: %v", p)
	}
	reg.Unpublish("m", "a", tr)
	if p := recv(t, a); p[0].Publishing {
		t.Fatalf("expected not publishing: %v", p)
	}
}

func nextMsg(t *testing.T, s *Session) any {
	t.Helper()
	select {
	case m := <-s.Messages():
		return m
	case <-time.After(time.Second):
		t.Fatal("no message")
		return nil
	}
}

func TestChatBroadcastAndHistory(t *testing.T) {
	h := NewPresenceHub(NewTrackRegistry())
	a := join(t, h, "m", "a", false)
	defer a.Close()
	b := join(t, h, "m", "b", false)
	defer b.Close()

	a.Chat("  halo  ")
	for _, s := range []*Session{a, b} {
		msg, ok := nextMsg(t, s).(ChatMessage)
		if !ok || msg.From != "a" || msg.Text != "halo" {
			t.Fatalf("got %+v", msg)
		}
	}

	late := join(t, h, "m", "c", false)
	defer late.Close()
	hist, ok := nextMsg(t, late).([]ChatMessage)
	if !ok || len(hist) != 1 || hist[0].Text != "halo" {
		t.Fatalf("history: %+v", hist)
	}
}

func TestAdminCommandsAndKick(t *testing.T) {
	h := NewPresenceHub(NewTrackRegistry())
	admin := join(t, h, "m", "admin", true)
	defer admin.Close()
	a := join(t, h, "m", "a", false)

	if err := a.AdminAction("kick", "a"); err != ErrForbidden {
		t.Fatalf("participant must not run admin actions: %v", err)
	}
	if err := admin.AdminAction("mute_mic", "a"); err != nil {
		t.Fatal(err)
	}
	if c, ok := nextMsg(t, a).(Command); !ok || c.Action != "mute_mic" {
		t.Fatalf("got %+v", c)
	}
	if err := admin.AdminAction("explode", "a"); err != ErrBadAction {
		t.Fatalf("got %v", err)
	}

	if err := admin.AdminAction("kick", "a"); err != nil {
		t.Fatal(err)
	}
	if c, ok := nextMsg(t, a).(Command); !ok || c.Action != "kick" {
		t.Fatalf("got %+v", c)
	}
	select {
	case <-a.Kicked():
	default:
		t.Fatal("session should be kicked")
	}
	a.Close()

	if _, err := h.Join("m", "a", false); err != ErrBlocked {
		t.Fatalf("blocked user must not rejoin: %v", err)
	}
	if st := h.Snapshot("m"); len(st.Blocked) != 1 || st.Blocked[0] != "a" {
		t.Fatalf("blocked list: %v", st.Blocked)
	}
	if err := admin.AdminAction("allow", "a"); err != nil {
		t.Fatal(err)
	}
	if s, err := h.Join("m", "a", false); err != nil {
		t.Fatalf("allowed user can rejoin: %v", err)
	} else {
		s.Close()
	}
}

func TestActiveMeetingsListsOnlyMeetingsWithParticipants(t *testing.T) {
	h := NewPresenceHub(NewTrackRegistry())
	obs := join(t, h, "watched", "admin", true) // hanya admin: tidak aktif
	defer obs.Close()
	a := join(t, h, "m1", "a", false)
	defer a.Close()
	b := join(t, h, "m2", "b", false)
	defer b.Close()
	c := join(t, h, "m2", "c", false)
	defer c.Close()

	got := h.Meetings()
	if len(got) != 2 || got[0].ID != "m2" || got[0].Participants != 2 || got[1].ID != "m1" {
		t.Fatalf("got %+v", got)
	}
	if got[0].StartedAt == 0 {
		t.Fatal("startedAt must be set")
	}

	b.Close()
	c.Close()
	if got := h.Meetings(); len(got) != 1 || got[0].ID != "m1" {
		t.Fatalf("ended meeting must disappear: %+v", got)
	}
}

func TestQualityShownOnlyWhilePublishing(t *testing.T) {
	reg := NewTrackRegistry()
	h := NewPresenceHub(reg)
	a := join(t, h, "m", "a", false)
	defer a.Close()
	_ = recv(t, a)

	h.SetQuality("m", "a", Quality{RTTMs: 40, Level: QualityGood})
	if p := recv(t, a); p[0].Quality != nil {
		t.Fatalf("quality must be hidden while not publishing: %+v", p[0].Quality)
	}

	tr := newTrack(t)
	reg.Publish("m", "a", tr)
	if p := recv(t, a); p[0].Quality == nil || p[0].Quality.RTTMs != 40 {
		t.Fatalf("expected quality: %+v", p[0].Quality)
	}
	h.SetQuality("m", "a", Quality{RTTMs: 90, Level: QualityGood})
	if p := recv(t, a); p[0].Quality.RTTMs != 90 {
		t.Fatalf("expected update: %+v", p[0].Quality)
	}

	reg.Unpublish("m", "a", tr)
	if p := recv(t, a); p[0].Quality != nil {
		t.Fatalf("stale quality must be hidden: %+v", p[0].Quality)
	}
}
