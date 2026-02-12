.PHONY: build test install clean run lint wtx fog fogd fogcloud all release-artifacts release-formula

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
RELEASE_TAG ?= v0.0.0-dev
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

# Build all binaries
all: wtx fog fogd fogcloud

# Build wtx
wtx:
	@echo "Building wtx $(VERSION)..."
	@go build $(LDFLAGS) -o bin/wtx ./cmd/wtx

# Build fog
fog:
	@echo "Building fog $(VERSION)..."
	@go build $(LDFLAGS) -o bin/fog ./cmd/fog

# Build fogd
fogd:
	@echo "Building fogd $(VERSION)..."
	@go build $(LDFLAGS) -o bin/fogd ./cmd/fogd

# Build fogcloud
fogcloud:
	@echo "Building fogcloud $(VERSION)..."
	@go build $(LDFLAGS) -o bin/fogcloud ./cmd/fogcloud

# Build all (default target)
build: all

test:
	@echo "Running tests..."
	@go test -v ./...

install: all
	@echo "Installing wtx, fog, fogd, and fogcloud..."
	@go install $(LDFLAGS) ./cmd/wtx
	@go install $(LDFLAGS) ./cmd/fog
	@go install $(LDFLAGS) ./cmd/fogd
	@go install $(LDFLAGS) ./cmd/fogcloud

clean:
	@echo "Cleaning..."
	@rm -rf bin/ dist/

run:
	@go run ./cmd/wtx

lint:
	@echo "Running linters..."
	@go fmt ./...
	@go vet ./...

dev: build
	@./bin/wtx

release-artifacts:
	@echo "Building release artifacts for $(RELEASE_TAG)..."
	@chmod +x scripts/release/build-artifacts.sh
	@scripts/release/build-artifacts.sh "$(RELEASE_TAG)" dist

release-formula: release-artifacts
	@echo "Generating Homebrew formula for $(RELEASE_TAG)..."
	@chmod +x scripts/release/generate-homebrew-formula.sh
	@scripts/release/generate-homebrew-formula.sh \
		"$(RELEASE_TAG)" \
		"dist/wtx_$$(echo $(RELEASE_TAG) | sed 's/^v//')_checksums.txt" \
		"darkLord19/wtx" > dist/wtx.rb
	@echo "Formula generated at dist/wtx.rb"
