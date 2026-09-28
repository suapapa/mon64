.PHONY: help build run test vet fmt clean \
	install-service install-user-service uninstall-service \
	status-service restart-service logs-service log-service

UNAME_S := $(shell uname -s)

PREFIX ?= $(HOME)/.local
BINDIR ?= $(PREFIX)/bin
XDG_CONFIG_HOME ?= $(HOME)/.config
CONFIG_DIR ?= $(XDG_CONFIG_HOME)/mon64
SYSTEMD_USER_DIR ?= $(XDG_CONFIG_HOME)/systemd/user
SERVICE_NAME := mon64.service

# macOS launchd
LAUNCHD_USER_DIR ?= $(HOME)/Library/LaunchAgents
LAUNCHD_LABEL := com.suapapa.mon64
LAUNCHD_PLIST := $(LAUNCHD_LABEL).plist
LOG_DIR ?= $(HOME)/Library/Logs

BINARY := bin/mon64
CONFIG_SRC ?= $(firstword $(wildcard configs/homin.yaml) configs/example.yaml)

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build                  Build mon64 to $(BINARY)"
	@echo "  run                    Run locally with configs/example.yaml"
	@echo "  test                   Run go test ./..."
	@echo "  vet                    Run go vet ./..."
	@echo "  fmt                    Run gofmt -w ."
	@echo "  clean                  Remove build artifacts"
	@echo "  install-service        Install & start user service (LaunchAgent / systemd --user)"
	@echo "  uninstall-service      Stop & remove user service"
	@echo "  status-service         Show user service status"
	@echo "  restart-service        Restart user service"
	@echo "  logs-service           Follow user service logs"
	@echo ""
	@echo "Variables:"
	@echo "  CONFIG_SRC=$(CONFIG_SRC)   # seed config on first install"
	@echo "  BINDIR=$(BINDIR)"
	@echo "  CONFIG_DIR=$(CONFIG_DIR)"

build:
	@echo "==> Building mon64..."
	@mkdir -p bin
	go build -o $(BINARY) ./cmd/mon64

run: build
	./$(BINARY) -config configs/example.yaml

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

clean:
	rm -rf bin

install-service: build
	@echo "==> Installing mon64 binary to $(BINDIR)..."
	@mkdir -p "$(BINDIR)"
	@rm -f "$(BINDIR)/mon64"
	@cp -f $(BINARY) "$(BINDIR)/mon64"
	@chmod 0755 "$(BINDIR)/mon64"
	@echo "==> Setting up configuration in $(CONFIG_DIR)..."
	@mkdir -p "$(CONFIG_DIR)"
	@if [ ! -f "$(CONFIG_DIR)/config.yaml" ]; then \
		if [ ! -f "$(CONFIG_SRC)" ]; then \
			echo "Error: seed config not found: $(CONFIG_SRC)" >&2; exit 1; \
		fi; \
		cp "$(CONFIG_SRC)" "$(CONFIG_DIR)/config.yaml"; \
		chmod 0600 "$(CONFIG_DIR)/config.yaml"; \
		echo "    Copied $(CONFIG_SRC) -> $(CONFIG_DIR)/config.yaml"; \
	else \
		echo "    Preserved existing $(CONFIG_DIR)/config.yaml"; \
	fi
ifeq ($(UNAME_S),Darwin)
	@echo "==> Installing LaunchAgent to $(LAUNCHD_USER_DIR)..."
	@mkdir -p "$(LAUNCHD_USER_DIR)"
	@mkdir -p "$(LOG_DIR)"
	@sed -e 's|__BINDIR__|$(BINDIR)|g' \
	     -e 's|__CONFIG_DIR__|$(CONFIG_DIR)|g' \
	     -e 's|__LOG_DIR__|$(LOG_DIR)|g' \
	     -e 's|__HOME__|$(HOME)|g' \
	     launchd/$(LAUNCHD_PLIST) > "$(LAUNCHD_USER_DIR)/$(LAUNCHD_PLIST)"
	@echo "==> Loading $(LAUNCHD_LABEL) with launchctl..."
	@launchctl unload "$(LAUNCHD_USER_DIR)/$(LAUNCHD_PLIST)" 2>/dev/null || true
	@launchctl load -w "$(LAUNCHD_USER_DIR)/$(LAUNCHD_PLIST)"
	@echo ""
	@echo "mon64 LaunchAgent installed and started."
	@echo "  Status : make status-service"
	@echo "  Logs   : make logs-service"
	@echo "  Config : $(CONFIG_DIR)/config.yaml"
else ifeq ($(UNAME_S),Linux)
	@command -v systemctl >/dev/null 2>&1 || { \
		echo "Error: systemctl not found. This target requires Linux systemd." >&2; exit 1; \
	}
	@echo "==> Installing systemd user unit to $(SYSTEMD_USER_DIR)..."
	@mkdir -p "$(SYSTEMD_USER_DIR)"
	@sed -e 's|%h/.local/bin/mon64|$(subst $(HOME),%h,$(BINDIR))/mon64|g' \
	     -e 's|%h/.config/mon64|$(subst $(HOME),%h,$(CONFIG_DIR))|g' \
	     systemd/$(SERVICE_NAME) > "$(SYSTEMD_USER_DIR)/$(SERVICE_NAME)"
	@echo "==> Reloading systemd daemon..."
	@systemctl --user daemon-reload
	@echo "==> Enabling and starting $(SERVICE_NAME)..."
	@systemctl --user enable $(SERVICE_NAME)
	@systemctl --user restart $(SERVICE_NAME)
	@echo ""
	@echo "mon64 user service installed and started."
	@echo "  Status : make status-service"
	@echo "  Logs   : make logs-service"
	@echo "  Config : $(CONFIG_DIR)/config.yaml"
	@echo ""
	@echo "Tip: keep user services after logout with: loginctl enable-linger"
else
	@echo "Error: unsupported OS: $(UNAME_S)" >&2; exit 1;
endif

install-user-service: install-service

uninstall-service:
ifeq ($(UNAME_S),Darwin)
	@echo "==> Stopping and unloading $(LAUNCHD_LABEL)..."
	@launchctl unload -w "$(LAUNCHD_USER_DIR)/$(LAUNCHD_PLIST)" 2>/dev/null || true
	@echo "==> Removing LaunchAgent plist..."
	@rm -f "$(LAUNCHD_USER_DIR)/$(LAUNCHD_PLIST)"
	@echo "==> Removing binary $(BINDIR)/mon64..."
	@rm -f "$(BINDIR)/mon64"
	@echo "==> Service uninstalled."
	@echo "Note: configuration at $(CONFIG_DIR) was preserved."
else ifeq ($(UNAME_S),Linux)
	@command -v systemctl >/dev/null 2>&1 || { \
		echo "Error: systemctl not found. This target requires Linux systemd." >&2; exit 1; \
	}
	@echo "==> Stopping and disabling $(SERVICE_NAME)..."
	@systemctl --user disable --now $(SERVICE_NAME) 2>/dev/null || true
	@echo "==> Removing systemd unit..."
	@rm -f "$(SYSTEMD_USER_DIR)/$(SERVICE_NAME)"
	@systemctl --user daemon-reload
	@echo "==> Removing binary $(BINDIR)/mon64..."
	@rm -f "$(BINDIR)/mon64"
	@echo "==> Service uninstalled."
	@echo "Note: configuration at $(CONFIG_DIR) was preserved."
else
	@echo "Error: unsupported OS: $(UNAME_S)" >&2; exit 1;
endif

status-service:
ifeq ($(UNAME_S),Darwin)
	@echo "==> Checking LaunchAgent status for $(LAUNCHD_LABEL)..."
	@if launchctl list $(LAUNCHD_LABEL) >/dev/null 2>&1; then \
		launchctl list $(LAUNCHD_LABEL); \
		echo ""; \
		PID=$$(launchctl list $(LAUNCHD_LABEL) | awk '/"PID"/ {gsub(/[^0-9]/, ""); print}'); \
		if [ -n "$$PID" ]; then \
			echo "Status: Running (PID $$PID)"; \
		else \
			echo "Status: Loaded (not currently running)"; \
		fi; \
	else \
		echo "Service $(LAUNCHD_LABEL) is not loaded."; \
		echo "Run 'make install-service' to install and start it."; \
	fi
else ifeq ($(UNAME_S),Linux)
	@systemctl --user status $(SERVICE_NAME)
else
	@echo "Error: unsupported OS: $(UNAME_S)" >&2; exit 1;
endif

restart-service:
ifeq ($(UNAME_S),Darwin)
	@echo "==> Restarting $(LAUNCHD_LABEL)..."
	@launchctl kickstart -k gui/$$(id -u)/$(LAUNCHD_LABEL) 2>/dev/null || { \
		launchctl unload "$(LAUNCHD_USER_DIR)/$(LAUNCHD_PLIST)" 2>/dev/null || true; \
		launchctl load -w "$(LAUNCHD_USER_DIR)/$(LAUNCHD_PLIST)"; \
	}
	@echo "Service restarted."
else ifeq ($(UNAME_S),Linux)
	@systemctl --user restart $(SERVICE_NAME)
else
	@echo "Error: unsupported OS: $(UNAME_S)" >&2; exit 1;
endif

logs-service:
ifeq ($(UNAME_S),Darwin)
	@mkdir -p "$(LOG_DIR)"
	@touch "$(LOG_DIR)/mon64.log"
	@echo "==> Following $(LOG_DIR)/mon64.log (Ctrl+C to exit)..."
	@tail -f "$(LOG_DIR)/mon64.log"
else ifeq ($(UNAME_S),Linux)
	@journalctl --user -u $(SERVICE_NAME) -f
else
	@echo "Error: unsupported OS: $(UNAME_S)" >&2; exit 1;
endif

log-service: logs-service
