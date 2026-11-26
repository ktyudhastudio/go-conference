package config

import (
	"os"
	"strconv"

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
