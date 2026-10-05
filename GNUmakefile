# Go build variants: release (no tag), dev and testauth.
GO_TAGS = "" dev testauth

GOLANGCI_LINT = github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0
AIR           = github.com/air-verse/air@v1.67.4

# Build information for the release binary and image, see internal/buildinfo.
VERSION   = $(shell git describe --tags --exact-match 2>/dev/null)
COMMIT    = $(shell git rev-parse HEAD)
BUILDINFO = github.com/digineo/sitrep/internal/buildinfo
LDFLAGS   = -s -w \
            -X $(BUILDINFO).Version=$(VERSION) \
            -X $(BUILDINFO).Commit=$(COMMIT) \
            -X $(BUILDINFO).Date=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@awk -F ':.*## ' '/^[a-z0-9-]+:.*## / { printf "  %-14s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

frontend/node_modules: frontend/package-lock.json
	cd frontend && npm ci
	@touch $@

.PHONY: frontend-dist
frontend-dist: frontend/node_modules ## Build the frontend into frontend/dist/app
	cd frontend && npx vite build

.PHONY: build
build: frontend-dist ## Build the release binary ./sitrep
	go build -trimpath -ldflags "$(LDFLAGS)" -o sitrep ./cmd/sitrep

.PHONY: docker-build
docker-build: ## Build the Docker image sitrep
	docker build --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) -t sitrep .

.PHONY: dev
dev: frontend/node_modules ## Run Vite and the Go server with live reload, configured by .env.local
	trap 'kill 0' EXIT; \
	(cd frontend && npx vite) & \
	go run $(AIR) \
		-build.cmd "go build -tags 'dev testauth' -o tmp/sitrep ./cmd/sitrep" \
		-build.entrypoint tmp/sitrep \
		-build.args_bin serve \
		-build.exclude_dir frontend,tmp \
		-build.include_ext go,json

.PHONY: test
test: test-backend test-frontend test-e2e ## Run all test suites

.PHONY: test-backend
test-backend: ## Run the Go tests
	go test ./...
	go test -tags testauth ./...

.PHONY: test-frontend
test-frontend: frontend/node_modules ## Run the Vitest tests
	cd frontend && npx vitest run

.PHONY: test-e2e
test-e2e: frontend-dist ## Run the Playwright tests against a testauth build
	go build -tags testauth -o sitrep-e2e ./cmd/sitrep
	cd frontend && npx playwright test

.PHONY: lint
lint: frontend/node_modules ## Run go vet and golangci-lint for every build variant, then ESLint and vue-tsc
	for tags in $(GO_TAGS); do \
		go vet -tags "$$tags" ./... && \
		go run $(GOLANGCI_LINT) run --build-tags "$$tags" ./... || exit 1; \
	done
	cd frontend && npx eslint .
	cd frontend && npx vue-tsc --noEmit
