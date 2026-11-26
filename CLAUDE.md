# go-conference

Go (Gin + Pion WebRTC v4) SFU signaling server for video conference. No authentication (experimental). Works with the Angular UI in `../go-video-conference/ui` (proxy → `:8080`).

## Commands
- `make` — list commands · `make run` · `make build` · `make test` · `make dep`
- Config via `.env` (copy `.env.example`): `APP_PORT` (8080), `APP_ENV` (`localhost` binds 0.0.0.0), `APP_HOST` (IP/hostname server untuk media; wajib di Docker), `APP_UDP_PORT_MIN/MAX`, `STUN_SERVER`

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
- `GET /webrtc/meetings` → `{"meetings":[{id,participants,startedAt}]}` rapat yang sedang aktif (ada peserta; in-memory).
- `GET /ws/m/:meetingId?userId=..&observer=true|false` — WebSocket presence (join = buka socket). Server push `{"type":"state","participants":[{userId,camera,mic,publishing}]}`; klien kirim `{"type":"media","camera":bool,"mic":bool}` atau `{"type":"chat","text":".."}`; server juga mengirim `chat`, `history` (riwayat saat join) dan `command` (`mute_mic|camera_off|kick`). Observer (admin) boleh kirim `{"type":"admin","action":"mute_mic|camera_off|kick|allow","target":"userId"}`; `kick` memblokir userId (403 saat join) sampai `allow`. Chat/riwayat/blocklist in-memory, hilang saat rapat kosong (tanpa auth; persistensi → database nanti). `observer=true` (admin) tidak terdaftar sebagai peserta. `GET /webrtc/m/:meetingId/participants` = snapshot.
- Publisher mengirim video+audio dalam satu PeerConnection; track terdaftar di registry saat koneksi `connected` (bukan saat paket pertama), sehingga peserta dengan kamera mati tetap bisa di-subscribe. PC yang mati ditutup `watchConnection`.
- `GET /webrtc/ice` → `{"iceServers":[...]}` (STUN + TURN) untuk `RTCPeerConnection` klien. TURN agar peserta di luar LAN bisa konek tanpa membuka port: `CF_TURN_KEY_ID` + `CF_TURN_API_TOKEN` (Cloudflare Realtime TURN, kredensial dibuat via API & di-cache) atau statis `TURN_URLS` (koma) + `TURN_USERNAME` + `TURN_CREDENTIAL`. PC milik server juga memakai TURN (satu URL UDP) → relay↔relay. Tanpa konfigurasi: STUN saja.
- Codec: VP8 video + Opus audio.
- Tests live next to code (`*_test.go`); `services/sdp_service_test.go` has an end-to-end publish/subscribe test using a pion client — no browser needed.

## Skills
- `/add-endpoint` — add a new endpoint following the layers
- `/run-tests` — run and interpret tests
