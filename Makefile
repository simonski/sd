.DEFAULT_GOAL := help

BINARY := sd
PKG := ./cmd/sd
BIN_DIR := bin
VERSION_FILE := VERSION
VERSION ?= $(shell (cat $(VERSION_FILE) 2>/dev/null || git describe --tags --abbrev=0 2>/dev/null || echo v0.0.0) | sed 's/^v//')
PUBLISH_TAG ?=

.PHONY: help build install unit-test test cover vet bench bench-check vuln release-check ci publish bump-version

help:
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@echo "  build    Increment patch version in $(VERSION_FILE) and build $(BINARY) into $(BIN_DIR)/"
	@echo "  install  Install $(BINARY) to GOBIN using go install"
	@echo "  unit-test Run Go unit tests only"
	@echo "  test     Run the same checks as GitHub CI"
	@echo "  cover    Run Go tests with coverage"
	@echo "  vet      Run go vet"
	@echo "  bench    Run benchmark suite"
	@echo "  bench-check  Run benchmark threshold checks"
	@echo "  vuln     Run govulncheck ./..."
	@echo "  release-check  Validate .goreleaser config"
	@echo "  ci       Run local CI-equivalent checks"
	@echo "  publish  Create/push release tag, publish GitHub release + Homebrew formula"

bump-version:
	@current=$$(cat $(VERSION_FILE) 2>/dev/null || echo 0.0.0); \
	major=$${current%%.*}; rest=$${current#*.}; minor=$${rest%%.*}; patch=$${rest#*.}; \
	case "$$major.$$minor.$$patch" in \
		''|*[^0-9.]*|*.*.*.*) echo "Invalid version in $(VERSION_FILE): $$current"; exit 1 ;; \
	esac; \
	next="$$major.$$minor.$$((patch+1))"; \
	printf '%s\n' "$$next" > $(VERSION_FILE); \
	echo "Version: $$current -> $$next"

build: bump-version
	@mkdir -p $(BIN_DIR)
	@VERSION=$$(cat $(VERSION_FILE)); go build -ldflags "-X main.version=$$VERSION" -o $(BIN_DIR)/$(BINARY) $(PKG)

install:
	@VERSION=$$(cat $(VERSION_FILE) 2>/dev/null || echo $(VERSION)); go install -ldflags "-X main.version=$$VERSION" $(PKG)

unit-test:
	@go test ./...

cover:
	@go test -cover ./...

vet:
	@go vet ./...

bench:
	@go test ./cmd/sd -run ^$$ -bench Benchmark -benchmem

bench-check:
	@./scripts/check-bench.sh

vuln:
	@command -v govulncheck >/dev/null 2>&1 || go install golang.org/x/vuln/cmd/govulncheck@latest
	@govulncheck ./...

release-check:
	@go run github.com/goreleaser/goreleaser/v2@latest check

test: unit-test cover vet bench-check vuln release-check
	@echo "test checks complete"

ci: test
	@echo "ci checks complete"

publish:
	@command -v goreleaser >/dev/null 2>&1 || (echo "goreleaser is required for publish"; exit 1)
	@VERSION=$$(cat $(VERSION_FILE) 2>/dev/null || echo $(VERSION)); \
	VERSION=$${VERSION#v}; \
	EXPECTED_TAG="v$$VERSION"; \
	TAG="$(PUBLISH_TAG)"; \
	if [ -z "$$TAG" ]; then \
		TAG="$$EXPECTED_TAG"; \
		echo "Using publish tag from $(VERSION_FILE): $$TAG"; \
	else \
		echo "Using requested publish tag: $$TAG"; \
		if [ "$$TAG" != "$$EXPECTED_TAG" ]; then \
			echo "PUBLISH_TAG ($$TAG) must match $(VERSION_FILE) ($$EXPECTED_TAG)"; \
			exit 1; \
		fi; \
	fi; \
	if git rev-parse "$$TAG" >/dev/null 2>&1; then \
		if ! git describe --tags --exact-match --match "$$TAG" >/dev/null 2>&1; then \
			echo "Tag $$TAG already exists but is not on HEAD. Bump $(VERSION_FILE) or move to the tagged commit."; \
			exit 1; \
		fi; \
		echo "Using existing tag on HEAD: $$TAG"; \
	else \
		echo "Creating and pushing release tag $$TAG"; \
		git tag -a "$$TAG" -m "Release $$TAG"; \
		git push origin "$$TAG"; \
	fi
	@if [ -z "$$GITHUB_TOKEN" ] && [ -z "$$GH_TOKEN" ]; then \
		command -v gh >/dev/null 2>&1 || (echo "gh CLI is required when GITHUB_TOKEN/GH_TOKEN is not set"; exit 1); \
		GH_AUTH_TOKEN=$$(gh auth token 2>/dev/null) || (echo "GitHub auth not found. Run: gh auth login"; exit 1); \
		[ -n "$$GH_AUTH_TOKEN" ] || (echo "GitHub auth token is empty. Run: gh auth login"; exit 1); \
		GITHUB_TOKEN="$$GH_AUTH_TOKEN" goreleaser release --clean; \
	else \
		goreleaser release --clean; \
	fi
