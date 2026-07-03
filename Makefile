BINARY := gbwatch
OUT_DIR := bin
VERSION ?= $(shell git rev-parse --short HEAD)
LDFLAGS := -X golang-build-watch/internal/version.Value=$(VERSION)

.PHONY: build build-all build-windows build-linux build-macos test clean

build:
	mkdir -p $(OUT_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(OUT_DIR)/$(BINARY) ./cmd

build-all: build-windows build-linux build-macos

build-windows:
	mkdir -p $(OUT_DIR)
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(OUT_DIR)/$(BINARY)-windows-amd64.exe ./cmd

build-linux:
	mkdir -p $(OUT_DIR)
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(OUT_DIR)/$(BINARY)-linux-amd64 ./cmd

build-macos:
	mkdir -p $(OUT_DIR)
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(OUT_DIR)/$(BINARY)-darwin-amd64 ./cmd
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(OUT_DIR)/$(BINARY)-darwin-arm64 ./cmd

test:
	go test ./...

clean:
	rm -rf $(OUT_DIR)
