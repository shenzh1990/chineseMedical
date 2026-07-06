APP_NAME := chinese-medical
BINARY := server

TARGET_GOOS ?= linux
TARGET_GOARCH ?= amd64
DIST_DIR ?= dist
DEPLOY_BINARY := $(DIST_DIR)/$(BINARY)-$(TARGET_GOOS)-$(TARGET_GOARCH)

REMOTE_HOST ?= hangali
REMOTE_DIR ?= /root/chinesemedical
REMOTE_CONFIG ?= configs/config.yaml

.PHONY: run build build-linux test tidy deploy

run:
	go run ./cmd/server

build:
	go build -trimpath -ldflags="-s -w" -o bin/server ./cmd/server

build-linux:
	mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=$(TARGET_GOOS) GOARCH=$(TARGET_GOARCH) go build -trimpath -ldflags="-s -w" -o $(DEPLOY_BINARY) ./cmd/server

test:
	go test ./...

tidy:
	go mod tidy

deploy: test build-linux
	ssh $(REMOTE_HOST) 'mkdir -p "$(REMOTE_DIR)/configs" "$(REMOTE_DIR)/generated" "$(REMOTE_DIR)/logs"'
	scp $(DEPLOY_BINARY) $(REMOTE_HOST):$(REMOTE_DIR)/$(BINARY).new
	ssh $(REMOTE_HOST) 'set -eu; \
		cd "$(REMOTE_DIR)"; \
		if [ -f "$(BINARY).pid" ]; then \
			pid=$$(cat "$(BINARY).pid"); \
			if [ -n "$$pid" ] && kill -0 "$$pid" 2>/dev/null; then \
				kill "$$pid"; \
				for i in 1 2 3 4 5; do \
					if kill -0 "$$pid" 2>/dev/null; then sleep 1; else break; fi; \
				done; \
				if kill -0 "$$pid" 2>/dev/null; then kill -9 "$$pid"; fi; \
			fi; \
			rm -f "$(BINARY).pid"; \
		fi; \
		pkill -f "$(REMOTE_DIR)/[s]erver" 2>/dev/null || true; \
		if [ -x "$(BINARY)" ]; then mv "$(BINARY)" "$(BINARY).bak.$$(date +%Y%m%d%H%M%S)"; fi; \
		mv "$(BINARY).new" "$(BINARY)"; \
		chmod +x "$(BINARY)"; \
		if [ ! -f "$(REMOTE_CONFIG)" ]; then cp configs/config.yaml.example "$(REMOTE_CONFIG)"; fi; \
		nohup env CONFIG_FILE="$(REMOTE_CONFIG)" ./$(BINARY) > logs/$(BINARY).log 2>&1 & \
		echo $$! > "$(BINARY).pid"; \
		sleep 1; \
		kill -0 $$(cat "$(BINARY).pid")'
	@echo "deployed $(APP_NAME) to $(REMOTE_HOST):$(REMOTE_DIR)"
