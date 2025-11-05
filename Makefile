# Include .env file if it exists
-include .env
export

# declare targets that are not files
.PHONY: all build build.linux build.pi build.darwin build.windows build.all run debug trace test test.coverage test.coverage.html test.coverage.stats clean install uninstall version version.patch version.minor version.major version.dev version.alpha version.rc lint lint-fix pre-commit setup-dev

# Version management
VERSION := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
NEW_VERSION = $(subst v,,$(VERSION))
BUILD_VERSION := $(shell git describe --tags --always --dirty)

# Binary paths
BINARY_NAME := aircast-cli
MAIN_GO := cmd/cli/main.go

# Default log level
LOG_LEVEL ?= info

all: build

build:
	@echo "Building $(BINARY_NAME) for current platform..."
	@go build -ldflags "-X main.version=$(BUILD_VERSION) -X main.commit=$(shell git rev-parse --short HEAD) -X main.date=$(shell date +%Y-%m-%d)" -o $(BINARY_NAME) $(MAIN_GO)
	@echo "Binary built: $(BINARY_NAME)"

# Build for Linux AMD64
build.linux:
	@echo "Building $(BINARY_NAME) for Linux (AMD64)..."
	@GOOS=linux GOARCH=amd64 go build -ldflags "-X main.version=$(BUILD_VERSION) -X main.commit=$(shell git rev-parse --short HEAD) -X main.date=$(shell date +%Y-%m-%d)" -o $(BINARY_NAME)-linux-amd64 $(MAIN_GO)
	@echo "Linux AMD64 binary built: $(BINARY_NAME)-linux-amd64"

# Build for Linux ARM64 (Raspberry Pi)
build.pi:
	@echo "Building $(BINARY_NAME) for Raspberry Pi (ARM64)..."
	@GOOS=linux GOARCH=arm64 go build -ldflags "-X main.version=$(BUILD_VERSION) -X main.commit=$(shell git rev-parse --short HEAD) -X main.date=$(shell date +%Y-%m-%d)" -o $(BINARY_NAME)-linux-arm64 $(MAIN_GO)
	@echo "ARM64 binary built: $(BINARY_NAME)-linux-arm64"

# Build for macOS
build.darwin:
	@echo "Building $(BINARY_NAME) for macOS..."
	@GOOS=darwin GOARCH=amd64 go build -ldflags "-X main.version=$(BUILD_VERSION) -X main.commit=$(shell git rev-parse --short HEAD) -X main.date=$(shell date +%Y-%m-%d)" -o $(BINARY_NAME)-darwin-amd64 $(MAIN_GO)
	@echo "macOS AMD64 binary built: $(BINARY_NAME)-darwin-amd64"
	@GOOS=darwin GOARCH=arm64 go build -ldflags "-X main.version=$(BUILD_VERSION) -X main.commit=$(shell git rev-parse --short HEAD) -X main.date=$(shell date +%Y-%m-%d)" -o $(BINARY_NAME)-darwin-arm64 $(MAIN_GO)
	@echo "macOS ARM64 binary built: $(BINARY_NAME)-darwin-arm64"

# Build for Windows
build.windows:
	@echo "Building $(BINARY_NAME) for Windows..."
	@GOOS=windows GOARCH=amd64 go build -ldflags "-X main.version=$(BUILD_VERSION) -X main.commit=$(shell git rev-parse --short HEAD) -X main.date=$(shell date +%Y-%m-%d)" -o $(BINARY_NAME)-windows-amd64.exe $(MAIN_GO)
	@echo "Windows AMD64 binary built: $(BINARY_NAME)-windows-amd64.exe"

# Build for all platforms
build.all: build.linux build.pi build.darwin build.windows
	@echo "All platform binaries built successfully!"

# Run the application
run:
	@LOG_LEVEL=$(LOG_LEVEL) go run -ldflags "-X main.version=$(BUILD_VERSION)" $(MAIN_GO)

# Run with debug level logging
debug:
	@LOG_LEVEL=debug go run -ldflags "-X main.version=$(BUILD_VERSION)" $(MAIN_GO)

# Run with trace level logging (most verbose)
trace:
	@LOG_LEVEL=trace go run -ldflags "-X main.version=$(BUILD_VERSION)" $(MAIN_GO)

# Display current version
version:
	@echo $(VERSION)
	@go run -ldflags "-X main.version=$(BUILD_VERSION)" $(MAIN_GO) --version

version.patch:
	@echo "Current version: $(VERSION)"
	@BASE_VERSION=$$(echo "$(NEW_VERSION)" | cut -d'-' -f1); \
	MAJOR=$$(echo "$$BASE_VERSION" | awk -F. '{print $$1}'); \
	MINOR=$$(echo "$$BASE_VERSION" | awk -F. '{print $$2}'); \
	PATCH=$$(echo "$$BASE_VERSION" | awk -F. '{print $$3}'); \
	while true; do \
		NEW_VERSION="v$$MAJOR.$$MINOR.$$PATCH"; \
		if ! git rev-parse "$$NEW_VERSION" >/dev/null 2>&1; then \
			break; \
		fi; \
		PATCH=$$((PATCH + 1)); \
	done; \
	echo "New version will be: $$NEW_VERSION"; \
	read -p "Are you sure you want to create release $$NEW_VERSION? [y/N] " confirm && [ "$$confirm" = "y" ] && \
	git tag -a "$$NEW_VERSION" -m "Release $$NEW_VERSION" && \
	git push --follow-tags

version.minor:
	@echo "Current version: $(VERSION)"
	@BASE_VERSION=$$(echo "$(NEW_VERSION)" | cut -d'-' -f1); \
	MAJOR=$$(echo "$$BASE_VERSION" | awk -F. '{print $$1}'); \
	MINOR=$$(echo "$$BASE_VERSION" | awk -F. '{print $$2}'); \
	while true; do \
		NEW_VERSION="v$$MAJOR.$$MINOR.0"; \
		if ! git rev-parse "$$NEW_VERSION" >/dev/null 2>&1; then \
			break; \
		fi; \
		MINOR=$$((MINOR + 1)); \
	done; \
	echo "New version will be: $$NEW_VERSION"; \
	read -p "Are you sure you want to create release $$NEW_VERSION? [y/N] " confirm && [ "$$confirm" = "y" ] && \
	git tag -a "$$NEW_VERSION" -m "Release $$NEW_VERSION" && \
	git push --follow-tags

version.major:
	@echo "Current version: $(VERSION)"
	@BASE_VERSION=$$(echo "$(NEW_VERSION)" | cut -d'-' -f1); \
	MAJOR=$$(echo "$$BASE_VERSION" | awk -F. '{print $$1}'); \
	while true; do \
		NEW_VERSION="v$$MAJOR.0.0"; \
		if ! git rev-parse "$$NEW_VERSION" >/dev/null 2>&1; then \
			break; \
		fi; \
		MAJOR=$$((MAJOR + 1)); \
	done; \
	echo "New version will be: $$NEW_VERSION"; \
	read -p "Are you sure you want to create release $$NEW_VERSION? [y/N] " confirm && [ "$$confirm" = "y" ] && \
	git tag -a "$$NEW_VERSION" -m "Release $$NEW_VERSION" && \
	git push --follow-tags

# Development version target
version.dev:
	@echo "Current version: $(VERSION)"
	@if echo "$(VERSION)" | grep -q "dev"; then \
		BASE_VERSION=$$(echo "$(VERSION)" | sed 's/-dev\.[0-9]*$$//'); \
		DEV_NUM=$$(echo "$(VERSION)" | grep -o 'dev\.[0-9]*' | cut -d. -f2); \
		NEXT_DEV=$$((DEV_NUM + 1)); \
	else \
		BASE_VERSION_RAW=$$(echo "$(VERSION)" | sed 's/-alpha\.[0-9]*$$//' | sed 's/-beta\.[0-9]*$$//' | sed 's/-rc\.[0-9]*$$//' | sed 's/^v//'); \
		MAJOR=$$(echo "$$BASE_VERSION_RAW" | awk -F. '{print $$1}'); \
		MINOR=$$(echo "$$BASE_VERSION_RAW" | awk -F. '{print $$2}'); \
		PATCH=$$(echo "$$BASE_VERSION_RAW" | awk -F. '{print $$3}'); \
		PATCH=$$((PATCH + 1)); \
		BASE_VERSION="v$$MAJOR.$$MINOR.$$PATCH"; \
		NEXT_DEV=1; \
	fi; \
	while true; do \
		NEW_VERSION="$${BASE_VERSION}-dev.$$NEXT_DEV"; \
		if ! git rev-parse "$$NEW_VERSION" >/dev/null 2>&1; then \
			break; \
		fi; \
		NEXT_DEV=$$((NEXT_DEV + 1)); \
	done; \
	echo "New development version will be: $$NEW_VERSION"; \
	read -p "Are you sure you want to create development release $$NEW_VERSION? [y/N] " confirm && [ "$$confirm" = "y" ] && \
	git tag -a "$$NEW_VERSION" -m "Development Release $$NEW_VERSION" && \
	git push --follow-tags

version.alpha:
	@echo "Current version: $(VERSION)"
	@if echo "$(VERSION)" | grep -q "alpha"; then \
		BASE_VERSION=$$(echo "$(VERSION)" | sed 's/-alpha\.[0-9]*$$//'); \
		ALPHA_NUM=$$(echo "$(VERSION)" | grep -o 'alpha\.[0-9]*' | cut -d. -f2); \
		NEXT_ALPHA=$$((ALPHA_NUM + 1)); \
	else \
		BASE_VERSION="$(VERSION)"; \
		NEXT_ALPHA=1; \
	fi; \
	while true; do \
		NEW_VERSION="$${BASE_VERSION}-alpha.$$NEXT_ALPHA"; \
		if ! git rev-parse "$$NEW_VERSION" >/dev/null 2>&1; then \
			break; \
		fi; \
		NEXT_ALPHA=$$((NEXT_ALPHA + 1)); \
	done; \
	echo "New version will be: $$NEW_VERSION"; \
	read -p "Are you sure you want to create release $$NEW_VERSION? [y/N] " confirm && [ "$$confirm" = "y" ] && \
	git tag -a "$$NEW_VERSION" -m "Release $$NEW_VERSION" && \
	git push --follow-tags

version.rc:
	@echo "Current version: $(VERSION)"
	@BASE_VERSION=$$(echo "$(NEW_VERSION)" | cut -d'-' -f1); \
	if echo "$(VERSION)" | grep -q "rc"; then \
		RC_NUM=$$(echo "$(VERSION)" | grep -o 'rc\.[0-9]*' | cut -d. -f2); \
		NEXT_RC=$$((RC_NUM + 1)); \
	else \
		MAJOR=$$(echo "$$BASE_VERSION" | awk -F. '{print $$1}'); \
		MINOR=$$(echo "$$BASE_VERSION" | awk -F. '{print $$2}'); \
		PATCH=$$(echo "$$BASE_VERSION" | awk -F. '{print $$3}'); \
		PATCH=$$((PATCH + 1)); \
		BASE_VERSION="$$MAJOR.$$MINOR.$$PATCH"; \
		NEXT_RC=1; \
	fi; \
	while true; do \
		NEW_VERSION="v$$BASE_VERSION-rc.$$NEXT_RC"; \
		if ! git rev-parse "$$NEW_VERSION" >/dev/null 2>&1; then \
			break; \
		fi; \
		NEXT_RC=$$((NEXT_RC + 1)); \
	done; \
	echo "New version will be: $$NEW_VERSION"; \
	read -p "Are you sure you want to create release $$NEW_VERSION? [y/N] " confirm && [ "$$confirm" = "y" ] && \
	git tag -a "$$NEW_VERSION" -m "Release $$NEW_VERSION" && \
	git push --follow-tags

# Lint the application
lint:
	@echo "🔍 Running linter..."
	@golangci-lint run

lint-fix:
	@echo "🔧 Running linter with auto-fix..."
	@golangci-lint run --fix

# Test the application
test:
	@echo "Testing..."
	@go test ./... -v -race

# Run tests with coverage and output to coverage.out
test.coverage:
	@go test ./... -coverprofile=coverage.out

# Generate HTML coverage report and open it in a browser
test.coverage.html: test.coverage
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"
	@if command -v open > /dev/null; then \
		open coverage.html; \
	elif command -v xdg-open > /dev/null; then \
		xdg-open coverage.html; \
	else \
		echo "Please open coverage.html in your browser"; \
	fi

# Show coverage statistics in the terminal
test.coverage.stats: test.coverage
	@go tool cover -func=coverage.out

# Pre-commit checks (run build and tests)
pre-commit:
	@echo "🔍 Running pre-commit checks..."
	@echo "🔧 Running linter..."
	@golangci-lint run --fix
	@echo "🎨 Formatting code..."
	@go fmt ./...
	@echo "🔧 Tidying go modules..."
	@go mod tidy
	@echo "🏗️  Building CLI..."
	@$(MAKE) build
	@echo "🧪 Running tests..."
	@go test ./... -race
	@echo "✅ All pre-commit checks passed!"

# Development setup (installs lefthook and dependencies)
setup-dev:
	@echo "🔧 Setting up development environment..."
	@echo "📦 Installing lefthook..."
	@if ! command -v lefthook >/dev/null 2>&1; then \
		echo "Installing lefthook via go install..."; \
		go install github.com/evilmartians/lefthook@latest; \
	else \
		echo "✅ lefthook already installed"; \
	fi
	@echo "📦 Checking golangci-lint..."
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "⚠️  golangci-lint not found. Please install it:"; \
		echo "   brew install golangci-lint"; \
		echo "   or visit: https://golangci-lint.run/usage/install/"; \
		exit 1; \
	else \
		echo "✅ golangci-lint already installed"; \
	fi
	@echo "🪝 Installing git hooks..."
	@lefthook install
	@echo ""
	@echo "✅ Development environment ready!"
	@echo "💡 Lefthook will now run linting, tests and build checks before every commit."
	@echo "🔍 Test hooks manually: lefthook run pre-commit"
	@echo "🔧 Run linter manually: make lint or make lint-fix"

# Clean the binary and generated files
clean:
	@echo "Cleaning..."
	@rm -f $(BINARY_NAME)
	@rm -f $(BINARY_NAME)-*
	@rm -f coverage.out coverage.html
	@echo "Clean complete"

# Install the CLI to /usr/local/bin
install: build
	@echo "Installing $(BINARY_NAME) to /usr/local/bin..."
	@sudo cp $(BINARY_NAME) /usr/local/bin/
	@echo "✅ $(BINARY_NAME) installed successfully!"
	@echo "Run with: $(BINARY_NAME)"

# Uninstall the CLI from /usr/local/bin
uninstall:
	@echo "Uninstalling $(BINARY_NAME) from /usr/local/bin..."
	@sudo rm -f /usr/local/bin/$(BINARY_NAME)
	@echo "✅ $(BINARY_NAME) uninstalled successfully!"
