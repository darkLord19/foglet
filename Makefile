.PHONY: build test install clean run lint wtx fog fogd all

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

# Build all binaries
all: wtx fog fogd

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

# Build all (default target)
build: all

test:
	@echo "Running tests..."
	@go test -v ./...

install: all
	@echo "Installing wtx, fog, and fogd..."
	@go install $(LDFLAGS) ./cmd/wtx
	@go install $(LDFLAGS) ./cmd/fog
	@go install $(LDFLAGS) ./cmd/fogd

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
