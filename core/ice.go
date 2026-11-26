package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/pion/webrtc/v4"

	"go-conference/config"
)

// ICEServer is the JSON shape browsers expect in RTCConfiguration.iceServers.
type ICEServer struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username,omitempty"`
	Credential string   `json:"credential,omitempty"`
}

// Pion converts to the pion type.
func (s ICEServer) Pion() webrtc.ICEServer {
	return webrtc.ICEServer{URLs: s.URLs, Username: s.Username, Credential: s.Credential}
}

const (
	cfBase     = "https://rtc.live.cloudflare.com"
	cfTTL      = 24 * time.Hour
	cfRefresh  = 30 * time.Minute // refresh when less than this remains
	fetchLimit = 5 * time.Second
)

// ICEProvider supplies STUN/TURN servers: static (coturn) or Cloudflare Realtime TURN
// with short-lived credentials that are cached and refreshed.
type ICEProvider struct {
	stun   string
	static []ICEServer

	cfKeyID, cfToken string
	cfBase           string
	client           *http.Client

	mu      sync.Mutex
	cached  []ICEServer
	expires time.Time
	retryAt time.Time // after a failed fetch, don't hammer Cloudflare (or stall requests) until then
}

func NewICEProvider(cfg config.Config) *ICEProvider {
	p := &ICEProvider{
		stun:    cfg.StunServer,
		cfKeyID: cfg.CFTurnKeyID,
		cfToken: cfg.CFTurnAPIToken,
		cfBase:  cfBase,
		client:  &http.Client{Timeout: fetchLimit},
	}
	if len(cfg.TurnURLs) > 0 {
		p.static = []ICEServer{{URLs: cfg.TurnURLs, Username: cfg.TurnUsername, Credential: cfg.TurnCredential}}
	}
	return p
}

// Enabled reports whether any TURN source is configured.
func (p *ICEProvider) Enabled() bool { return len(p.static) > 0 || p.cfKeyID != "" }

// ClientServers is what browsers get: STUN plus every TURN URL.
func (p *ICEProvider) ClientServers(ctx context.Context) []ICEServer {
	out := []ICEServer{{URLs: []string{p.stun}}}
	return append(out, p.turn(ctx)...)
}

// ServerTURN is what the SFU's own peer connections use: a single UDP TURN URL per server,
// to keep ICE gathering fast. Empty when TURN is not configured or unreachable.
func (p *ICEProvider) ServerTURN(ctx context.Context) []webrtc.ICEServer {
	var out []webrtc.ICEServer
	for _, s := range p.turn(ctx) {
		if u := pickUDPTurn(s.URLs); u != "" {
			out = append(out, ICEServer{URLs: []string{u}, Username: s.Username, Credential: s.Credential}.Pion())
		}
	}
	return out
}

func pickUDPTurn(urls []string) string {
	for _, u := range urls {
		l := strings.ToLower(u)
		if strings.HasPrefix(l, "turn:") && (strings.Contains(l, "transport=udp") || !strings.Contains(l, "transport=")) {
			return u
		}
	}
	return ""
}

func (p *ICEProvider) turn(ctx context.Context) []ICEServer {
	if p.cfKeyID == "" {
		return p.static
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cached != nil && time.Until(p.expires) > cfRefresh {
		return p.cached
	}
	fallback := func() []ICEServer { // stale creds may still work; else static/none
		return append(append([]ICEServer{}, p.cached...), p.static...)
	}
	if time.Now().Before(p.retryAt) {
		return fallback()
	}
	servers, err := p.fetchCloudflare(ctx)
	if err != nil {
		log.Printf("ice: cloudflare turn: %v", err)
		p.retryAt = time.Now().Add(time.Minute)
		return fallback()
	}
	p.cached, p.expires = servers, time.Now().Add(cfTTL)
	return servers
}

func (p *ICEProvider) fetchCloudflare(ctx context.Context) ([]ICEServer, error) {
	var errs []string
	for _, path := range []string{"generate-ice-servers", "generate"} {
		servers, err := p.cfRequest(ctx, path)
		if err == nil {
			return servers, nil
		}
		errs = append(errs, err.Error())
	}
	return nil, fmt.Errorf("%s (check CF_TURN_KEY_ID is the TURN key ID and CF_TURN_API_TOKEN its token)", strings.Join(errs, "; "))
}

func (p *ICEProvider) cfRequest(ctx context.Context, path string) ([]ICEServer, error) {
	url := fmt.Sprintf("%s/v1/turn/keys/%s/credentials/%s", p.cfBase, p.cfKeyID, path)
	body := fmt.Sprintf(`{"ttl":%d}`, int(cfTTL.Seconds()))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBufferString(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.cfToken)
	req.Header.Set("Content-Type", "application/json")
	res, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode/100 != 2 {
		return nil, fmt.Errorf("%s: status %d", path, res.StatusCode)
	}
	var raw struct {
		IceServers json.RawMessage `json:"iceServers"`
	}
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		return nil, err
	}
	var many []ICEServer
	if err := json.Unmarshal(raw.IceServers, &many); err == nil && len(many) > 0 {
		return dropPort53(many), nil
	}
	var one ICEServer // older shape: a single object
	if err := json.Unmarshal(raw.IceServers, &one); err != nil || len(one.URLs) == 0 {
		return nil, fmt.Errorf("%s: unexpected response", path)
	}
	return dropPort53([]ICEServer{one}), nil
}

// dropPort53 removes URLs on port 53: browsers time out on them, which would stall
// non-trickle ICE gathering.
func dropPort53(in []ICEServer) []ICEServer {
	out := make([]ICEServer, 0, len(in))
	for _, s := range in {
		var urls []string
		for _, u := range s.URLs {
			if !strings.Contains(u, ":53?") && !strings.HasSuffix(u, ":53") {
				urls = append(urls, u)
			}
		}
		if len(urls) > 0 {
			s.URLs = urls
			out = append(out, s)
		}
	}
	return out
}
