.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/homedepot-pp-cli$(BIN_EXT) ./cmd/homedepot-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/homedepot-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/homedepot-pp-mcp$(BIN_EXT) ./cmd/homedepot-pp-mcp

install-mcp:
	go install ./cmd/homedepot-pp-mcp

build-all: build build-mcp
