# Makefile for Mac Storage Cleaner
# Project: github.com/rahulvramesh/cleanWithCli

# Variables
BINARY_NAME=mac-cleaner
BINARY_PATH=./$(BINARY_NAME)
MODULE_NAME=github.com/rahulvramesh/cleanWithCli
GO_VERSION=1.24.5

# Build info
BUILD_TIME=$(shell date +%FT%T%z)
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

# Go build flags
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.gitCommit=$(GIT_COMMIT)"

# Colors for output
RED=\033[0;31m
GREEN=\033[0;32m
YELLOW=\033[0;33m
BLUE=\033[0;34m
NC=\033[0m # No Color

.PHONY: help build run test clean install deps vendor mod-tidy lint fmt vet check dev install-tools release

# Default target
.DEFAULT_GOAL := help

## help: Show this help message
help:
	@echo "$(BLUE)Mac Storage Cleaner - Available commands:$(NC)"
	@echo ""
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'
	@echo ""

## build: Build the application
build:
	@echo "$(YELLOW)Building $(BINARY_NAME)...$(NC)"
	@go build $(LDFLAGS) -o $(BINARY_PATH) ./cmd/mac-cleaner
	@echo "$(GREEN)✓ Built $(BINARY_PATH)$(NC)"

## run: Run the application
run:
	@echo "$(YELLOW)Running $(BINARY_NAME)...$(NC)"
	@go run ./cmd/mac-cleaner 

## dev: Run the application in development mode (rebuild on changes)
dev: build
	@echo "$(YELLOW)Starting $(BINARY_NAME) in development mode...$(NC)"
	@./$(BINARY_NAME)

## test: Run tests
test:
	@echo "$(YELLOW)Running tests...$(NC)"
	@go test -v ./...
	@echo "$(GREEN)✓ Tests completed$(NC)"

## test-coverage: Run tests with coverage
test-coverage:
	@echo "$(YELLOW)Running tests with coverage...$(NC)"
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)✓ Coverage report generated: coverage.html$(NC)"

## bench: Run benchmarks
bench:
	@echo "$(YELLOW)Running benchmarks...$(NC)"
	@go test -bench=. -benchmem ./...

## clean: Clean build artifacts
clean:
	@echo "$(YELLOW)Cleaning build artifacts...$(NC)"
	@rm -f $(BINARY_PATH)
	@rm -f coverage.out coverage.html
	@go clean -cache -testcache -modcache
	@echo "$(GREEN)✓ Cleaned$(NC)"

## install: Install the application to GOPATH/bin
install:
	@echo "$(YELLOW)Installing $(BINARY_NAME)...$(NC)"
	@go install $(LDFLAGS) .
	@echo "$(GREEN)✓ Installed to $(shell go env GOPATH)/bin/$(BINARY_NAME)$(NC)"

## deps: Download dependencies
deps:
	@echo "$(YELLOW)Downloading dependencies...$(NC)"
	@go mod download
	@echo "$(GREEN)✓ Dependencies downloaded$(NC)"

## vendor: Create vendor directory
vendor:
	@echo "$(YELLOW)Creating vendor directory...$(NC)"
	@go mod vendor
	@echo "$(GREEN)✓ Vendor directory created$(NC)"

## mod-tidy: Tidy up go.mod
mod-tidy:
	@echo "$(YELLOW)Tidying go.mod...$(NC)"
	@go mod tidy
	@echo "$(GREEN)✓ go.mod tidied$(NC)"

## fmt: Format Go code
fmt:
	@echo "$(YELLOW)Formatting Go code...$(NC)"
	@go fmt ./...
	@echo "$(GREEN)✓ Code formatted$(NC)"

## vet: Run go vet
vet:
	@echo "$(YELLOW)Running go vet...$(NC)"
	@go vet ./...
	@echo "$(GREEN)✓ go vet completed$(NC)"

## lint: Run linter (requires golangci-lint)
lint:
	@echo "$(YELLOW)Running linter...$(NC)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
		echo "$(GREEN)✓ Linting completed$(NC)"; \
	else \
		echo "$(RED)✗ golangci-lint not found. Run 'make install-tools' to install it$(NC)"; \
	fi

## check: Run all checks (fmt, vet, lint, test)
check: fmt vet lint test
	@echo "$(GREEN)✓ All checks completed$(NC)"

## install-tools: Install development tools
install-tools:
	@echo "$(YELLOW)Installing development tools...$(NC)"
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "$(GREEN)✓ Development tools installed$(NC)"

## release: Build release version (optimized)
release:
	@echo "$(YELLOW)Building release version...$(NC)"
	@CGO_ENABLED=0 go build -trimpath $(LDFLAGS) -a -installsuffix cgo -o $(BINARY_PATH) .
	@echo "$(GREEN)✓ Release build completed: $(BINARY_PATH)$(NC)"

## cross-compile: Build for multiple Darwin architectures
cross-compile:
	@echo "$(YELLOW)Cross-compiling for Darwin architectures...$(NC)"
	@mkdir -p dist
	@echo "  Building for darwin/amd64..."
	@CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-amd64 .
	@echo "  Building for darwin/arm64..."
	@CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-arm64 .
	@echo "$(GREEN)✓ Cross-compilation completed. Binaries in ./dist/$(NC)"

## info: Show project information
info:
	@echo "$(BLUE)Project Information:$(NC)"
	@echo "  Name: $(BINARY_NAME)"
	@echo "  Module: $(MODULE_NAME)"
	@echo "  Go Version: $(GO_VERSION)"
	@echo "  Version: $(VERSION)"
	@echo "  Git Commit: $(GIT_COMMIT)"
	@echo "  Build Time: $(BUILD_TIME)"
	@echo ""
	@echo "$(BLUE)Environment:$(NC)"
	@echo "  Go Version: $(shell go version)"
	@echo "  GOPATH: $(shell go env GOPATH)"
	@echo "  GOROOT: $(shell go env GOROOT)"

## size: Show binary size
size: build
	@echo "$(BLUE)Binary size:$(NC)"
	@ls -lh $(BINARY_PATH) | awk '{print "  " $$5 " - " $$9}'

## deps-update: Update all dependencies to latest versions
deps-update:
	@echo "$(YELLOW)Updating dependencies...$(NC)"
	@go get -u ./...
	@go mod tidy
	@echo "$(GREEN)✓ Dependencies updated$(NC)"

## deps-check: Check for outdated dependencies
deps-check:
	@echo "$(YELLOW)Checking for outdated dependencies...$(NC)"
	@if command -v go-mod-outdated >/dev/null 2>&1; then \
		go list -u -m -json all | go-mod-outdated -update -direct; \
	else \
		echo "$(YELLOW)go-mod-outdated not found. Install with: go install github.com/psampaz/go-mod-outdated@latest$(NC)"; \
		go list -u -m all; \
	fi
