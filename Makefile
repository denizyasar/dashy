BINARY_DIR := build
DASHY_BIN := $(BINARY_DIR)/dashy
AGENT_BIN := $(BINARY_DIR)/dashy_agent
PREVIEW_DIR := testdata
PREVIEW_CONFIG := $(PREVIEW_DIR)/config.json
PREVIEW_LOG := $(PREVIEW_DIR)/preview.log
PREVIEW_PID := $(PREVIEW_DIR)/preview.pid

.PHONY: all build test clean preview stop-preview

all: build

build:
	mkdir -p $(BINARY_DIR)
	go build -ldflags="-s -w" -trimpath -o $(DASHY_BIN) .
	go build -ldflags="-s -w" -trimpath -o $(AGENT_BIN) ./agent

test:
	go test ./...

clean:
	rm -rf $(BINARY_DIR)
	rm -f $(PREVIEW_LOG) $(PREVIEW_PID) preview.pid $(PREVIEW_CONFIG)

preview: build
	cd $(PREVIEW_DIR) && ln -sf dashy.json config.json
	cd $(PREVIEW_DIR) && (nohup ../$(DASHY_BIN) run . > preview.log 2>&1 & echo $$! > preview.pid)
	@echo "Waiting for dashboard to become available..."
	@bash -c 'for i in $$(seq 1 20); do if curl -sI http://127.0.0.1:8080/ >/dev/null 2>&1; then exit 0; fi; sleep 0.5; done; exit 1'
	cd $(PREVIEW_DIR) && ./send_testdata.sh
	@echo "Preview server started; logs are in $(PREVIEW_LOG)"

stop-preview:
	@if [ -f $(PREVIEW_PID) ]; then \
		kill `cat $(PREVIEW_PID)` 2>/dev/null || true; \
		rm -f $(PREVIEW_PID); \
	elif [ -f preview.pid ]; then \
		kill `cat preview.pid` 2>/dev/null || true; \
		rm -f preview.pid; \
	fi
	@rm -f $(PREVIEW_CONFIG)
	@echo "Preview stopped"
