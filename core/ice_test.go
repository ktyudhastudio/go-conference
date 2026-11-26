package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"go-conference/config"
)

func cfServer(t *testing.T, body string, hits *int32) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(hits, 1)
		if r.Header.Get("Authorization") != "Bearer tok" {
			w.WriteHeader(401)
			return
		}
		w.Write([]byte(body))
	}))
}

func newCF(url string) *ICEProvider {
	p := NewICEProvider(config.Config{StunServer: "stun:s:1", CFTurnKeyID: "k", CFTurnAPIToken: "tok"})
	p.cfBase = url
	return p
}

func TestICEStaticTurn(t *testing.T) {
	p := NewICEProvider(config.Config{StunServer: "stun:s:1", TurnURLs: []string{"turn:t:3478"}, TurnUsername: "u", TurnCredential: "c"})
	got := p.ClientServers(context.Background())
	if len(got) != 2 || got[1].Username != "u" || got[1].Credential != "c" {
		t.Fatalf("client servers: %+v", got)
	}
	if srv := p.ServerTURN(context.Background()); len(srv) != 1 || srv[0].URLs[0] != "turn:t:3478" {
		t.Fatalf("server turn: %+v", srv)
	}
}

func TestICENoTurnIsStunOnly(t *testing.T) {
	p := NewICEProvider(config.Config{StunServer: "stun:s:1"})
	if p.Enabled() || len(p.ClientServers(context.Background())) != 1 || len(p.ServerTURN(context.Background())) != 0 {
		t.Fatal("expected stun only")
	}
}

func TestICECloudflareArrayShapeCachedAndFiltered(t *testing.T) {
	var hits int32
	srv := cfServer(t, `{"iceServers":[{"urls":["stun:x:3478"]},{"urls":["turns:x:443?transport=tcp","turn:x:3478?transport=tcp","turn:x:3478?transport=udp"],"username":"u","credential":"c"}]}`, &hits)
	defer srv.Close()
	p := newCF(srv.URL)
	ctx := context.Background()
	if got := p.ClientServers(ctx); len(got) != 3 {
		t.Fatalf("client servers: %+v", got)
	}
	srv2 := p.ServerTURN(ctx)
	if len(srv2) != 1 || srv2[0].URLs[0] != "turn:x:3478?transport=udp" {
		t.Fatalf("server turn: %+v", srv2)
	}
	if hits != 1 {
		t.Fatalf("credentials should be cached, hits=%d", hits)
	}
}

func TestICECloudflareObjectShape(t *testing.T) {
	var hits int32
	srv := cfServer(t, `{"iceServers":{"urls":["turn:x:3478?transport=udp"],"username":"u","credential":"c"}}`, &hits)
	defer srv.Close()
	if got := newCF(srv.URL).ServerTURN(context.Background()); len(got) != 1 {
		t.Fatalf("got %+v", got)
	}
}

func TestICECloudflareFailureDegradesToStun(t *testing.T) {
	var hits int32
	srv := cfServer(t, `{}`, &hits)
	defer srv.Close()
	p := newCF(srv.URL)
	p.cfToken = "wrong"
	if got := p.ClientServers(context.Background()); len(got) != 1 {
		t.Fatalf("want stun only, got %+v", got)
	}
}
