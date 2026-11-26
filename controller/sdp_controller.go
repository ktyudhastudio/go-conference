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
//
//	@Summary		Negosiasi SDP (publish / subscribe)
//	@Description	isSender=true: publish video+audio. isSender=false: subscribe ke peerId. ICE non-trickle: server menunggu gathering selesai sebelum membalas. Sdp = base64(JSON SessionDescription).
//	@Tags			WebRTC
//	@Accept			json
//	@Produce		json
//	@Param			meetingId	path		string	true	"ID rapat"
//	@Param			userID		path		string	true	"ID pemanggil"
//	@Param			peerId		path		string	true	"Subscribe: userId publisher yang dituju. Publish: isi '-'"
//	@Param			isSender	path		bool	true	"true = publish, false = subscribe"
//	@Param			body		body		Sdp		true	"SDP offer"
//	@Success		200			{object}	Sdp		"SDP answer"
//	@Failure		400			{object}	ErrorResponse
//	@Failure		500			{object}	ErrorResponse
//	@Router			/webrtc/sdp/m/{meetingId}/c/{userID}/p/{peerId}/s/{isSender} [post]
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
