APP := 7d2d-mod-tracker
MISE ?= mise
GO ?= $(MISE) exec -- go
GO_BIN := $(shell $(GO) env GOPATH)/bin
FYNE ?= $(GO_BIN)/fyne
FYNE_CROSS ?= $(GO_BIN)/fyne-cross
FYNE_CROSS_LINUX_IMAGE := 7d2d-fyne-cross-linux
BUILD_DIR := build
LDFLAGS := -s -w

.PHONY: help test run build package-native tools cross-image-linux \
	cross-all cross-linux cross-windows cross-darwin cross-freebsd clean

help:
	@echo "Development:"
	@echo "  make test             Run all Go tests"
	@echo "  make run              Run the app locally"
	@echo "  make build            Build a native executable in build/"
	@echo "  make package-native   Create a native Fyne distribution package"
	@echo ""
	@echo "Cross-compilation (requires Docker and fyne-cross):"
	@echo "  make cross-all        Build every supported desktop target"
	@echo "  make cross-linux      Linux: amd64, 386, arm64, arm"
	@echo "  make cross-windows    Windows: amd64, 386, arm64"
	@echo "  make cross-darwin     macOS: amd64, arm64"
	@echo "  make cross-freebsd    FreeBSD: amd64, arm64"

test:
	$(GO) test ./...

run:
	$(GO) run .

build: test
	mkdir -p $(BUILD_DIR)
	$(GO) build -tags=migrated_fynedo -trimpath -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(APP) .

package-native: test
	$(FYNE) package -release

tools:
	$(GO) install fyne.io/tools/cmd/fyne@latest
	$(GO) install github.com/fyne-io/fyne-cross@latest

cross-all: test cross-linux cross-windows cross-darwin cross-freebsd

cross-image-linux:
	docker build --pull -t $(FYNE_CROSS_LINUX_IMAGE) \
		-f packaging/fyne-cross-linux.Dockerfile .

cross-linux: cross-image-linux
	$(FYNE_CROSS) linux -image $(FYNE_CROSS_LINUX_IMAGE) \
		-arch=amd64,386,arm64,arm -output $(APP) .

cross-windows:
	$(FYNE_CROSS) windows --pull -arch=amd64,386,arm64 -output $(APP) .

cross-darwin:
	$(FYNE_CROSS) darwin --pull -arch=amd64,arm64 -output $(APP) .

cross-freebsd:
	$(FYNE_CROSS) freebsd --pull -arch=amd64,arm64 -output $(APP) .

clean:
	rm -rf $(BUILD_DIR) fyne-cross
