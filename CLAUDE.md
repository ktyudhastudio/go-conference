# go-conference

Go (Gin + Pion WebRTC v4) SFU signaling server for video conference. No authentication (experimental). Works with the Angular UI in `../go-video-conference/ui` (proxy → `:8080`).

## Commands
- `make` — list commands · `make run` · `make build` · `make test` · `make dep`
- Config via `.env` (copy `.env.example`): `GOLANG_PORT` (8080), `APP_ENV` (`localhost` binds 0.0.0.0), `STUN_SERVER`

## Structure
```
main.go        bootstrap: providers → gin → routes
providers/     DI wiring (samber/do), names in pkg/constants
routes/        endpoint definitions only
controller/    HTTP layer: parse/validate request, map errors → status codes (no WebRTC logic)
services/      business logic (WebRTC negotiation, track forwarding); no gin imports
core/          infrastructure: pion API/config, TrackRegistry, SDP encode/decode
config/        env loading
middlewares/   gin middlewares (CORS)
pkg/constants/ DI keys and shared constants
```
Dependency direction: routes → controller → services → core. Never import upward.

## Conventions
- New dependency: register in `providers/core.go` with a constant name in `pkg/constants`, resolve with `do.MustInvokeNamed`.
- Return errors; never `log.Fatal`/`panic` in request paths.
- SDP over the wire is base64(JSON SessionDescription) in `{"Sdp": "..."}`.
- Endpoint: `POST /webrtc/sdp/m/:meetingId/c/:userID/p/:peerId/s/:isSender` (`isSender=true` publishes, else subscribes to `peerId`).
- `GET /webrtc/m/:meetingId/peers` → `{"peers":[userId,...]}` publisher aktif per meeting (registry per meeting; entry dihapus saat publisher putus).
- Codec: VP8 video only.
- Tests live next to code (`*_test.go`); `services/sdp_service_test.go` has an end-to-end publish/subscribe test using a pion client — no browser needed.

## Skills
- `/add-endpoint` — add a new endpoint following the layers
- `/run-tests` — run and interpret tests
