# WorkBuddy AI CLIProxyAPI Plugin Makefile
.PHONY: all build build-windows build-linux build-darwin clean test

BINARY_NAME=workbuddy
OUT_DIR=bin
PLUGIN_PKG=./cmd/workbuddy-plugin
LDFLAGS=-s -w

all: build

build:
ifeq ($(OS),Windows_NT)
	@echo "Building Windows DLL..."
	@if not exist $(OUT_DIR) mkdir $(OUT_DIR)
	go build -buildmode=c-shared -ldflags="$(LDFLAGS)" -o $(OUT_DIR)/$(BINARY_NAME).dll $(PLUGIN_PKG)
else
	@echo "Building Linux/Unix Shared Object..."
	@mkdir -p $(OUT_DIR)
	go build -buildmode=c-shared -ldflags="$(LDFLAGS)" -o $(OUT_DIR)/$(BINARY_NAME).so $(PLUGIN_PKG)
endif

build-windows:
	@mkdir -p $(OUT_DIR)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc go build -buildmode=c-shared -ldflags="$(LDFLAGS)" -o $(OUT_DIR)/$(BINARY_NAME).dll $(PLUGIN_PKG)

build-linux:
	@mkdir -p $(OUT_DIR)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build -buildmode=c-shared -ldflags="$(LDFLAGS)" -o $(OUT_DIR)/$(BINARY_NAME).so $(PLUGIN_PKG)

build-linux-arm64:
	@mkdir -p $(OUT_DIR)
	GOOS=linux GOARCH=arm64 CGO_ENABLED=1 CC=aarch64-linux-gnu-gcc go build -buildmode=c-shared -ldflags="$(LDFLAGS)" -o $(OUT_DIR)/$(BINARY_NAME)_arm64.so $(PLUGIN_PKG)

build-darwin:
	@mkdir -p $(OUT_DIR)
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 go build -buildmode=c-shared -ldflags="$(LDFLAGS)" -o $(OUT_DIR)/$(BINARY_NAME).dylib $(PLUGIN_PKG)

test:
	go test -v ./...

clean:
ifeq ($(OS),Windows_NT)
	@if exist $(OUT_DIR) rd /s /q $(OUT_DIR)
else
	@rm -rf $(OUT_DIR)
endif
