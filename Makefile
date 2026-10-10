.PHONY: dev-api dev-web check test build

VERSION ?= dev

dev-api:
	go run ./cmd/faultharbor

dev-web:
	npm --prefix web run dev

check:
	test -z "$$(gofmt -l cmd internal)"
	go vet ./cmd/... ./internal/...
	go test -race ./cmd/... ./internal/...
	npm --prefix web run lint
	npm --prefix web run typecheck
	npm --prefix web test

test:
	go test -race ./cmd/... ./internal/...
	npm --prefix web test

build:
	npm --prefix web run build
	go build -trimpath -ldflags "-X main.version=$(VERSION)" -o bin/faultharbor ./cmd/faultharbor
