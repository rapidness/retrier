# Makefile for proxy-api

BINARY_NAME=retry-middleware
GO=go
MAIN=./cmd/proxy
LDFLAGS=-ldflags "-s -w"

.PHONY: all build clean test test-cover benchmark lint docker run

all: build

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	$(GO) build $(LDFLAGS) -o ./bin/$(BINARY_NAME) $(MAIN)
	@echo "Done: ./bin/$(BINARY_NAME)"

# Build for all platforms
build-all: build-linux build-darwin build-windows

build-linux:
	GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o ./bin/$(BINARY_NAME)-linux-amd64 $(MAIN)

build-darwin:
	GOOS=darwin GOARCH=amd64 $(GO) build $(LDFLAGS) -o ./bin/$(BINARY_NAME)-darwin-amd64 $(MAIN)
	GOOS=darwin GOARCH=arm64 $(GO) build $(LDFLAGS) -o ./bin/$(BINARY_NAME)-darwin-arm64 $(MAIN)

build-windows:
	GOOS=windows GOARCH=amd64 $(GO) build $(LDFLAGS) -o ./bin/$(BINARY_NAME)-windows-amd64.exe $(MAIN)

# Run tests
test:
	$(GO) test ./internal/... -v

# Run tests with coverage
test-cover:
	$(GO) test ./internal/... -cover -coverprofile=coverage.out
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Run benchmarks
benchmark:
	$(GO) test ./internal/... -bench=. -benchmem

# Lint
lint:
	golangci-lint run ./...

# Clean build artifacts
clean:
	rm -rf ./bin/
	rm -f coverage.out coverage.html

# Build Docker image
docker:
	docker build -t $(BINARY_NAME):latest .

# Run locally with default config
run: build
	./bin/$(BINARY_NAME) -config ./configs/config.yaml

# Run with logging enabled (for debugging)
run-debug: build
	@sed 's/enabled: false/enabled: true/' ./configs/config.yaml > /tmp/debug-config.yaml
	./bin/$(BINARY_NAME) -config /tmp/debug-config.yaml
