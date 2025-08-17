
# ==============================================================================
# Makefile for splunk-cli
#
# This Makefile provides commands for building, testing, linting, and cleaning
# the splunk-cli project.
# ==============================================================================

# --- Configuration ---
# Get the version from the latest git tag
VERSION ?= $(shell git describe --tags --abbrev=0 --exact-match 2>/dev/null || echo "v0.1.0")
# Get the git commit hash
GIT_COMMIT ?= $(shell git rev-parse --short HEAD)
# Get the build date
BUILD_DATE ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOCLEAN=$(GOCMD) clean
GOLINT=golangci-lint run
GOVULNCHECK=govulncheck

# Source and output configuration
SOURCE_FILE=splunk-cli.go
OUTPUT_NAME=splunk-cli
DIST_DIR=dist

# --- Build Targets ---

.PHONY: all build clean test lint vulncheck package help

all: build

# Build binaries for all target platforms
build: build-macos build-linux build-windows

# Package binaries for release
package: clean build
	@echo "📦 Packaging binaries for release..."
	@cd ./${DIST_DIR}/macos && zip ../${OUTPUT_NAME}_${VERSION}_macos_universal.zip ${OUTPUT_NAME} && cd ../..
	@cd ./${DIST_DIR}/linux && zip ../${OUTPUT_NAME}_${VERSION}_linux_amd64.zip ${OUTPUT_NAME} && cd ../..
	@cd ./${DIST_DIR}/windows && zip ../${OUTPUT_NAME}_${VERSION}_windows_amd64.zip ${OUTPUT_NAME}.exe && cd ../..
	@echo "✅ Packaging complete. Archives and binaries are in ./${DIST_DIR}/"

# Build for macOS (Universal Binary)
build-macos:
	@echo "📦 Building for macOS (Universal)..."
	@mkdir -p ./${DIST_DIR}/macos
	@GOOS=darwin GOARCH=amd64 $(GOBUILD) -ldflags="-X 'main.Version=${VERSION}' -X 'main.Commit=${GIT_COMMIT}' -X 'main.Date=${BUILD_DATE}'" -o ./${DIST_DIR}/macos/${OUTPUT_NAME}_amd64 ./${SOURCE_FILE}
	@GOOS=darwin GOARCH=arm64 $(GOBUILD) -ldflags="-X 'main.Version=${VERSION}' -X 'main.Commit=${GIT_COMMIT}' -X 'main.Date=${BUILD_DATE}'" -o ./${DIST_DIR}/macos/${OUTPUT_NAME}_arm64 ./${SOURCE_FILE}
	@lipo -create -output ./${DIST_DIR}/macos/${OUTPUT_NAME} ./${DIST_DIR}/macos/${OUTPUT_NAME}_amd64 ./${DIST_DIR}/macos/${OUTPUT_NAME}_arm64
	@rm ./${DIST_DIR}/macos/${OUTPUT_NAME}_amd64 ./${DIST_DIR}/macos/${OUTPUT_NAME}_arm64
	@echo "🍏 macOS build complete: ./${DIST_DIR}/macos/${OUTPUT_NAME}"

# Build for Linux (amd64)
build-linux:
	@echo "📦 Building for Linux (amd64)..."
	@mkdir -p ./${DIST_DIR}/linux
	@GOOS=linux GOARCH=amd64 $(GOBUILD) -ldflags="-X 'main.Version=${VERSION}' -X 'main.Commit=${GIT_COMMIT}' -X 'main.Date=${BUILD_DATE}'" -o ./${DIST_DIR}/linux/${OUTPUT_NAME} ./${SOURCE_FILE}
	@echo "🐧 Linux build complete: ./${DIST_DIR}/linux/${OUTPUT_NAME}"

# Build for Windows (amd64)
build-windows:
	@echo "📦 Building for Windows (amd64)..."
	@mkdir -p ./${DIST_DIR}/windows
	@GOOS=windows GOARCH=amd64 $(GOBUILD) -ldflags="-X 'main.Version=${VERSION}' -X 'main.Commit=${GIT_COMMIT}' -X 'main.Date=${BUILD_DATE}'" -o ./${DIST_DIR}/windows/${OUTPUT_NAME}.exe ./${SOURCE_FILE}
	@echo "🪟  Windows build complete: ./${DIST_DIR}/windows/${OUTPUT_NAME}.exe"

# --- Quality & Verification ---

# Run tests
test:
	@echo "🧪 Running tests..."
	@$(GOTEST) -v ./...

# Run linter
lint:
	@echo "🔍 Running linter..."
	@$(GOLINT) ./...

# Run vulnerability check
vulncheck:
	@echo "🛡️  Checking for vulnerabilities..."
	@$(GOVULNCHECK) ./...

# --- Housekeeping ---

# Clean up build artifacts
clean:
	@echo "🧹 Cleaning up..."
	@rm -rf ./${DIST_DIR}
	@$(GOCLEAN)

# --- Help ---

# Display help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all          Build all binaries (default)."
	@echo "  build        Alias for 'all'."
	@echo "  build-macos  Build universal binary for macOS."
	@echo "  build-linux  Build binary for Linux (amd64)."
	@echo "  build-windows Build binary for Windows (amd64)."
	@echo "  package      Package binaries for release."
	@echo "  test         Run tests."
	@echo "  lint         Run the linter."
	@echo "  vulncheck    Run vulnerability scanner."
	@echo "  clean        Remove build artifacts."
	@echo "  help         Show this help message."

