BINARY := gbwatch
OUT_DIR := bin

.PHONY: build build-all build-windows build-linux build-macos test clean

build:
	mkdir -p $(OUT_DIR)
	go build -o $(OUT_DIR)/$(BINARY) ./cmd

build-all: build-windows build-linux build-macos

build-windows:
	mkdir -p $(OUT_DIR)
	GOOS=windows GOARCH=amd64 go build -o $(OUT_DIR)/$(BINARY)-windows-amd64.exe ./cmd

build-linux:
	mkdir -p $(OUT_DIR)
	GOOS=linux GOARCH=amd64 go build -o $(OUT_DIR)/$(BINARY)-linux-amd64 ./cmd

build-macos:
	mkdir -p $(OUT_DIR)
	GOOS=darwin GOARCH=amd64 go build -o $(OUT_DIR)/$(BINARY)-darwin-amd64 ./cmd
	GOOS=darwin GOARCH=arm64 go build -o $(OUT_DIR)/$(BINARY)-darwin-arm64 ./cmd

test:
	go test ./...

clean:
	rm -rf $(OUT_DIR)
