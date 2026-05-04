BINARY  ?= agent-callable
PKG     ?= ./...
DISTDIR ?= dist
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

# Target platforms (Linux + macOS, no Windows)
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64

# Plugin Claude Code
PLUGIN_SRC   ?= plugins/agent-callable
PLUGIN_KEY   ?= agent-callable@agent-callable
PLUGIN_NAME  ?= $(shell jq -r .name $(PLUGIN_SRC)/.claude-plugin/plugin.json 2>/dev/null)
PLUGIN_VER   ?= $(shell jq -r .version $(PLUGIN_SRC)/.claude-plugin/plugin.json 2>/dev/null)
PLUGIN_DIR   ?= $(CURDIR)/$(PLUGIN_SRC)

.PHONY: build install test fmt tidy clean build-all clean-all package info dev repair tag

LDFLAGS ?= -ldflags="-s -w -X main.version=$(VERSION)"

# Local build (current platform)
build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/agent-callable

install:
	go install $(LDFLAGS) ./cmd/agent-callable

test:
	go test $(PKG)

fmt:
	go fmt $(PKG)

tidy:
	go mod tidy

clean:
	rm -f $(BINARY)

# Cross-compilation (all platforms)
build-all:
	@mkdir -p $(DISTDIR)
	@for platform in $(PLATFORMS); do \
		GOOS=$${platform%/*} GOARCH=$${platform#*/} \
		go build -ldflags="-s -w -X main.version=$(VERSION)" -o $(DISTDIR)/$(BINARY)-$${platform%/*}-$${platform#*/} ./cmd/agent-callable; \
		echo "Built: $(DISTDIR)/$(BINARY)-$${platform%/*}-$${platform#*/}"; \
	done

clean-all: clean
	rm -rf $(DISTDIR)

# Package for distribution (create archives)
package: build-all
	@for platform in $(PLATFORMS); do \
		bin=$(BINARY)-$${platform%/*}-$${platform#*/}; \
		tar -czf $(DISTDIR)/$$bin.tar.gz -C $(DISTDIR) $$bin; \
		echo "Packaged: $(DISTDIR)/$$bin.tar.gz"; \
	done
	@cd $(DISTDIR) && sha256sum *.tar.gz > checksums.txt
	@echo "Checksums: $(DISTDIR)/checksums.txt"

# Plugin dev: point cache to current directory (main or worktree)
dev:
	@cplugins dev $(PLUGIN_KEY) $(PLUGIN_DIR)

# Plugin repair: fix broken cache (typically after a worktree was cleaned up)
repair:
	@cplugins repair $(PLUGIN_KEY)

# Create annotated tag from plugin.json version
tag:
	@ver=$$(jq -r .version $(PLUGIN_SRC)/.claude-plugin/plugin.json); \
	if git rev-parse "v$$ver" >/dev/null 2>&1; then \
		echo "Tag v$$ver already exists"; exit 1; \
	fi; \
	sed -i "s|release-v[0-9.]*-blue|release-v$$ver-blue|" README.md; \
	jq --arg v "$$ver" '.plugins[0].version = $$v' .claude-plugin/marketplace.json > .claude-plugin/marketplace.json.tmp \
		&& mv .claude-plugin/marketplace.json.tmp .claude-plugin/marketplace.json; \
	if git diff --quiet HEAD -- $(PLUGIN_SRC)/.claude-plugin/plugin.json README.md .claude-plugin/marketplace.json; then :; else \
		git add $(PLUGIN_SRC)/.claude-plugin/plugin.json README.md .claude-plugin/marketplace.json && git commit -m "chore: bump version to v$$ver"; \
	fi; \
	git tag -a "v$$ver" -m "v$$ver"; \
	echo "Created tag v$$ver"

# Show build info
info:
	@echo "Binary:    $(BINARY)"
	@echo "Version:   $(VERSION)"
	@echo "Platforms: $(PLATFORMS)"
	@echo "Plugin:    $(PLUGIN_NAME) v$(PLUGIN_VER)"
	@cplugins status $(PLUGIN_KEY)
