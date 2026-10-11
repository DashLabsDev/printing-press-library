.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/neon-pp-cli$(BIN_EXT) ./cmd/neon-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/neon-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/neon-pp-mcp$(BIN_EXT) ./cmd/neon-pp-mcp

install-mcp:
	go install ./cmd/neon-pp-mcp

build-all: build build-mcp
