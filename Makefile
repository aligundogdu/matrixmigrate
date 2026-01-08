# MatrixMigrate Makefile

# Application info
APP_NAME := matrixmigrate
MAIN_PKG := ./cmd/matrixmigrate

# Version info (can be overridden)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Build flags
LDFLAGS := -ldflags "-X github.com/aligundogdu/matrixmigrate/internal/version.Version=$(VERSION) \
	-X github.com/aligundogdu/matrixmigrate/internal/version.GitCommit=$(GIT_COMMIT) \
	-X github.com/aligundogdu/matrixmigrate/internal/version.BuildTime=$(BUILD_TIME)"

# Go commands
GO := go
GOBUILD := $(GO) build $(LDFLAGS)
GOTEST := $(GO) test
GOCLEAN := $(GO) clean
GOMOD := $(GO) mod

.PHONY: all build clean test deps version help release

# Default target
all: build

# Build the application
build:
	@echo "Building $(APP_NAME)..."
	@echo "Version: $(VERSION)"
	@echo "Commit:  $(GIT_COMMIT)"
	@echo "Time:    $(BUILD_TIME)"
	$(GOBUILD) -o $(APP_NAME) $(MAIN_PKG)
	@echo "Build complete: ./$(APP_NAME)"

# Build for production (with optimizations)
release:
	@echo "Building release version..."
	$(GOBUILD) -o $(APP_NAME) -trimpath $(MAIN_PKG)
	@echo "Release build complete: ./$(APP_NAME)"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -f $(APP_NAME)

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# Show version info
version:
	@echo "Version: $(VERSION)"
	@echo "Commit:  $(GIT_COMMIT)"
	@echo "Time:    $(BUILD_TIME)"

# Cross-compile for different platforms
build-all: build-linux build-darwin build-windows

build-linux:
	@echo "Building for Linux..."
	GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(APP_NAME)-linux-amd64 $(MAIN_PKG)

build-darwin:
	@echo "Building for macOS..."
	GOOS=darwin GOARCH=amd64 $(GOBUILD) -o $(APP_NAME)-darwin-amd64 $(MAIN_PKG)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) -o $(APP_NAME)-darwin-arm64 $(MAIN_PKG)

build-windows:
	@echo "Building for Windows..."
	GOOS=windows GOARCH=amd64 $(GOBUILD) -o $(APP_NAME)-windows-amd64.exe $(MAIN_PKG)

# Help
help:
	@echo "MatrixMigrate Build System"
	@echo ""
	@echo "Usage:"
	@echo "  make              Build the application"
	@echo "  make build        Build the application"
	@echo "  make release      Build optimized release version"
	@echo "  make clean        Remove build artifacts"
	@echo "  make test         Run tests"
	@echo "  make deps         Download and tidy dependencies"
	@echo "  make version      Show version information"
	@echo "  make build-all    Cross-compile for all platforms"
	@echo ""
	@echo "Build with custom version:"
	@echo "  make VERSION=1.0.0 build"
