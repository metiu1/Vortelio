# Vortelio Makefile
BINARY  := vortelio
MODULE  := github.com/vortelio/vortelio
MAIN    := ./cmd/vortelio
VERSION := $(shell grep 'Version = ' ./internal/version/version.go | sed 's/.*"\(.*\)".*/\1/')
LDFLAGS := -s -w -X $(MODULE)/internal/version.Version=$(VERSION)

NSI_SCRIPT := ../vortelio-installer/vortelio-installer.nsi

.PHONY: build build-all clean install test lint deps installer

## Build for current platform
build:
	go build -ldflags="$(LDFLAGS)" -o $(BINARY).exe $(MAIN)
	@echo "Built: ./$(BINARY).exe  v$(VERSION)"

## Build Windows release binary
build-windows:
	@mkdir -p dist
	GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-windows-amd64.exe $(MAIN)
	@echo "Built: dist/$(BINARY)-windows-amd64.exe  v$(VERSION)"

## Build for all platforms
build-all:
	@mkdir -p dist
	GOOS=linux   GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-linux-amd64   $(MAIN)
	GOOS=linux   GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-linux-arm64   $(MAIN)
	GOOS=darwin  GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-darwin-amd64  $(MAIN)
	GOOS=darwin  GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-darwin-arm64  $(MAIN)
	GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-windows-amd64.exe $(MAIN)
	@echo "All binaries in ./dist/"

## Build Windows installer (requires NSIS)
installer: build-windows
	cp dist/$(BINARY)-windows-amd64.exe ../vortelio-installer/build/vortelio.exe
	cp dist/$(BINARY)-windows-amd64.exe ../vortelio-installer/build/vortelio-server.exe
	"C:/Program Files (x86)/NSIS/makensis.exe" /DPRODUCT_VERSION=$(VERSION) $(NSI_SCRIPT)
	@echo "Installer built: vortelio-$(VERSION)-setup.exe"

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
	rm -f $(BINARY).exe
	rm -rf dist/
