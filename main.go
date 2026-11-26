package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/samber/do"

	"go-conference/config"
	"go-conference/middlewares"
	"go-conference/pkg/constants"
	"go-conference/providers"
	"go-conference/routes"
)

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
