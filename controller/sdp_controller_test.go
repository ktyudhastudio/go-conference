package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"go-conference/config"
	"go-conference/core"
	"go-conference/services"
)

func newRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	api, err := core.NewWebRTCAPI()
	if err != nil {
		t.Fatal(err)
	}
	svc := services.NewSdpService(api, core.PeerConfig(config.Load()), core.NewTrackRegistry())
	r := gin.New()
	r.POST("/webrtc/sdp/m/:meetingId/c/:userID/p/:peerId/s/:isSender", NewSdpController(svc).Handle)
	return r
}

func post(r *gin.Engine, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestHandleInvalidIsSender(t *testing.T) {
	w := post(newRouter(t), "/webrtc/sdp/m/1/c/a/p/b/s/maybe", `{"Sdp":"x"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("code %d", w.Code)
	}
}

func TestHandleInvalidJSON(t *testing.T) {
	w := post(newRouter(t), "/webrtc/sdp/m/1/c/a/p/b/s/true", `not json`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("code %d", w.Code)
	}
}

func TestHandleInvalidSdp(t *testing.T) {
	w := post(newRouter(t), "/webrtc/sdp/m/1/c/a/p/b/s/true", `{"Sdp":"!!!"}`)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("code %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "invalid sdp") {
		t.Fatalf("body %s", w.Body.String())
	}
}
