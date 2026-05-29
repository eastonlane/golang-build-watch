BINARY := gbwatch
OUT_DIR := bin

.PHONY: build build-all build-windows build-linux build-macos test clean

build:
	if not exist $(OUT_DIR) mkdir $(OUT_DIR)
	go build -o $(OUT_DIR)/$(BINARY) ./cmd

build-all: build-windows build-linux build-macos

build-windows:
	if not exist $(OUT_DIR) mkdir $(OUT_DIR)
	set GOOS=windows&& set GOARCH=amd64&& go build -o $(OUT_DIR)/$(BINARY)-windows-amd64.exe ./cmd

build-linux:
	if not exist $(OUT_DIR) mkdir $(OUT_DIR)
	set GOOS=linux&& set GOARCH=amd64&& go build -o $(OUT_DIR)/$(BINARY)-linux-amd64 ./cmd

build-macos:
	if not exist $(OUT_DIR) mkdir $(OUT_DIR)
	set GOOS=darwin&& set GOARCH=amd64&& go build -o $(OUT_DIR)/$(BINARY)-darwin-amd64 ./cmd
	set GOOS=darwin&& set GOARCH=arm64&& go build -o $(OUT_DIR)/$(BINARY)-darwin-arm64 ./cmd

test:
	go test ./...

clean:
	if exist $(OUT_DIR) rmdir /s /q $(OUT_DIR)
