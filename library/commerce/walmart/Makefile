.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/walmart-pp-cli$(BIN_EXT) ./cmd/walmart-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/walmart-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/walmart-pp-mcp$(BIN_EXT) ./cmd/walmart-pp-mcp

install-mcp:
	go install ./cmd/walmart-pp-mcp

build-all: build build-mcp
