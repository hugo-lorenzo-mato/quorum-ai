# Metadata
VERSION ?= $(shell git describe --tags --always --dirty)
COMMIT  ?= $(shell git rev-parse --short HEAD)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

# Go settings
GOBIN      ?= $(shell go env GOPATH)/bin
GOOS       ?= $(shell go env GOOS)
GOARCH     ?= $(shell go env GOARCH)
CGO_ENABLED ?= 0

# Build flags
LDFLAGS := -s -w \
    -X main.version=$(VERSION) \
    -X main.commit=$(COMMIT) \
    -X main.date=$(DATE)

# Directories
BIN_DIR    := bin
DIST_DIR   := dist
COVER_FILE := coverage.out

# Default target
.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

.PHONY: all
all: lint test build ## Run lint, test, and build

.PHONY: check
check: lint test security frontend-check ## Run all checks (Go + frontend)

# Build targets
.PHONY: build
build: ## Build for current OS/arch
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=$(CGO_ENABLED) go build -ldflags "$(LDFLAGS)" \
		-o $(BIN_DIR)/quorum ./cmd/quorum

.PHONY: build-all
build-all: ## Build for all platforms
	@mkdir -p $(DIST_DIR)
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" \
		-o $(DIST_DIR)/quorum-linux-amd64 ./cmd/quorum
	GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" \
		-o $(DIST_DIR)/quorum-linux-arm64 ./cmd/quorum
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" \
		-o $(DIST_DIR)/quorum-darwin-amd64 ./cmd/quorum
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" \
		-o $(DIST_DIR)/quorum-darwin-arm64 ./cmd/quorum
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" \
		-o $(DIST_DIR)/quorum-windows-amd64.exe ./cmd/quorum

.PHONY: install
install: build ## Install to GOPATH/bin
	cp $(BIN_DIR)/quorum $(GOBIN)/quorum

# Test targets
.PHONY: test
test: ## Run unit tests
	go test -race -v ./...

.PHONY: test-short
test-short: ## Run unit tests (short mode)
	go test -race -short ./...

.PHONY: test-coverage
test-coverage: ## Run tests with coverage
	go test -race -coverprofile=$(COVER_FILE) -covermode=atomic ./...
	go tool cover -func=$(COVER_FILE)

.PHONY: cover
cover: test-coverage ## Open coverage report in browser
	go tool cover -html=$(COVER_FILE)

.PHONY: bench
bench: ## Run benchmarks
	go test -bench=. -benchmem ./...

.PHONY: fuzz
fuzz: ## Run fuzz tests (30s per target)
	go test -fuzz=Fuzz -fuzztime=30s ./internal/config/...
	go test -fuzz=Fuzz -fuzztime=30s ./internal/core/...

.PHONY: test-integration
test-integration: ## Run integration tests
	go test -race -v -tags=integration -timeout 20m ./...

.PHONY: test-e2e
test-e2e: build ## Run end-to-end tests
	go test -race -v -tags=e2e -timeout 30m ./tests/e2e/...

.PHONY: test-all
test-all: test test-integration test-e2e frontend-test ## Run all tests

# Quality targets
.PHONY: lint
lint: ## Run golangci-lint
	golangci-lint run

.PHONY: lint-fix
lint-fix: ## Run golangci-lint with auto-fix
	golangci-lint run --fix

.PHONY: fmt
fmt: ## Format code
	gofmt -s -w .
	goimports -w .

.PHONY: vet
vet: ## Run go vet
	go vet ./...

# Security targets
.PHONY: security
security: ## Run security checks
	govulncheck ./...
	gosec -quiet -exclude-dir=.gomodcache -exclude-dir=vendor -exclude-dir=testdata -exclude-dir=.worktrees ./...
	@$(MAKE) trivy

# Analysis targets (complementary to SonarCloud)
.PHONY: analyze
analyze: ## Run local analysis (struct alignment, performance hints)
	@echo "=== Local Analysis (SonarCloud complement) ==="
	@echo ""
	@echo "[1/2] Struct alignment..."
	@if [ -f $(GOBIN)/betteralign ]; then \
		$(GOBIN)/betteralign ./internal/... 2>&1 | head -20; \
	else \
		echo "  Install: make tools"; \
	fi
	@echo ""
	@echo "[2/2] Performance hints (hugeParam, rangeValCopy)..."
	@if [ -f $(GOBIN)/gocritic ]; then \
		$(GOBIN)/gocritic check -enableAll ./internal/... 2>&1 | grep -E "hugeParam|rangeValCopy|appendCombine" | head -20; \
	else \
		echo "  Install: make tools"; \
	fi
	@echo ""
	@echo "Full analysis: https://sonarcloud.io/project/overview?id=hugo-lorenzo-mato_quorum-ai"

.PHONY: trivy
trivy: ## Scan dependencies with Trivy (fails on findings)
	@echo "Trivy scan..."
	@if command -v trivy >/dev/null; then \
		trivy fs . --scanners vuln --quiet --exit-code 1 --severity MEDIUM,HIGH,CRITICAL \
			--skip-dirs .git \
			--skip-dirs .worktrees \
			--skip-dirs .quorum \
			--skip-dirs .orchestrator \
			--skip-dirs .gocache \
			--skip-dirs .gomodcache \
			--skip-dirs frontend/node_modules; \
	elif command -v docker >/dev/null; then \
		docker run --rm -v $(PWD):/src aquasec/trivy:latest fs /src --scanners vuln --quiet --exit-code 1 --severity MEDIUM,HIGH,CRITICAL \
			--skip-dirs .git \
			--skip-dirs .worktrees \
			--skip-dirs .quorum \
			--skip-dirs .orchestrator \
			--skip-dirs .gocache \
			--skip-dirs .gomodcache \
			--skip-dirs frontend/node_modules; \
	else \
		echo "  Requires trivy or docker"; \
		exit 1; \
	fi

.PHONY: sonar-report
sonar-report: ## Download SonarCloud report locally (requires SONAR_TOKEN)
	@./scripts/sonar-report.sh

# Release targets
.PHONY: release-dry
release-dry: ## Dry-run goreleaser
	goreleaser release --snapshot --clean --skip=publish

.PHONY: release
release: ## Create release (requires GITHUB_TOKEN)
	goreleaser release --clean

# Utility targets
.PHONY: clean
clean: ## Clean build artifacts
	rm -rf $(BIN_DIR) $(DIST_DIR) $(COVER_FILE)
	go clean -cache -testcache

.PHONY: deps
deps: ## Download dependencies
	go mod download
	go mod verify

.PHONY: tools
tools: ## Install analysis tools
	go install github.com/dkorunic/betteralign/cmd/betteralign@latest
	go install github.com/go-critic/go-critic/cmd/gocritic@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8
	go install github.com/uudashr/gocognit/cmd/gocognit@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest

.PHONY: tidy
tidy: ## Tidy go.mod
	go mod tidy

.PHONY: version
version: ## Show version
	@echo "Version: $(VERSION)"
	@echo "Commit:  $(COMMIT)"
	@echo "Date:    $(DATE)"

.PHONY: generate
generate: ## Run go generate
	go generate ./...

# Frontend targets
FRONTEND_DIR := frontend

.PHONY: frontend-deps
frontend-deps: ## Install frontend dependencies
	cd $(FRONTEND_DIR) && if [ -n "$$CI" ]; then npm ci; else npm install; fi

.PHONY: frontend-lint
frontend-lint: ## Run frontend lint
	cd $(FRONTEND_DIR) && npm run lint

.PHONY: frontend-test
frontend-test: ## Run frontend tests
	cd $(FRONTEND_DIR) && npm run test:run

.PHONY: frontend-coverage
frontend-coverage: ## Run frontend tests with coverage
	cd $(FRONTEND_DIR) && npm run test:coverage

.PHONY: frontend-audit
frontend-audit: frontend-deps ## Run frontend security audit
	cd $(FRONTEND_DIR) && npm audit

.PHONY: frontend-check
frontend-check: frontend-deps frontend-lint frontend-test build-frontend frontend-audit ## Run all frontend checks

.PHONY: build-frontend
build-frontend: frontend-deps ## Build frontend for production
	cd $(FRONTEND_DIR) && npm run build

.PHONY: dev-frontend
dev-frontend: ## Run frontend dev server
	cd $(FRONTEND_DIR) && npm run dev

.PHONY: build-web
build-web: build-frontend build ## Build frontend and Go binary with embedded assets
