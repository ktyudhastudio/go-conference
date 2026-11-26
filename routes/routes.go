package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/samber/do"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"go-conference/config"
	"go-conference/controller"
	"go-conference/pkg/constants"
)

// Register wires all HTTP routes, resolving controllers from the injector.
func Register(r *gin.Engine, injector *do.Injector) {
	sdp := do.MustInvokeNamed[*controller.SdpController](injector, constants.SdpController)

	peer := do.MustInvokeNamed[*controller.PeerController](injector, constants.PeerController)

	ice := do.MustInvokeNamed[*controller.IceController](injector, constants.IceController)

	presence := do.MustInvokeNamed[*controller.PresenceController](injector, constants.PresenceController)

	r.GET("/health", health)

	if do.MustInvokeNamed[config.Config](injector, constants.Config).SwaggerEnabled {
		r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	webrtc := r.Group("/webrtc")
	webrtc.GET("/m/:meetingId/peers", peer.List)
	webrtc.GET("/ice", ice.Handle)
	webrtc.GET("/meetings", presence.Meetings)
	webrtc.GET("/m/:meetingId/participants", presence.List)
	webrtc.POST("/sdp/m/:meetingId/c/:userID/p/:peerId/s/:isSender", sdp.Handle)

	r.GET("/ws/m/:meetingId", presence.Socket)
}

// health reports liveness.
//
//	@Summary	Cek server hidup
//	@Tags		System
//	@Produce	json
//	@Success	200	{object}	controller.HealthResponse
//	@Router		/health [get]
func health(c *gin.Context) { c.JSON(200, controller.HealthResponse{Status: "ok"}) }
