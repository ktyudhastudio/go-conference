package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"go-conference/config"
	"go-conference/core"
	"go-conference/services"
)

func TestIceEndpointReturnsStunAndTurn(t *testing.T) {
	gin.SetMode(gin.TestMode)
	p := core.NewICEProvider(config.Config{StunServer: "stun:s:1", TurnURLs: []string{"turn:t:3478"}, TurnUsername: "u", TurnCredential: "c"})
	r := gin.New()
	r.GET("/webrtc/ice", NewIceController(services.NewIceService(p)).Handle)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/webrtc/ice", nil))
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("code=%d headers=%v", w.Code, w.Header())
	}
	var body struct {
		IceServers []core.ICEServer `json:"iceServers"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || len(body.IceServers) != 2 || body.IceServers[1].Username != "u" {
		t.Fatalf("body=%s err=%v", w.Body.String(), err)
	}
}
