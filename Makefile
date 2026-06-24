SHELL := /bin/bash
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

.PHONY: build test vet lint clean install release release-dry

## build: compile the binary into ./nestor
build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o nestor .

## test: run all tests with verbose output
test:
	go test -v -count=1 ./...

## vet: run go vet
vet:
	go vet ./...

## lint: run golangci-lint (install: https://golangci-lint.run)
lint:
	golangci-lint run ./...

## install: build and install to GOPATH/bin
install:
	CGO_ENABLED=0 go install -ldflags "$(LDFLAGS)" .

## release: cross-compile binaries into dist/ (linux/darwin/windows, amd64/arm64)
release:
	@mkdir -p dist
	@for os in linux darwin windows; do \
		for arch in amd64 arm64; do \
			ext=""; [ $$os = windows ] && ext=".exe"; \
			echo "  -> $$os/$$arch"; \
			GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 \
				go build -ldflags "$(LDFLAGS)" -o dist/nestor-$$os-$$arch$$ext .; \
		done; \
	done
	@cd dist && sha256sum * > checksums-sha256.txt
	@echo "Binaries and checksums are in dist/"

## release-dry: show what release would build (no compilation)
release-dry:
	@echo "Targets: linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64"
	@echo "Output:  dist/nestor-<os>-<arch>[.exe] + dist/checksums-sha256.txt"

## clean: remove build artifacts
clean:
	rm -f nestor
	rm -rf dist/
