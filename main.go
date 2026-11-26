package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/samber/do"

	"go-conference/config"
	_ "go-conference/docs"
	"go-conference/middlewares"
	"go-conference/pkg/constants"
	"go-conference/providers"
	"go-conference/routes"
)

// @title			go-conference API
// @version		1.0
// @description	SFU signaling server (Gin + Pion WebRTC). Tanpa autentikasi. SDP dikirim sebagai base64(JSON SessionDescription) di {"Sdp":"..."}.
// @BasePath		/
func main() {
	injector := do.New()
	providers.RegisterDependencies(injector)

	cfg := do.MustInvokeNamed[config.Config](injector, constants.Config)

	server := gin.Default()
	server.Use(middlewares.CORSMiddleware())
	routes.Register(server, injector)

	if err := server.Run(cfg.ServeAddr()); err != nil {
		log.Fatalf("error running server: %v", err)
	}
}
