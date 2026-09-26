.PHONY: all test build build-windows clean run

all: test build build-windows

test:
	go test -v ./...

build:
	mkdir -p bin
	go build -o bin/biblios-server ./cmd/server

build-windows:
	mkdir -p bin
	GOOS=windows GOARCH=amd64 go build -o bin/biblios-server.exe ./cmd/server

run:
	go run ./cmd/server

clean:
	rm -rf bin/
