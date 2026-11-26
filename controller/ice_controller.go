package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"go-conference/services"
)

type IceController struct{ svc *services.IceService }

func NewIceController(svc *services.IceService) *IceController { return &IceController{svc: svc} }

// Handle returns {"iceServers":[...]} (STUN + TURN) for RTCPeerConnection.
//
//	@Summary		Daftar ICE server (STUN + TURN)
//	@Description	Dipakai klien untuk new RTCPeerConnection({iceServers}). Kredensial TURN dibuat via Cloudflare dan di-cache.
//	@Tags			WebRTC
//	@Produce		json
//	@Success		200	{object}	IceResponse
//	@Router			/webrtc/ice [get]
func (c *IceController) Handle(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, gin.H{"iceServers": c.svc.Servers(ctx.Request.Context())})
}
