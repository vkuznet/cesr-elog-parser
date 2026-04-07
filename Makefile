# Binary name (change if needed)

APP_NAME := cesr_elog_parser

# Go toolchain

GO := go

# Output binary

BINARY := $(APP_NAME)

# Default target

.PHONY: all
all: build

# Build static binary

.PHONY: build
build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 
	$(GO) build -ldflags="-s -w -extldflags '-static'" -o $(BINARY) .

# Run locally (non-static, for convenience)

.PHONY: run
run:
	$(GO) run .

# Run tests

.PHONY: test
test:
	$(GO) test ./...

# Clean artifacts

.PHONY: clean
clean:
	rm -f $(BINARY)

# Format code

.PHONY: fmt
fmt:
	gofmt -s -w .

# Tidy modules

.PHONY: tidy
tidy:
	$(GO) mod tidy

