package core

import (
	"github.com/pion/webrtc/v4"

	"go-conference/config"
)

// NewWebRTCAPI builds a pion API that supports VP8 video only (simplifies proxying).
func NewWebRTCAPI() (*webrtc.API, error) {
	m := &webrtc.MediaEngine{}
	if err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:  webrtc.MimeTypeVP8,
			ClockRate: 90000,
		},
		PayloadType: 96,
	}, webrtc.RTPCodecTypeVideo); err != nil {
		return nil, err
	}
	return webrtc.NewAPI(webrtc.WithMediaEngine(m)), nil
}

// PeerConfig returns the ICE configuration.
func PeerConfig(cfg config.Config) webrtc.Configuration {
	return webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{{URLs: []string{cfg.StunServer}}},
	}
}
