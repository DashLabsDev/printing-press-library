.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/costco-sameday-pp-cli$(BIN_EXT) ./cmd/costco-sameday-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/costco-sameday-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/costco-sameday-pp-mcp$(BIN_EXT) ./cmd/costco-sameday-pp-mcp

install-mcp:
	go install ./cmd/costco-sameday-pp-mcp

build-all: build build-mcp
