.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/ofa-health-clearances-pp-cli$(BIN_EXT) ./cmd/ofa-health-clearances-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/ofa-health-clearances-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/ofa-health-clearances-pp-mcp$(BIN_EXT) ./cmd/ofa-health-clearances-pp-mcp

install-mcp:
	go install ./cmd/ofa-health-clearances-pp-mcp

build-all: build build-mcp
