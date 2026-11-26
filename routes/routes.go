package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/samber/do"

	"go-conference/controller"
	"go-conference/pkg/constants"
)

// Register wires all HTTP routes, resolving controllers from the injector.
func Register(r *gin.Engine, injector *do.Injector) {
	sdp := do.MustInvokeNamed[*controller.SdpController](injector, constants.SdpController)

	peer := do.MustInvokeNamed[*controller.PeerController](injector, constants.PeerController)

	presence := do.MustInvokeNamed[*controller.PresenceController](injector, constants.PresenceController)

	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })

	webrtc := r.Group("/webrtc")
	webrtc.GET("/m/:meetingId/peers", peer.List)
	webrtc.GET("/meetings", presence.Meetings)
	webrtc.GET("/m/:meetingId/participants", presence.List)
	webrtc.POST("/sdp/m/:meetingId/c/:userID/p/:peerId/s/:isSender", sdp.Handle)

	r.GET("/ws/m/:meetingId", presence.Socket)
}
