VERSION ?= 1.1.1
RELEASE_NAME := ISBN-Bridge-v$(VERSION)-linux-x64
DIST_DIR := dist
STAGE_DIR := $(DIST_DIR)/$(RELEASE_NAME)

.PHONY: all test build build-windows build-linux wheel release-linux clean run run-client test-client test-pipeline-linux

all: test test-client build build-windows

test:
	go test -v ./...

test-client:
	cd client_linux && uv run pytest

test-pipeline-linux:
	python3 tests/test_pipeline_linux.py

build:
	mkdir -p bin
	go build -o bin/isbn-bridge-server ./cmd/server

build-linux:
	mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/isbn-bridge-server ./cmd/server

build-windows:
	mkdir -p bin
	GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o bin/isbn-bridge-server.exe ./cmd/server

wheel:
	rm -rf client_linux/dist/
	cd client_linux && uv build

release-linux: build-linux wheel
	rm -rf $(STAGE_DIR) $(DIST_DIR)/$(RELEASE_NAME).tar.gz $(DIST_DIR)/$(RELEASE_NAME).tar.gz.sha256
	mkdir -p $(STAGE_DIR)/bin $(STAGE_DIR)/dist $(STAGE_DIR)/assets
	cp bin/isbn-bridge-server $(STAGE_DIR)/bin/
	cp client_linux/dist/*.whl $(STAGE_DIR)/dist/
	cp client_linux/dist/*.tar.gz $(STAGE_DIR)/dist/ 2>/dev/null || true
	cp client_linux/src/client_linux/assets/* $(STAGE_DIR)/assets/
	cp packaging/linux/isbn-bridge-client $(STAGE_DIR)/
	cp packaging/linux/isbn-bridge.desktop $(STAGE_DIR)/
	cp packaging/linux/isbn-bridge.service $(STAGE_DIR)/
	cp packaging/linux/install.sh $(STAGE_DIR)/
	cp packaging/linux/uninstall.sh $(STAGE_DIR)/
	cp scanner.conf $(STAGE_DIR)/
	cp README.md LICENSE $(STAGE_DIR)/
	tar -czf $(DIST_DIR)/$(RELEASE_NAME).tar.gz -C $(DIST_DIR) $(RELEASE_NAME)
	cd $(DIST_DIR) && sha256sum $(RELEASE_NAME).tar.gz > $(RELEASE_NAME).tar.gz.sha256
	@echo "Release archive created: $(DIST_DIR)/$(RELEASE_NAME).tar.gz"
	@cat $(DIST_DIR)/$(RELEASE_NAME).tar.gz.sha256

run:
	go run ./cmd/server

run-client:
	cd client_linux && uv run client-linux

clean:
	rm -rf bin/ dist/ client_linux/dist/
