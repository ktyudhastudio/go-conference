package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"go-conference/core"
	"go-conference/services"
)

func TestListPeersEmpty(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/webrtc/m/:meetingId/peers", NewPeerController(services.NewPeerService(core.NewTrackRegistry())).List)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/webrtc/m/m1/peers", nil))
	if w.Code != 200 || w.Body.String() != `{"peers":[]}` {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}
