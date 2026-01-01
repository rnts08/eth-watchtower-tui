# Makefile for eth-watchtower-tui

# Get the version from the VERSION file.
VERSION := $(shell cat VERSION)
# Get the current git commit hash. Use 'unknown' if not in a git repo.
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
# Build timestamp.
BUILD_DATE ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GORUN=$(GOCMD) run
BINARY_NAME=eth-watchtower-tui
# Set version variables at build time
LDFLAGS := -ldflags="-X main.version=$(VERSION) -X main.commit=$(GIT_COMMIT) -X main.date=$(BUILD_DATE)"

# Platforms for cross-compilation
PLATFORMS := linux/amd64 darwin/amd64 darwin/arm64 windows/amd64

# Default target
all: build

# Build the application binary
build:
	@echo "Building $(BINARY_NAME) version $(VERSION) (commit: $(GIT_COMMIT))..."
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) .

# Run the application
run:
	$(GORUN) .

# Run tests
test:
	$(GOTEST) -v ./...

# Run linter
lint:
	golangci-lint run

# Cross-compile binaries for distribution
dist:
	@echo "Building binaries for platforms: $(PLATFORMS)..."
	@mkdir -p dist
	@for platform in $(PLATFORMS); do \
		GOOS=$${platform%/*} ; \
		GOARCH=$${platform#*/} ; \
		OUTPUT_NAME=$(BINARY_NAME)-$$GOOS-$$GOARCH ; \
		if [ "$$GOOS" = "windows" ]; then OUTPUT_NAME=$${OUTPUT_NAME}.exe; fi ; \
		echo "Building $$OUTPUT_NAME..." ; \
		GOOS=$$GOOS GOARCH=$$GOARCH $(GOBUILD) $(LDFLAGS) -o dist/$$OUTPUT_NAME . ; \
	done

# Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -rf dist

# --- Versioning ---
# NOTE: The 'bump' variable must be passed for versioning targets.
# Example: make patch bump="Your change description"

patch:
	@$(eval NEW_VERSION=$(shell awk -F. -v OFS=. '{$$3=$$3+1; print}' VERSION))
	@echo $(NEW_VERSION) > VERSION
	@printf "## [$(NEW_VERSION)] - $(shell date +%Y-%m-%d)\n### Fixed\n- $(bump)\n\n$(shell cat CHANGELOG.md)" > CHANGELOG.md.tmp && mv CHANGELOG.md.tmp CHANGELOG.md
	@echo "Version bumped to $(NEW_VERSION)"

minor:
	@$(eval NEW_VERSION=$(shell awk -F. -v OFS=. '{$$2=$$2+1; $$3=0; print}' VERSION))
	@echo $(NEW_VERSION) > VERSION
	@printf "## [$(NEW_VERSION)] - $(shell date +%Y-%m-%d)\n### Added\n- $(bump)\n\n$(shell cat CHANGELOG.md)" > CHANGELOG.md.tmp && mv CHANGELOG.md.tmp CHANGELOG.md
	@echo "Version bumped to $(NEW_VERSION)"

major:
	@$(eval NEW_VERSION=$(shell awk -F. -v OFS=. '{$$1=$$1+1; $$2=0; $$3=0; print}' VERSION))
	@echo $(NEW_VERSION) > VERSION
	@printf "## [$(NEW_VERSION)] - $(shell date +%Y-%m-%d)\n### Changed\n- $(bump)\n\n$(shell cat CHANGELOG.md)" > CHANGELOG.md.tmp && mv CHANGELOG.md.tmp CHANGELOG.md
	@echo "Version bumped to $(NEW_VERSION)"

.PHONY: all build run test lint clean dist patch minor major