package core

import (
	"net"

	"github.com/pion/webrtc/v4"

	"go-conference/config"
)

// NewWebRTCAPI builds a pion API that supports VP8 video and Opus audio.
func NewWebRTCAPI(opts ...func(*webrtc.SettingEngine)) (*webrtc.API, error) {
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
	if err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:    webrtc.MimeTypeOpus,
			ClockRate:   48000,
			Channels:    2,
			SDPFmtpLine: "minptime=10;useinbandfec=1",
		},
		PayloadType: 111,
	}, webrtc.RTPCodecTypeAudio); err != nil {
		return nil, err
	}
	se := webrtc.SettingEngine{}
	for _, o := range opts {
		o(&se)
	}
	return webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithSettingEngine(se)), nil
}

// NetworkOptions pins the UDP port range and advertises the public IP so media
// works behind Docker/NAT (open the same range in the firewall).
func NetworkOptions(cfg config.Config) func(*webrtc.SettingEngine) {
	return func(se *webrtc.SettingEngine) {
		if cfg.UDPPortMin > 0 && cfg.UDPPortMax >= cfg.UDPPortMin {
			_ = se.SetEphemeralUDPPortRange(cfg.UDPPortMin, cfg.UDPPortMax)
		}
		if ip := resolveHost(cfg.Host); ip != "" {
			se.SetNAT1To1IPs([]string{ip}, webrtc.ICECandidateTypeHost)
		}
	}
}

// PeerConfig returns the ICE configuration.
func PeerConfig(cfg config.Config) webrtc.Configuration {
	return webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{{URLs: []string{cfg.StunServer}}},
	}
}

// resolveHost accepts an IP or a hostname (resolved to its first IPv4).
func resolveHost(h string) string {
	if h == "" {
		return ""
	}
	if ip := net.ParseIP(h); ip != nil {
		return ip.String()
	}
	ips, err := net.LookupIP(h)
	if err != nil {
		return ""
	}
	for _, ip := range ips {
		if v4 := ip.To4(); v4 != nil {
			return v4.String()
		}
	}
	return ""
}
