---
name: run-tests
description: Run go-conference tests and interpret failures. Use when asked to test, verify, or check the server works.
---

# Run tests

1. `go vet ./...`
2. `make test` (or `go test ./... -count=1`; add `-race` for concurrency changes, `-run <Name>` for one test).
3. Smoke test the server: `make run` then `curl localhost:8080/health` → `{"status":"ok"}`.

Notes:
- `TestPublishAndSubscribeEndToEnd` (services) uses loopback WebRTC and takes a few seconds; a failure with "subscriber received no RTP" means publish→registry→subscribe forwarding is broken.
- Controller tests expect 400 for bad `isSender`/JSON and 500 with `invalid sdp` for undecodable SDP.
- Report failing test names and output verbatim; don't claim success if anything was skipped.
