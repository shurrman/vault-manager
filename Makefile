GO ?= go
BINARY ?= bin/vault-manager
VERSION ?= 0.1
DIST_DIR ?= dist
RELEASE_BINARY := $(DIST_DIR)/vault-manager_$(VERSION)_linux_amd64
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build release test test-race vet check

build:
	mkdir -p $(dir $(BINARY))
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOAMD64=v1 $(GO) build -trimpath -ldflags="$(LDFLAGS)" -o $(BINARY) ./cmd/vault-manager

release:
	mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOAMD64=v1 $(GO) build -trimpath -ldflags="$(LDFLAGS)" -o $(RELEASE_BINARY) ./cmd/vault-manager
	cd $(DIST_DIR) && sha256sum $(notdir $(RELEASE_BINARY)) > $(notdir $(RELEASE_BINARY)).sha256

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...

check:
	$(GO) run ./cmd/vault-manager --check
