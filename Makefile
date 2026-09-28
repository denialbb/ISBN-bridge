.PHONY: all test build build-windows clean run test-client run-client test-pipeline-linux

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

build-windows:
	mkdir -p bin
	GOOS=windows GOARCH=amd64 go build -o bin/isbn-bridge-server.exe ./cmd/server

run:
	go run ./cmd/server

run-client:
	cd client_linux && uv run client-linux

clean:
	rm -rf bin/
