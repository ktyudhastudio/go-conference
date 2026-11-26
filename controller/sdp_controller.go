package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"go-conference/services"
)

// Sdp is the request/response body carrying a base64 session description.
type Sdp struct {
	Sdp string `json:"Sdp"`
}

type SdpController struct {
	svc *services.SdpService
}

func NewSdpController(svc *services.SdpService) *SdpController {
	return &SdpController{svc: svc}
}

// Handle: POST /webrtc/sdp/m/:meetingId/c/:userID/p/:peerId/s/:isSender
func (h *SdpController) Handle(c *gin.Context) {
	isSender, err := strconv.ParseBool(c.Param("isSender"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "isSender must be a boolean"})
		return
	}

	var body Sdp
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	answer, err := h.svc.Negotiate(c.Request.Context(), isSender, c.Param("meetingId"), c.Param("userID"), c.Param("peerId"), body.Sdp)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, Sdp{Sdp: answer})
}
