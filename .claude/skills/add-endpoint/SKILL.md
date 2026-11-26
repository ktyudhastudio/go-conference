---
name: add-endpoint
description: Add a new HTTP endpoint to go-conference following the routes → controller → services → core layering, with DI registration and tests. Use when asked to add an API/endpoint/feature.
---

# Add an endpoint

1. **Service** — `services/<name>_service.go`: struct + `New<Name>Service(...)`. Business logic only, no gin. Return errors.
2. **Controller** — `controller/<name>_controller.go`: struct holding the service; handler methods bind/validate input and map service errors to status codes.
3. **Constants** — add DI keys in `pkg/constants/common.go`.
4. **Provider** — register service then controller in `providers/core.go` (`do.ProvideNamed`, resolve deps with `do.MustInvokeNamed`).
5. **Route** — in `routes/routes.go` resolve the controller from the injector and add the route to a group.
6. **Core** — only if new infrastructure is needed (shared state, pion setup, helpers) put it in `core/`.
7. **Tests** — service test in `services/`, controller test with `httptest` in `controller/`. Follow existing `*_test.go` files.
8. Run `go vet ./... && make test`; update the Structure/Endpoint notes in `CLAUDE.md` if they changed.

No authentication is required (project is experimental).
