package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"go-conference/services"
)

type PeerController struct {
	svc *services.PeerService
}

func NewPeerController(svc *services.PeerService) *PeerController {
	return &PeerController{svc: svc}
}

// List: GET /webrtc/m/:meetingId/peers
func (h *PeerController) List(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"peers": h.svc.ListPublishers(c.Param("meetingId"))})
}
