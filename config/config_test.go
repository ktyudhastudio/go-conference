package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("APP_PORT", "")
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

func TestLoadTurn(t *testing.T) {
	t.Setenv("TURN_URLS", " turn:a:3478?transport=udp , ,turns:a:443?transport=tcp")
	t.Setenv("CF_TURN_KEY_ID", "k")
	c := Load()
	if len(c.TurnURLs) != 2 || c.TurnURLs[0] != "turn:a:3478?transport=udp" || c.CFTurnKeyID != "k" {
		t.Fatalf("%+v", c)
	}
}

func TestLoadTrimsCloudflareCredentials(t *testing.T) {
	t.Setenv("CF_TURN_KEY_ID", "  abc123 \n")
	t.Setenv("CF_TURN_API_TOKEN", "\ttok ")
	c := Load()
	if c.CFTurnKeyID != "abc123" || c.CFTurnAPIToken != "tok" {
		t.Fatalf("%q %q", c.CFTurnKeyID, c.CFTurnAPIToken)
	}
}
