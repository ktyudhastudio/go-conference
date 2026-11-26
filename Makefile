ifneq (,$(wildcard ./.env))
	include .env
	export $(shell sed 's/=.*//' .env)
endif

IMAGE ?= ghcr.io/ktyudha/go-conference:latest

.DEFAULT_GOAL := help
.PHONY: help swag dep run build run-build test docker-build docker-run docker-push deploy

help: ## Tampilkan daftar perintah
	@echo "Usage: make <target>"
	@echo ""
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z_-]+:.*## / {printf "  \033[32m%-12s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

swag: ## Generate docs/ (OpenAPI) dari anotasi handler
	go run github.com/swaggo/swag/cmd/swag@v1.16.4 init -g main.go -o docs --outputTypes go,json,yaml --parseDependency --parseInternal

dep: ## Rapikan dependency (go mod tidy)
	go mod tidy

run: ## Jalankan server (go run)
	go run main.go

build: ## Build binary ke ./main
	go build -o main main.go

run-build: build ## Build lalu jalankan binary
	./main

test: ## Jalankan semua test
	go test -v ./...

docker-build: ## Build image Docker
	docker build -t $(IMAGE) .

docker-run: ## Jalankan container (pakai .env)
	docker compose up -d --build

docker-push: ## Push image ke GHCR (perlu docker login ghcr.io)
	docker buildx build --platform linux/amd64 -t $(IMAGE) --push .

deploy: ## Di server: tarik image terbaru & restart
	docker compose pull && docker compose up -d
