package config

import (
	"os"

	"github.com/joho/godotenv"
)

// Config holds application settings.
type Config struct {
	AppEnv     string
	Port       string
	StunServer string
}

// Load reads .env (if present) then environment variables, with defaults.
func Load() Config {
	_ = godotenv.Load(".env")
	return Config{
		AppEnv:     getenv("APP_ENV", "production"),
		Port:       getenv("GOLANG_PORT", "8080"),
		StunServer: getenv("STUN_SERVER", "stun:stun.l.google.com:19302"),
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
