BINARY     := latexci
MODULE     := github.com/sitrakaforler/latexci
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS    := -s -w -X main.version=$(VERSION)
BUILD_OPTS := CGO_ENABLED=0

.PHONY: build test lint clean release install

build:
	$(BUILD_OPTS) go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/$(BINARY)

install:
	$(BUILD_OPTS) go install -ldflags "$(LDFLAGS)" ./cmd/$(BINARY)

test:
	go test -race -cover ./...

lint:
	golangci-lint run ./...

vet:
	go vet ./...

clean:
	rm -rf bin/ dist/

# Cross-compile for Linux, macOS, Windows (amd64 + arm64)
release:
	goreleaser release --clean

release-snapshot:
	goreleaser release --snapshot --clean

# Quick local dev: build then run build command
dev: build
	./bin/$(BINARY) build

fmt:
	gofmt -w .

.DEFAULT_GOAL := build
