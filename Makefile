.DEFAULT_GOAL := help

BINARY := sd
PKG := ./cmd/sd
BIN_DIR := bin
VERSION ?= $(shell (git describe --tags --abbrev=0 2>/dev/null || echo v0.0.0) | sed 's/^v//')
PUBLISH_TAG ?=

.PHONY: help build install test publish

help:
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@echo "  build    Build $(BINARY) into $(BIN_DIR)/"
	@echo "  install  Install $(BINARY) to GOBIN using go install"
	@echo "  test     Run Go tests"
	@echo "  publish  Create/push release tag, publish GitHub release + Homebrew formula"

build:
	@mkdir -p $(BIN_DIR)
	@go build -ldflags "-X main.version=$(VERSION)" -o $(BIN_DIR)/$(BINARY) $(PKG)

install:
	@go install -ldflags "-X main.version=$(VERSION)" $(PKG)

test:
	@go test ./...

publish:
	@command -v goreleaser >/dev/null 2>&1 || (echo "goreleaser is required for publish"; exit 1)
	@TAG="$(PUBLISH_TAG)"; \
	if [ -z "$$TAG" ]; then \
		if git describe --tags --exact-match >/dev/null 2>&1; then \
			TAG=$$(git describe --tags --exact-match); \
			echo "Using existing tag on HEAD: $$TAG"; \
		else \
			LATEST=$$(git tag --list 'v*' --sort=-v:refname | head -n1); \
			if [ -z "$$LATEST" ]; then \
				TAG="v0.1.0"; \
			else \
				NUM=$${LATEST#v}; \
				MAJOR=$${NUM%%.*}; REST=$${NUM#*.}; MINOR=$${REST%%.*}; PATCH=$${REST#*.}; \
				TAG="v$$MAJOR.$$MINOR.$$((PATCH+1))"; \
			fi; \
			echo "Creating and pushing release tag $$TAG"; \
			git tag -a "$$TAG" -m "Release $$TAG"; \
			git push origin "$$TAG"; \
		fi; \
	else \
		echo "Using requested publish tag: $$TAG"; \
		if ! git rev-parse "$$TAG" >/dev/null 2>&1; then \
			git tag -a "$$TAG" -m "Release $$TAG"; \
			git push origin "$$TAG"; \
		fi; \
	fi
	@if [ -z "$$GITHUB_TOKEN" ] && [ -z "$$GH_TOKEN" ]; then \
		command -v gh >/dev/null 2>&1 || (echo "gh CLI is required when GITHUB_TOKEN/GH_TOKEN is not set"; exit 1); \
		GH_AUTH_TOKEN=$$(gh auth token 2>/dev/null) || (echo "GitHub auth not found. Run: gh auth login"; exit 1); \
		[ -n "$$GH_AUTH_TOKEN" ] || (echo "GitHub auth token is empty. Run: gh auth login"; exit 1); \
		GITHUB_TOKEN="$$GH_AUTH_TOKEN" goreleaser release --clean; \
	else \
		goreleaser release --clean; \
	fi
