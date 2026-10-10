.PHONY: dev-api dev-web check fmt fmt-check lint lint-go lint-web test test-go test-web coverage security build test-integration

VERSION ?= dev
GOLANGCI_LINT ?= golangci-lint
GO_PACKAGES := ./cmd/... ./internal/...

dev-api:
	go run ./cmd/faultharbor

dev-web:
	npm --prefix web run dev

check: fmt-check lint test

fmt:
	gofmt -w cmd internal

fmt-check:
	test -z "$$(gofmt -l cmd internal)"

lint: lint-go lint-web

lint-go:
	$(GOLANGCI_LINT) run $(GO_PACKAGES)

lint-web:
	npm --prefix web run lint
	npm --prefix web run typecheck

test: test-go test-web

test-go:
	go test -race -shuffle=on -count=1 -timeout=2m $(GO_PACKAGES)

test-web:
	npm --prefix web test

coverage:
	mkdir -p coverage
	go test -race -shuffle=on -count=1 -timeout=2m -covermode=atomic -coverprofile=coverage/go.out $(GO_PACKAGES)
	go tool cover -func=coverage/go.out
	go tool cover -html=coverage/go.out -o coverage/go.html

security:
	go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 $(GO_PACKAGES)
	npm --prefix web audit --audit-level=high

build:
	npm --prefix web run build
	go build -trimpath -ldflags "-X main.version=$(VERSION)" -o bin/faultharbor ./cmd/faultharbor
	go build -trimpath -o bin/faultharbor-collector ./cmd/faultharbor-collector

test-integration:
	sh tests/integration/docker-inspection.sh
