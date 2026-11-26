package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("GOLANG_PORT", "")
	t.Setenv("APP_ENV", "")
	t.Setenv("STUN_SERVER", "")
	c := Load()
	if c.Port == "" || c.StunServer == "" || c.AppEnv == "" {
		t.Fatalf("defaults missing: %+v", c)
	}
}

func TestServeAddr(t *testing.T) {
	if got := (Config{AppEnv: "localhost", Port: "9000"}).ServeAddr(); got != "0.0.0.0:9000" {
		t.Fatal(got)
	}
	if got := (Config{AppEnv: "production", Port: "9000"}).ServeAddr(); got != ":9000" {
		t.Fatal(got)
	}
}
