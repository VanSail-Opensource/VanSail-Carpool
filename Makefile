SHELL := /bin/sh

GO ?= go
NPM ?= npm
DOCKER ?= docker
BINARY := bin/vansail-carpool
COVERAGE_MIN ?= 80
DOMAIN_PACKAGES := ./internal/accounts ./internal/admin ./internal/billing ./internal/catalog ./internal/entitlements
IMAGE ?= vansail-carpool:local

.DEFAULT_GOAL := build

.PHONY: api-types build ci clean docker-build frontend-audit frontend-build \
	frontend-install frontend-test frontend-typecheck go-audit go-build go-fmt-check \
	go-test go-vet govulncheck lint run test

frontend-install:
	cd web && $(NPM) ci

api-types: frontend-install
	cd web && $(NPM) run generate:api

frontend-typecheck: frontend-install
	cd web && $(NPM) run typecheck

frontend-test: frontend-install
	cd web && $(NPM) run test

frontend-build: frontend-install
	cd web && $(NPM) run generate:api
	cd web && $(NPM) run build

frontend-audit: frontend-build
	cd web && $(NPM) run lint
	cd web && $(NPM) run typecheck
	cd web && $(NPM) run test
	cd web && $(NPM) audit --audit-level=high

go-fmt-check:
	@test -z "$$(gofmt -l $$(find cmd internal -type f -name '*.go'))" || \
		(printf '%s\n' 'Go files require gofmt' && gofmt -l $$(find cmd internal -type f -name '*.go') && exit 1)

go-vet: frontend-build
	$(GO) vet ./...

lint: frontend-build
	golangci-lint run ./...

go-test: frontend-build
	$(GO) test -race ./...
	$(GO) test -covermode=atomic -coverprofile=domain-coverage.out $(DOMAIN_PACKAGES)
	@$(GO) tool cover -func=domain-coverage.out | awk -v min=$(COVERAGE_MIN) \
		'/^total:/ { gsub(/%/, "", $$3); if (($$3 + 0) < min) { printf "coverage %.1f%% is below %.1f%%\n", $$3, min; exit 1 } }'

govulncheck: frontend-build
	govulncheck ./...

go-audit: frontend-build go-fmt-check go-vet lint go-test govulncheck

go-build: frontend-build
	mkdir -p bin
	CGO_ENABLED=0 $(GO) build -trimpath -o $(BINARY) ./cmd/server

build: go-build

test: frontend-test go-test

ci: frontend-audit go-audit docker-build

run: frontend-build
	$(GO) run ./cmd/server serve

docker-build:
	$(DOCKER) build -t $(IMAGE) .

clean:
	rm -rf bin web/coverage
	find internal/webui/dist -mindepth 1 -maxdepth 1 ! -name .placeholder -exec rm -rf {} +
	rm -f coverage.out domain-coverage domain-coverage.out coverage.html
