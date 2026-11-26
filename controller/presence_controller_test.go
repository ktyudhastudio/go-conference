package controller

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"go-conference/core"
	"go-conference/services"
)

func dial(t *testing.T, srv *httptest.Server, query string) *websocket.Conn {
	t.Helper()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/m/m1?" + query
	c, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// readState skips non-state messages (chat/history/command).
func readState(t *testing.T, c *websocket.Conn) []core.Participant {
	t.Helper()
	for {
		_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
		var raw map[string]json.RawMessage
		if err := c.ReadJSON(&raw); err != nil {
			t.Fatal(err)
		}
		if string(raw["type"]) != `"state"` {
			continue
		}
		var parts []core.Participant
		_ = json.Unmarshal(raw["participants"], &parts)
		return parts
	}
}

func readType(t *testing.T, c *websocket.Conn, want string) map[string]json.RawMessage {
	t.Helper()
	for {
		_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
		var raw map[string]json.RawMessage
		if err := c.ReadJSON(&raw); err != nil {
			t.Fatal(err)
		}
		if string(raw["type"]) == `"`+want+`"` {
			return raw
		}
	}
}

func TestSocketJoinMediaAndLeave(t *testing.T) {
	gin.SetMode(gin.TestMode)
	hub := core.NewPresenceHub(core.NewTrackRegistry())
	r := gin.New()
	r.GET("/ws/m/:meetingId", NewPresenceController(services.NewPresenceService(hub)).Socket)
	srv := httptest.NewServer(r)
	defer srv.Close()

	admin := dial(t, srv, "userId=admin&observer=true")
	defer admin.Close()
	if p := readState(t, admin); len(p) != 0 {
		t.Fatalf("observer joins empty room: %v", p)
	}

	a := dial(t, srv, "userId=a")
	if p := readState(t, admin); len(p) != 1 || p[0].UserID != "a" {
		t.Fatalf("admin should see a: %v", p)
	}

	// kamera mati, mic nyala: peserta tetap terdaftar (kartu tetap tampil)
	if err := a.WriteJSON(map[string]any{"type": "media", "camera": false, "mic": true}); err != nil {
		t.Fatal(err)
	}
	if p := readState(t, admin); len(p) != 1 || p[0].Camera || !p[0].Mic {
		t.Fatalf("media state: %v", p)
	}

	a.Close()
	if p := readState(t, admin); len(p) != 0 {
		t.Fatalf("a should leave: %v", p)
	}
}

func TestSocketRequiresUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/ws/m/:meetingId", NewPresenceController(services.NewPresenceService(core.NewPresenceHub(core.NewTrackRegistry()))).Socket)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/ws/m/m1", nil))
	if w.Code != 400 {
		t.Fatalf("code %d", w.Code)
	}
}

func TestSocketChatKickAndBlocked(t *testing.T) {
	gin.SetMode(gin.TestMode)
	hub := core.NewPresenceHub(core.NewTrackRegistry())
	r := gin.New()
	r.GET("/ws/m/:meetingId", NewPresenceController(services.NewPresenceService(hub)).Socket)
	srv := httptest.NewServer(r)
	defer srv.Close()

	admin := dial(t, srv, "userId=admin&observer=true")
	defer admin.Close()
	a := dial(t, srv, "userId=a")
	defer a.Close()

	_ = a.WriteJSON(map[string]any{"type": "chat", "text": "halo"})
	raw := readType(t, admin, "chat")
	if string(raw["from"]) != `"a"` || string(raw["text"]) != `"halo"` {
		t.Fatalf("chat: %v", raw)
	}

	_ = admin.WriteJSON(map[string]any{"type": "admin", "action": "kick", "target": "a"})
	readType(t, a, "command") // participant receives the kick command, then the socket closes

	// rejoin is refused with 403
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/m/m1?userId=a"
	if _, resp, err := websocket.DefaultDialer.Dial(url, nil); err == nil || resp == nil || resp.StatusCode != 403 {
		t.Fatalf("expected 403, got err=%v resp=%v", err, resp)
	}
}
