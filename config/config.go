package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds application settings.
type Config struct {
	AppEnv     string
	Port       string
	StunServer string
	Host       string // APP_HOST: IP/hostname yang diiklankan ke browser untuk media
	UDPPortMin uint16
	UDPPortMax uint16

	// TURN relay so peers outside the LAN can connect without open ports.
	// Option 1 (Cloudflare Realtime TURN, short-lived credentials generated via API):
	CFTurnKeyID    string // CF_TURN_KEY_ID
	CFTurnAPIToken string // CF_TURN_API_TOKEN
	// Option 2 (static, e.g. coturn): TURN_URLS comma-separated, TURN_USERNAME, TURN_CREDENTIAL
	TurnURLs       []string
	TurnUsername   string
	TurnCredential string
}

// Load reads .env (if present) then environment variables, with defaults.
func Load() Config {
	_ = godotenv.Load(".env")
	return Config{
		AppEnv:     getenv("APP_ENV", "production"),
		Port:       getenv("APP_PORT", getenv("GOLANG_PORT", "8080")),
		StunServer: getenv("STUN_SERVER", "stun:stun.l.google.com:19302"),
		Host:       getenv("APP_HOST", ""),
		UDPPortMin: port("APP_UDP_PORT_MIN"),
		UDPPortMax: port("APP_UDP_PORT_MAX"),

		CFTurnKeyID:    getenv("CF_TURN_KEY_ID", ""),
		CFTurnAPIToken: getenv("CF_TURN_API_TOKEN", ""),
		TurnURLs:       split(getenv("TURN_URLS", "")),
		TurnUsername:   getenv("TURN_USERNAME", ""),
		TurnCredential: getenv("TURN_CREDENTIAL", ""),
	}
}

// ServeAddr binds to all interfaces on localhost env, otherwise just the port.
func (c Config) ServeAddr() string {
	if c.AppEnv == "localhost" {
		return "0.0.0.0:" + c.Port
	}
	return ":" + c.Port
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func port(k string) uint16 {
	n, err := strconv.ParseUint(os.Getenv(k), 10, 16)
	if err != nil {
		return 0
	}
	return uint16(n)
}

func split(v string) []string {
	var out []string
	for _, p := range strings.Split(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
