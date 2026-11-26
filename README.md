# go-conference

Server signaling **SFU** (Selective Forwarding Unit) untuk video conference, ditulis dengan Go. Peserta mengirim video+audio satu kali ke server, lalu server meneruskannya ke peserta lain. Tanpa autentikasi (eksperimental).

UI pasangannya: [react-conference](../react-conference) (proxy → `:8080`).

## Daftar isi
- [Tools & library](#tools--library)
- [Cara kerja](#cara-kerja)
- [Menjalankan](#menjalankan)
- [Environment](#environment)
- [API](#api)
- [WebSocket (presence & chat)](#websocket-presence--chat)
- [Koneksi di luar LAN (TURN)](#koneksi-di-luar-lan-turn)
- [Deploy](#deploy)
- [Struktur kode](#struktur-kode)
- [Test](#test)
- [Troubleshooting](#troubleshooting)

Dokumentasi API (Swagger/OpenAPI 2.0) dibuat otomatis dari anotasi di controller: [docs/swagger.yaml](docs/swagger.yaml). Swagger UI ada di `http://localhost:8080/swagger/index.html` (aktif saat `APP_ENV=localhost` atau `SWAGGER_ENABLED=true`; mati default di produksi).

## Tools & library

| Kegunaan | Pilihan |
|---|---|
| Bahasa | Go 1.26 |
| HTTP framework | [Gin](https://github.com/gin-gonic/gin) |
| WebRTC | [Pion WebRTC v4](https://github.com/pion/webrtc) (+ `pion/rtcp`) |
| WebSocket | [gorilla/websocket](https://github.com/gorilla/websocket) |
| Dependency injection | [samber/do](https://github.com/samber/do) |
| Konfigurasi `.env` | [godotenv](https://github.com/joho/godotenv) |
| TURN/STUN | Cloudflare Realtime TURN (atau coturn sendiri) |
| Container | Docker (image distroless), Docker Compose |
| CI/CD | GitHub Actions → GHCR → deploy via SSH |

Codec: **VP8** (video) + **Opus** (audio).

## Cara kerja

```
 Peserta A ──video/audio──▶ ┌──────────────┐ ──▶ Peserta B
 (publisher)                │  SFU (Pion)  │ ──▶ Peserta C
 Peserta B ──video/audio──▶ │  registry    │ ──▶ Peserta A
                            └──────────────┘
        ▲  signaling: HTTP (SDP)    ▲  presence & chat: WebSocket
```

1. **Join.** Peserta membuka WebSocket `/ws/m/:meetingId`. Membuka socket = masuk rapat. Server mendorong daftar peserta (`state`) setiap ada perubahan.
2. **Publish.** Browser membuat `RTCPeerConnection`, mengirim offer SDP ke `POST /webrtc/sdp/.../s/true`, dan menerima answer. Satu koneksi membawa video+audio. Track didaftarkan ke *registry* saat koneksi `connected`, jadi peserta dengan kamera mati tetap bisa di-subscribe.
3. **Subscribe.** Untuk setiap peserta lain, browser membuka koneksi `recvonly` lewat `POST /webrtc/sdp/.../s/false` dengan `peerId` = pemilik video. Daftar publisher didapat dari `GET /webrtc/m/:meetingId/peers`.
4. **ICE gathering non-trickle.** Server menunggu pengumpulan kandidat ICE selesai sebelum membalas, jadi satu request/response sudah cukup (tanpa pertukaran kandidat susulan).
5. **Pembersihan.** Koneksi yang gagal, putus lebih dari 5 detik, atau tidak pernah terhubung dalam 20 detik ditutup otomatis, dan publisher dihapus dari registry.
6. **Admin.** Pengguna dengan `observer=true` (tidak dihitung sebagai peserta) bisa membisukan mic, mematikan kamera, atau mengeluarkan peserta (`kick` memblokir `userId` sampai di-`allow`).

Format SDP di jaringan: `base64(JSON SessionDescription)` di dalam `{"Sdp": "..."}`.

Semua state (peserta, chat, blocklist) **in-memory** dan hilang saat rapat kosong atau server restart.

## Menjalankan

```bash
cp .env.example .env     # sesuaikan
make run                 # go run main.go
```

| Perintah | Fungsi |
|---|---|
| `make` | Daftar perintah |
| `make run` | Jalankan server |
| `make build` | Build binary `./main` |
| `make test` | Jalankan semua test |
| `make dep` | `go mod tidy` |
| `make swag` | Generate ulang `docs/` dari anotasi handler (jalankan setelah mengubah endpoint) |
| `make docker-build` / `docker-run` / `docker-push` | Image Docker |

Cek server: `curl localhost:8080/health` → `{"status":"ok"}`.

### Docker

```bash
docker compose up -d     # memakai .env
```

Compose memetakan port TCP `APP_PORT` dan rentang UDP `APP_UDP_PORT_MIN–MAX`. Di Docker, **`APP_HOST` wajib diisi** (lihat di bawah).

## Environment

Salin `.env.example` ke `.env`.

| Variabel | Default | Keterangan |
|---|---|---|
| `APP_NAME` | – | Nama aplikasi |
| `APP_ENV` | `production` | `localhost` → bind `0.0.0.0:PORT`; lainnya `:PORT` |
| `APP_PORT` | `8080` | Port HTTP/WebSocket |
| `APP_HOST` | *(kosong)* | IP/hostname server yang **bisa dijangkau peserta** untuk media UDP. Isi IP LAN untuk rapat internal. Jangan isi domain yang diproxy Cloudflare/tunnel (UDP tidak lewat sana). |
| `APP_UDP_PORT_MIN` / `MAX` | `50000` / `50100` | Rentang port UDP media. Buka di firewall dan petakan di Docker. |
| `SWAGGER_ENABLED` | `true` jika `APP_ENV=localhost`, selain itu `false` | Tampilkan Swagger UI di `/swagger/index.html` |
| `STUN_SERVER` | `stun:stun.cloudflare.com:3478` | STUN untuk server dan klien |
| `CF_TURN_KEY_ID` | – | ID TURN key Cloudflare Realtime |
| `CF_TURN_API_TOKEN` | – | API token TURN key tersebut |
| `TURN_URLS` | – | Alternatif: URL TURN statis (pisah koma), mis. coturn |
| `TURN_USERNAME` / `TURN_CREDENTIAL` | – | Kredensial untuk `TURN_URLS` |

Pilih satu jalur TURN: Cloudflare (`CF_TURN_*`) **atau** statis (`TURN_*`). Tanpa keduanya, hanya STUN yang dipakai.

## API

Base URL lokal: `http://localhost:8080`. CORS terbuka (`*`). Semua respons JSON.

| Method | Path | Fungsi |
|---|---|---|
| GET | `/health` | Cek hidup |
| GET | `/webrtc/ice` | Daftar ICE server (STUN+TURN) untuk `RTCPeerConnection` |
| POST | `/webrtc/sdp/m/{meetingId}/c/{userId}/p/{peerId}/s/{isSender}` | Negosiasi SDP (publish/subscribe) |
| GET | `/webrtc/m/{meetingId}/peers` | Publisher aktif di rapat |
| GET | `/webrtc/m/{meetingId}/participants` | Snapshot peserta |
| GET | `/webrtc/meetings` | Rapat yang sedang aktif |
| GET (WS) | `/ws/m/{meetingId}?userId=&observer=` | Presence, chat, perintah admin |

### `GET /webrtc/ice`
```json
{"iceServers":[
  {"urls":["stun:stun.cloudflare.com:3478"]},
  {"urls":["turn:turn.cloudflare.com:3478?transport=udp","turns:turn.cloudflare.com:443?transport=tcp"],
   "username":"…","credential":"…"}
]}
```
Kredensial TURN dibuat via API Cloudflare dan di-cache di server.

### `POST /webrtc/sdp/m/{meetingId}/c/{userId}/p/{peerId}/s/{isSender}`
| Param | Arti |
|---|---|
| `meetingId` | ID rapat (bebas) |
| `userId` | ID pemanggil |
| `peerId` | Saat subscribe: `userId` publisher yang dituju. Saat publish: isi `-` |
| `isSender` | `true` = publish, `false` = subscribe ke `peerId` |

Body: `{"Sdp":"<base64(JSON offer)>"}` → respons `{"Sdp":"<base64(JSON answer)>"}`.
Galat: `400` (`isSender` bukan boolean / body salah), `500` (SDP tidak valid, dll.).

Contoh di browser:
```js
const offer = await pc.createOffer();
await pc.setLocalDescription(offer);
// tunggu iceGatheringState === "complete" (non-trickle)
const sdp = btoa(JSON.stringify(pc.localDescription));
const res = await fetch(`/webrtc/sdp/m/${meetingId}/c/${userId}/p/-/s/true`, {
  method: "POST", headers: {"Content-Type":"application/json"},
  body: JSON.stringify({ Sdp: sdp }),
});
const { Sdp } = await res.json();
await pc.setRemoteDescription(JSON.parse(atob(Sdp)));
```

### `GET /webrtc/m/{meetingId}/peers`
`{"peers":["u-abc","u-def"]}`: publisher aktif. Entri dihapus saat publisher putus.

### `GET /webrtc/m/{meetingId}/participants`
```json
{"participants":[{"userId":"u-abc","camera":true,"mic":false,"publishing":true,
  "quality":{"rttMs":42,"lossPct":0.3,"jitterMs":5,"bitrateKbps":900,"level":"good"}}],
 "blocked":[]}
```
`quality.level`: `good` | `fair` | `poor` (field `quality` muncul setelah sampel pertama).

### `GET /webrtc/meetings`
`{"meetings":[{"id":"abc-defg-hij","participants":3,"startedAt":1764200000000}]}` (`startedAt` = unix ms).

## WebSocket (presence & chat)

`GET /ws/m/{meetingId}?userId=<id>&observer=true|false`

- `userId` wajib (`400` bila kosong). `observer=true` = admin, tidak terdaftar sebagai peserta.
- `403` bila `userId` diblokir admin (sampai di-`allow`).
- Server mengirim ping tiap 15 detik; koneksi diputus bila tidak ada pong/pesan 40 detik.

**Server → klien**
| `type` | Isi |
|---|---|
| `state` | `{"type":"state","participants":[…],"blocked":[…]}` tiap ada perubahan |
| `history` | `{"type":"history","messages":[{id,from,text,at}]}` riwayat chat saat join |
| `chat` | `{"type":"chat","id":1,"from":"u-abc","text":"halo","at":1764200000000}` |
| `command` | `{"type":"command","action":"mute_mic\|camera_off\|kick"}` perintah dari admin |

**Klien → server**
```json
{"type":"media","camera":true,"mic":false}
{"type":"chat","text":"halo"}
{"type":"admin","action":"mute_mic|camera_off|kick|allow","target":"u-abc"}
```
`admin` hanya berlaku untuk `observer=true`. Teks chat maksimal 1000 karakter, riwayat 100 pesan.

Uji cepat: `npx wscat -c "ws://localhost:8080/ws/m/demo?userId=u1"`.

## Koneksi di luar LAN (TURN)

Media WebRTC berjalan lewat UDP langsung ke `APP_HOST:50000-50100`. Peserta di luar LAN tidak bisa menjangkau IP privat server, jadi dibutuhkan **relay TURN**: server dan peserta sama-sama terhubung keluar ke TURN, sehingga tidak ada port yang perlu dibuka.

- Isi `CF_TURN_KEY_ID` + `CF_TURN_API_TOKEN` (Dashboard Cloudflare → Realtime → TURN Server → Create). Pakai **Key ID TURN**, bukan App ID SFU.
- Untuk peer connection milik server, URL TURN dipilih berurutan **TCP → TLS → UDP**. Alasannya: pada beberapa jaringan server, UDP ke TURN tidak dijawab sehingga ICE gathering macet ±8 detik tanpa kandidat relay. TURN lewat TCP tetap menghasilkan alamat relay UDP.
- Klien mendapat semua URL dari `/webrtc/ice` dan memilih sendiri.

Alternatif tanpa TURN: beri `APP_HOST` IP publik dan forward UDP `50000-50100` di router (tidak bekerja di belakang CGNAT).

## Deploy

Workflow `.github/workflows/main.yml`: build image → push ke GHCR → SSH ke server → tulis `.env` dari GitHub Secrets → `docker compose pull && up -d`.

Secret yang dipakai:
`APP_ENV`, `APP_NAME`, `APP_HOST`, `APP_PORT`, `APP_UDP_PORT_MIN`, `APP_UDP_PORT_MAX`, `STUN_SERVER`, `CF_TURN_KEY_ID`, `CF_TURN_API_TOKEN`, serta `DEPLOY_HOST`, `DEPLOY_USER`, `DEPLOY_SSH_KEY`.

Catatan: secret `TURN_URLS/USERNAME/CREDENTIAL` belum diteruskan workflow. Tambahkan bila memakai coturn.

## Struktur kode

```
main.go        bootstrap: providers → gin → routes
providers/     DI wiring (samber/do), nama di pkg/constants
routes/        definisi endpoint saja
controller/    lapisan HTTP: parse/validasi, petakan error → status code
services/      logika bisnis (negosiasi WebRTC, forwarding track, presence)
core/          infrastruktur: Pion, TrackRegistry, SDP, ICE provider, presence hub
config/        pemuatan env
middlewares/   CORS
pkg/constants/ kunci DI dan konstanta
docs/          Swagger hasil generate (jangan diedit tangan)
```
Arah dependensi: `routes → controller → services → core`. Jangan impor ke atas.

## Test

```bash
make test
```
Test berada di sebelah kodenya (`*_test.go`). `services/sdp_service_test.go` berisi tes end-to-end publish/subscribe memakai klien Pion, tanpa browser.

## Troubleshooting

| Gejala | Penyebab umum |
|---|---|
| "Network issues, reconnecting…" di UI | Peserta di luar LAN tanpa relay: cek `CF_TURN_*`, dan pastikan `/webrtc/ice` berisi entri `turn:`. `POST /webrtc/sdp` yang selalu ±8 detik = server tidak mendapat kandidat relay. |
| `404` saat membuat kredensial TURN | `CF_TURN_KEY_ID` bukan Key ID TURN (jangan pakai App ID SFU), atau key sudah dihapus |
| Tidak ada video antar peserta di Docker | `APP_HOST` kosong/salah, atau rentang UDP belum dipetakan/dibuka di firewall |
| `403` saat membuka WebSocket | `userId` diblokir admin (`kick`); kirim aksi `allow` |
| Log `ice: cloudflare turn: …` | Kredensial TURN salah/kedaluwarsa; server mencoba lagi tiap menit |

> Server ini tidak punya autentikasi: siapa pun yang tahu URL dapat bergabung atau menjadi admin (`observer=true`). Jangan dipakai untuk data sensitif.
