# PullAI Makefile
BINARY  := pullai
VERSION := 0.1.0
MODULE  := github.com/pullai/pullai
MAIN    := ./cmd/pullai

.PHONY: build build-all clean install test lint deps

## Build for current platform
build:
	go build -ldflags="-s -w -X main.Version=$(VERSION)" -o $(BINARY) $(MAIN)
	@echo "✅  Built: ./$(BINARY)"

## Build for all platforms
build-all:
	@mkdir -p dist
	GOOS=linux   GOARCH=amd64 go build -ldflags="-s -w" -o dist/$(BINARY)-linux-amd64   $(MAIN)
	GOOS=linux   GOARCH=arm64 go build -ldflags="-s -w" -o dist/$(BINARY)-linux-arm64   $(MAIN)
	GOOS=darwin  GOARCH=amd64 go build -ldflags="-s -w" -o dist/$(BINARY)-darwin-amd64  $(MAIN)
	GOOS=darwin  GOARCH=arm64 go build -ldflags="-s -w" -o dist/$(BINARY)-darwin-arm64  $(MAIN)
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o dist/$(BINARY)-windows-amd64.exe $(MAIN)
	@echo "✅  All binaries in ./dist/"

## Install to /usr/local/bin (requires sudo on Linux/macOS)
install: build
	cp $(BINARY) /usr/local/bin/$(BINARY)
	@echo "✅  Installed to /usr/local/bin/$(BINARY)"

## Download Go dependencies
deps:
	go mod tidy
	go mod download

## Run tests
test:
	go test ./... -v

## Lint (requires golangci-lint)
lint:
	golangci-lint run ./...

## Clean build artifacts
clean:
	rm -f $(BINARY)
	rm -rf dist/
