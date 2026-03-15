INSTALL_DIR ?= $(HOME)/.local/bin
BINARY      := tko
DIST        := dist
VERSION     ?= $(shell git describe --tags --exact-match 2>/dev/null || echo dev)
LDFLAGS     := -ldflags "-X tko/internal/version.Version=$(VERSION)"

.PHONY: build test install uninstall clean

build:
	@mkdir -p $(DIST)
	go build $(LDFLAGS) -o $(DIST)/$(BINARY) ./cmd/tko/

test:
	go test ./...

install: build
	@mkdir -p $(INSTALL_DIR)
	install -m755 $(DIST)/$(BINARY) $(INSTALL_DIR)/$(BINARY)
	@echo "Installed: $(INSTALL_DIR)/$(BINARY)"
	@echo ""
	@echo "Run 'tko hook install' to set up the Claude Code hook."

uninstall:
	@tko hook uninstall 2>/dev/null && echo "Hook removed." || true
	@rm -f $(INSTALL_DIR)/$(BINARY)
	@echo "Uninstalled $(INSTALL_DIR)/$(BINARY)"

clean:
	rm -rf $(DIST)
