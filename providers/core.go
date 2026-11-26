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
		cfg := do.MustInvokeNamed[config.Config](i, constants.Config)
		return core.NewWebRTCAPI(core.NetworkOptions(cfg))
	})

	do.ProvideNamed(injector, constants.TrackRegistry, func(i *do.Injector) (*core.TrackRegistry, error) {
		return core.NewTrackRegistry(), nil
	})

	do.ProvideNamed(injector, constants.ICEProvider, func(i *do.Injector) (*core.ICEProvider, error) {
		cfg := do.MustInvokeNamed[config.Config](i, constants.Config)
		return core.NewICEProvider(cfg), nil
	})

	do.ProvideNamed(injector, constants.IceService, func(i *do.Injector) (*services.IceService, error) {
		return services.NewIceService(do.MustInvokeNamed[*core.ICEProvider](i, constants.ICEProvider)), nil
	})

	do.ProvideNamed(injector, constants.IceController, func(i *do.Injector) (*controller.IceController, error) {
		return controller.NewIceController(do.MustInvokeNamed[*services.IceService](i, constants.IceService)), nil
	})

	do.ProvideNamed(injector, constants.SdpService, func(i *do.Injector) (*services.SdpService, error) {
		cfg := do.MustInvokeNamed[config.Config](i, constants.Config)
		api := do.MustInvokeNamed[*webrtc.API](i, constants.WebRTCAPI)
		reg := do.MustInvokeNamed[*core.TrackRegistry](i, constants.TrackRegistry)
		svc := services.NewSdpService(api, core.PeerConfig(cfg), reg)
		hub := do.MustInvokeNamed[*core.PresenceHub](i, constants.PresenceHub)
		svc.SetQualityReporter(hub.SetQuality)
		svc.SetICEProvider(do.MustInvokeNamed[*core.ICEProvider](i, constants.ICEProvider))
		return svc, nil
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

	do.ProvideNamed(injector, constants.PresenceHub, func(i *do.Injector) (*core.PresenceHub, error) {
		reg := do.MustInvokeNamed[*core.TrackRegistry](i, constants.TrackRegistry)
		return core.NewPresenceHub(reg), nil
	})

	do.ProvideNamed(injector, constants.PresenceService, func(i *do.Injector) (*services.PresenceService, error) {
		hub := do.MustInvokeNamed[*core.PresenceHub](i, constants.PresenceHub)
		return services.NewPresenceService(hub), nil
	})

	do.ProvideNamed(injector, constants.PresenceController, func(i *do.Injector) (*controller.PresenceController, error) {
		svc := do.MustInvokeNamed[*services.PresenceService](i, constants.PresenceService)
		return controller.NewPresenceController(svc), nil
	})
}
