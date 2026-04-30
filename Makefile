.DEFAULT_GOAL := help

BINARY := sd
PKG := ./cmd/sd
BIN_DIR := bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: help build install test publish

help:
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@echo "  build    Build $(BINARY) into $(BIN_DIR)/"
	@echo "  install  Install $(BINARY) to GOBIN using go install"
	@echo "  test     Run Go tests"
	@echo "  publish  Create GitHub release artifacts + Homebrew formula via goreleaser"

build:
	@mkdir -p $(BIN_DIR)
	@go build -ldflags "-X main.version=$(VERSION)" -o $(BIN_DIR)/$(BINARY) $(PKG)

install:
	@go install -ldflags "-X main.version=$(VERSION)" $(PKG)

test:
	@go test ./...

publish:
	@command -v goreleaser >/dev/null 2>&1 || (echo "goreleaser is required for publish"; exit 1)
	@goreleaser release --clean
