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
	go test -fuzz=Fuzz -fuzztime=30s ./internal/service/...

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
	gosec -quiet ./...

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
