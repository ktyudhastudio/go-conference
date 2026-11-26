package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"go-conference/services"
)

type IceController struct{ svc *services.IceService }

func NewIceController(svc *services.IceService) *IceController { return &IceController{svc: svc} }

// Handle returns {"iceServers":[...]} (STUN + TURN) for RTCPeerConnection.
func (c *IceController) Handle(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, gin.H{"iceServers": c.svc.Servers(ctx.Request.Context())})
}
