package providers

import (
	"github.com/pion/webrtc/v4"
	"github.com/samber/do"

	"go-conference/config"
	"go-conference/controller"
	"go-conference/core"
	"go-conference/pkg/constants"
	"go-conference/services"
)

func RegisterDependencies(injector *do.Injector) {
	do.ProvideNamedValue(injector, constants.Config, config.Load())

	do.ProvideNamed(injector, constants.WebRTCAPI, func(i *do.Injector) (*webrtc.API, error) {
		return core.NewWebRTCAPI()
	})

	do.ProvideNamed(injector, constants.TrackRegistry, func(i *do.Injector) (*core.TrackRegistry, error) {
		return core.NewTrackRegistry(), nil
	})

	do.ProvideNamed(injector, constants.SdpService, func(i *do.Injector) (*services.SdpService, error) {
		cfg := do.MustInvokeNamed[config.Config](i, constants.Config)
		api := do.MustInvokeNamed[*webrtc.API](i, constants.WebRTCAPI)
		reg := do.MustInvokeNamed[*core.TrackRegistry](i, constants.TrackRegistry)
		return services.NewSdpService(api, core.PeerConfig(cfg), reg), nil
	})

	do.ProvideNamed(injector, constants.SdpController, func(i *do.Injector) (*controller.SdpController, error) {
		svc := do.MustInvokeNamed[*services.SdpService](i, constants.SdpService)
		return controller.NewSdpController(svc), nil
	})

	do.ProvideNamed(injector, constants.PeerService, func(i *do.Injector) (*services.PeerService, error) {
		reg := do.MustInvokeNamed[*core.TrackRegistry](i, constants.TrackRegistry)
		return services.NewPeerService(reg), nil
	})

	do.ProvideNamed(injector, constants.PeerController, func(i *do.Injector) (*controller.PeerController, error) {
		svc := do.MustInvokeNamed[*services.PeerService](i, constants.PeerService)
		return controller.NewPeerController(svc), nil
	})
}
