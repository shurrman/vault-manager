GO ?= go
PYTHON ?= python3
BINARY ?= bin/vault-manager
VERSION ?= 0.4
DIST_DIR ?= dist
RELEASE_PREFIX := vault-manager_$(VERSION)
RELEASE_ARCHIVES := \
	$(DIST_DIR)/$(RELEASE_PREFIX)_linux_amd64.tar.gz \
	$(DIST_DIR)/$(RELEASE_PREFIX)_linux_arm64.tar.gz \
	$(DIST_DIR)/$(RELEASE_PREFIX)_darwin_amd64.tar.gz \
	$(DIST_DIR)/$(RELEASE_PREFIX)_darwin_arm64.tar.gz \
	$(DIST_DIR)/$(RELEASE_PREFIX)_windows_amd64.zip
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build release test test-race vet check

build:
	mkdir -p $(dir $(BINARY))
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOAMD64=v1 $(GO) build -trimpath -ldflags="$(LDFLAGS)" -o $(BINARY) ./cmd/vault-manager

release:
	mkdir -p $(DIST_DIR)/stage/linux_amd64 $(DIST_DIR)/stage/linux_arm64
	mkdir -p $(DIST_DIR)/stage/darwin_amd64 $(DIST_DIR)/stage/darwin_arm64
	mkdir -p $(DIST_DIR)/stage/windows_amd64
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOAMD64=v1 $(GO) build -trimpath -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/stage/linux_amd64/vault-manager ./cmd/vault-manager
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 GOARM64=v8.0 $(GO) build -trimpath -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/stage/linux_arm64/vault-manager ./cmd/vault-manager
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 GOAMD64=v1 $(GO) build -trimpath -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/stage/darwin_amd64/vault-manager ./cmd/vault-manager
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 GOARM64=v8.0 $(GO) build -trimpath -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/stage/darwin_arm64/vault-manager ./cmd/vault-manager
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 GOAMD64=v1 $(GO) build -trimpath -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/stage/windows_amd64/vault-manager.exe ./cmd/vault-manager
	tar -czf $(abspath $(DIST_DIR)/$(RELEASE_PREFIX)_linux_amd64.tar.gz) -C $(DIST_DIR)/stage/linux_amd64 vault-manager
	tar -czf $(abspath $(DIST_DIR)/$(RELEASE_PREFIX)_linux_arm64.tar.gz) -C $(DIST_DIR)/stage/linux_arm64 vault-manager
	tar -czf $(abspath $(DIST_DIR)/$(RELEASE_PREFIX)_darwin_amd64.tar.gz) -C $(DIST_DIR)/stage/darwin_amd64 vault-manager
	tar -czf $(abspath $(DIST_DIR)/$(RELEASE_PREFIX)_darwin_arm64.tar.gz) -C $(DIST_DIR)/stage/darwin_arm64 vault-manager
	cd $(DIST_DIR)/stage/windows_amd64 && $(PYTHON) -m zipfile -c $(abspath $(DIST_DIR)/$(RELEASE_PREFIX)_windows_amd64.zip) vault-manager.exe
	cd $(DIST_DIR) && sha256sum $(notdir $(RELEASE_ARCHIVES)) > checksums.txt

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...

check:
	$(GO) run ./cmd/vault-manager --check
