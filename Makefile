BINARY  ?= agent-callable
PKG     ?= ./...
DISTDIR ?= dist
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

# Plateformes cibles (Linux + macOS, pas Windows)
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64

# Plugin Claude Code
PLUGIN_SRC   ?= plugins/agent-callable
PLUGIN_NAME  ?= $(shell jq -r .name $(PLUGIN_SRC)/.claude-plugin/plugin.json 2>/dev/null)
PLUGIN_VER   ?= $(shell jq -r .version $(PLUGIN_SRC)/.claude-plugin/plugin.json 2>/dev/null)
PLUGIN_CACHE ?= $(HOME)/.claude/plugins/cache/agent-callable/$(PLUGIN_NAME)/$(PLUGIN_VER)

.PHONY: build install test fmt tidy clean build-all clean-all package info plugin-sync tag

LDFLAGS ?= -ldflags="-s -w -X main.version=$(VERSION)"

# Build local (plateforme courante)
build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/agent-callable

install: plugin-sync
	go install $(LDFLAGS) ./cmd/agent-callable

test:
	go test $(PKG)

fmt:
	go fmt $(PKG)

tidy:
	go mod tidy

clean:
	rm -f $(BINARY)

# Cross-compilation multi-plateforme
build-all:
	@mkdir -p $(DISTDIR)
	@for platform in $(PLATFORMS); do \
		GOOS=$${platform%/*} GOARCH=$${platform#*/} \
		go build -ldflags="-s -w -X main.version=$(VERSION)" -o $(DISTDIR)/$(BINARY)-$${platform%/*}-$${platform#*/} ./cmd/agent-callable; \
		echo "Built: $(DISTDIR)/$(BINARY)-$${platform%/*}-$${platform#*/}"; \
	done

clean-all: clean
	rm -rf $(DISTDIR)

# Package pour distribution (crée des archives)
package: build-all
	@for platform in $(PLATFORMS); do \
		bin=$(BINARY)-$${platform%/*}-$${platform#*/}; \
		tar -czf $(DISTDIR)/$$bin.tar.gz -C $(DISTDIR) $$bin; \
		echo "Packaged: $(DISTDIR)/$$bin.tar.gz"; \
	done
	@cd $(DISTDIR) && sha256sum *.tar.gz > checksums.txt
	@echo "Checksums: $(DISTDIR)/checksums.txt"

# Sync le plugin Claude Code vers le cache local (si installé)
plugin-sync:
	@mkdir -p $(PLUGIN_CACHE)
	@rsync -a --delete --exclude='.git' $(PLUGIN_SRC)/ $(PLUGIN_CACHE)/
	@echo "Synced $(PLUGIN_SRC)/ → $(PLUGIN_CACHE)/"

# Create annotated tag from plugin.json version
tag:
	@ver=$$(jq -r .version $(PLUGIN_SRC)/.claude-plugin/plugin.json); \
	if git rev-parse "v$$ver" >/dev/null 2>&1; then \
		echo "Tag v$$ver already exists"; exit 1; \
	fi; \
	sed -i "s|release-v[0-9.]*-blue|release-v$$ver-blue|" README.md; \
	jq --arg v "$$ver" '.plugins[0].version = $$v' .claude-plugin/marketplace.json > .claude-plugin/marketplace.json.tmp \
		&& mv .claude-plugin/marketplace.json.tmp .claude-plugin/marketplace.json; \
	if git diff --quiet README.md .claude-plugin/marketplace.json; then :; else \
		git add README.md .claude-plugin/marketplace.json && git commit -m "chore: bump version to v$$ver"; \
	fi; \
	git tag -a "v$$ver" -m "v$$ver"; \
	echo "Created tag v$$ver"

# Affiche les infos de build
info:
	@echo "Binary:    $(BINARY)"
	@echo "Version:   $(VERSION)"
	@echo "Platforms: $(PLATFORMS)"
	@echo "Plugin:    $(PLUGIN_NAME) v$(PLUGIN_VER)"
	@if [ -d "$(PLUGIN_CACHE)" ]; then echo "Cache:     $(PLUGIN_CACHE) (installed)"; else echo "Cache:     not installed"; fi
