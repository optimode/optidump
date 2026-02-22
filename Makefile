.PHONY: build build-prod clean install test fmt vet check security \
       build-linux-amd64 build-linux-arm64 build-darwin-amd64 build-darwin-arm64 build-all

# Binary name
BINARY=optidump
# Build directory
BUILD_DIR=bin
# Main package path
CMD_PATH=./cmd/optidump

# Build metadata
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

# ===========================================================================
# SECURITY HARDENING - Build Flags (CGO-Free)
# ===========================================================================

# Base ldflags: version info injection
LDFLAGS_BASE := -X main.version=$(VERSION) \
                -X main.gitCommit=$(GIT_COMMIT) \
                -X main.buildTime=$(BUILD_TIME)

# Production ldflags: strip symbols + remove build ID
# -s        : Strip debugging information (anti-reverse engineering)
# -w        : Omit DWARF symbol table (further size reduction)
# -buildid= : Remove build ID (reproducible builds, privacy)
LDFLAGS_PRODUCTION := $(LDFLAGS_BASE) -s -w -buildid=''

# Development ldflags: keep symbols for debugging
LDFLAGS_DEV := $(LDFLAGS_BASE)

# Go build flags
# -trimpath      : Remove absolute file paths from binary (privacy)
# -buildvcs=false: Disable VCS stamping (required for CI environments)
BUILDFLAGS := -trimpath -buildvcs=false
BUILDFLAGS_DEV := -trimpath

# Static binary (no CGO)
BUILD_ENV := CGO_ENABLED=0

# ===========================================================================
# Development
# ===========================================================================

# Build development binary (native platform, with debug symbols)
build:
	@mkdir -p $(BUILD_DIR)
	$(BUILD_ENV) go build $(BUILDFLAGS_DEV) -ldflags '$(LDFLAGS_DEV)' -o $(BUILD_DIR)/$(BINARY) $(CMD_PATH)

# ===========================================================================
# Production
# ===========================================================================

# Build production binary (native platform, hardened)
build-prod:
	@mkdir -p $(BUILD_DIR)
	$(BUILD_ENV) go build $(BUILDFLAGS) -ldflags '$(LDFLAGS_PRODUCTION)' -o $(BUILD_DIR)/$(BINARY) $(CMD_PATH)

# ===========================================================================
# Cross-compilation (production, hardened)
# ===========================================================================

build-linux-amd64:
	@mkdir -p $(BUILD_DIR)
	$(BUILD_ENV) GOOS=linux GOARCH=amd64 go build $(BUILDFLAGS) -ldflags '$(LDFLAGS_PRODUCTION)' -o $(BUILD_DIR)/$(BINARY)-linux-amd64 $(CMD_PATH)

build-linux-arm64:
	@mkdir -p $(BUILD_DIR)
	$(BUILD_ENV) GOOS=linux GOARCH=arm64 go build $(BUILDFLAGS) -ldflags '$(LDFLAGS_PRODUCTION)' -o $(BUILD_DIR)/$(BINARY)-linux-arm64 $(CMD_PATH)

build-darwin-amd64:
	@mkdir -p $(BUILD_DIR)
	$(BUILD_ENV) GOOS=darwin GOARCH=amd64 go build $(BUILDFLAGS) -ldflags '$(LDFLAGS_PRODUCTION)' -o $(BUILD_DIR)/$(BINARY)-darwin-amd64 $(CMD_PATH)

build-darwin-arm64:
	@mkdir -p $(BUILD_DIR)
	$(BUILD_ENV) GOOS=darwin GOARCH=arm64 go build $(BUILDFLAGS) -ldflags '$(LDFLAGS_PRODUCTION)' -o $(BUILD_DIR)/$(BINARY)-darwin-arm64 $(CMD_PATH)

# Build all platforms (production, hardened)
build-all: build-linux-amd64 build-linux-arm64 build-darwin-amd64 build-darwin-arm64

# ===========================================================================
# Quality & Security
# ===========================================================================

clean:
	rm -rf $(BUILD_DIR)
	go clean

install:
	$(BUILD_ENV) go install $(BUILDFLAGS) -ldflags '$(LDFLAGS_PRODUCTION)' $(CMD_PATH)

test:
	go test -v ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

# Run all checks before committing
check: fmt vet test

# Run security vulnerability check (requires: go install golang.org/x/vuln/cmd/govulncheck@latest)
security:
	govulncheck ./...
