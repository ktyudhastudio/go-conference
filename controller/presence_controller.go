package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"go-conference/core"
	"go-conference/services"
)

const (
	wsPingInterval = 15 * time.Second
	wsReadTimeout  = 40 * time.Second // putus jika tak ada pong/pesan selama ini
	wsWriteTimeout = 10 * time.Second
)

// Experimental server without auth: any origin may connect (same as CORS "*").
var upgrader = websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}

type PresenceController struct {
	svc *services.PresenceService
}

func NewPresenceController(svc *services.PresenceService) *PresenceController {
	return &PresenceController{svc: svc}
}

type stateMessage struct {
	Type string `json:"type"` // "state"
	core.State
}

type chatMessage struct {
	Type string `json:"type"` // "chat"
	core.ChatMessage
}

type historyMessage struct {
	Type     string             `json:"type"` // "history"
	Messages []core.ChatMessage `json:"messages"`
}

type commandMessage struct {
	Type string `json:"type"` // "command"
	core.Command
}

type clientMessage struct {
	Type   string `json:"type"` // media | chat | admin
	Camera bool   `json:"camera"`
	Mic    bool   `json:"mic"`
	Text   string `json:"text"`
	Action string `json:"action"`
	Target string `json:"target"`
}

// List: GET /webrtc/m/:meetingId/participants (snapshot, tanpa websocket)
//
//	@Summary	Snapshot peserta rapat
//	@Tags		Presence
//	@Produce	json
//	@Param		meetingId	path		string	true	"ID rapat"
//	@Success	200			{object}	core.State
//	@Router		/webrtc/m/{meetingId}/participants [get]
func (h *PresenceController) List(c *gin.Context) {
	c.JSON(http.StatusOK, h.svc.State(c.Param("meetingId")))
}

// Meetings: GET /webrtc/meetings (rapat yang sedang aktif, untuk admin)
//
//	@Summary		Rapat yang sedang aktif
//	@Description	In-memory; rapat tanpa peserta tidak tampil. startedAt = unix ms.
//	@Tags			Presence
//	@Produce		json
//	@Success		200	{object}	MeetingsResponse
//	@Router			/webrtc/meetings [get]
func (h *PresenceController) Meetings(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"meetings": h.svc.ActiveMeetings()})
}

// Socket: GET /ws/m/:meetingId?userId=..&observer=true
// Join = membuka socket. Server mendorong {"type":"state","participants":[...]} setiap ada
// perubahan, plus {"type":"chat"|"history"|"command"}. Klien mengirim
// {"type":"media","camera":bool,"mic":bool}, {"type":"chat","text":".."} dan (observer saja)
// {"type":"admin","action":"mute_mic|camera_off|kick|allow","target":"userId"}.
//
//	@Summary		WebSocket presence, chat, perintah admin
//	@Description	Upgrade ke WebSocket (Swagger UI tidak bisa menjalankannya; uji dengan wscat). Join = membuka socket.
//	@Description	Server -> klien: {"type":"state","participants":[...],"blocked":[...]}, {"type":"history","messages":[{id,from,text,at}]}, {"type":"chat","id":1,"from":"u","text":"..","at":1764200000000}, {"type":"command","action":"mute_mic|camera_off|kick"}.
//	@Description	Klien -> server: {"type":"media","camera":true,"mic":false}, {"type":"chat","text":".."} (maks 1000 karakter), {"type":"admin","action":"mute_mic|camera_off|kick|allow","target":"userId"} (hanya observer=true).
//	@Description	Ping server tiap 15 dtk; putus bila tak ada pong/pesan 40 dtk.
//	@Tags			Presence
//	@Param			meetingId	path		string	true	"ID rapat"
//	@Param			userId		query		string	true	"ID peserta"
//	@Param			observer	query		bool	false	"true = admin, tidak terdaftar sebagai peserta"
//	@Success		101			{string}	string	"Switching Protocols"
//	@Failure		400			{object}	ErrorResponse	"userId kosong"
//	@Failure		403			{object}	ErrorResponse	"userId diblokir admin (kick) sampai di-allow"
//	@Router			/ws/m/{meetingId} [get]
func (h *PresenceController) Socket(c *gin.Context) {
	userID := c.Query("userId")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "userId is required"})
		return
	}
	observer, _ := strconv.ParseBool(c.Query("observer"))

	sess, err := h.svc.Join(c.Param("meetingId"), userID, observer)
	if errors.Is(err, core.ErrBlocked) {
		c.JSON(http.StatusForbidden, gin.H{"error": "You were removed from this meeting by an admin"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer sess.Close()

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return // Upgrade already replied
	}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(wsReadTimeout))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(wsReadTimeout))
	})

	done := make(chan struct{})
	go func() { // reader: media updates + liveness
		defer close(done)
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			_ = conn.SetReadDeadline(time.Now().Add(wsReadTimeout))
			var msg clientMessage
			if json.Unmarshal(data, &msg) != nil {
				continue
			}
			switch msg.Type {
			case "media":
				sess.SetMedia(msg.Camera, msg.Mic)
			case "chat":
				sess.Chat(msg.Text)
			case "admin":
				_ = sess.AdminAction(msg.Action, msg.Target)
			}
		}
	}()

	ping := time.NewTicker(wsPingInterval)
	defer ping.Stop()
	for {
		select {
		case <-done:
			return
		case st := <-sess.Updates():
			if write(conn, stateMessage{Type: "state", State: st}) != nil {
				return
			}
		case m := <-sess.Messages():
			if write(conn, wrap(m)) != nil {
				return
			}
		case <-sess.Kicked():
			// kirim sisa pesan (termasuk perintah kick) sebelum menutup
			for {
				select {
				case m := <-sess.Messages():
					_ = write(conn, wrap(m))
				default:
					return
				}
			}
		case <-ping.C:
			_ = conn.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func write(conn *websocket.Conn, v any) error {
	_ = conn.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
	return conn.WriteJSON(v)
}

func wrap(m any) any {
	switch v := m.(type) {
	case core.ChatMessage:
		return chatMessage{Type: "chat", ChatMessage: v}
	case core.Command:
		return commandMessage{Type: "command", Command: v}
	case []core.ChatMessage:
		return historyMessage{Type: "history", Messages: v}
	}
	return m
}
