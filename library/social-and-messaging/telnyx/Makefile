.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/telnyx-pp-cli$(BIN_EXT) ./cmd/telnyx-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/telnyx-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/telnyx-pp-mcp$(BIN_EXT) ./cmd/telnyx-pp-mcp

install-mcp:
	go install ./cmd/telnyx-pp-mcp

build-all: build build-mcp
