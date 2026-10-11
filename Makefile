VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

.PHONY: build install test vet fmt snapshot

build:
	go build -ldflags "$(LDFLAGS)" -o bin/gy ./cmd/gy

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/gy

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

# Build the release artifacts into dist/ without publishing. Requires goreleaser.
snapshot:
	goreleaser release --snapshot --clean
