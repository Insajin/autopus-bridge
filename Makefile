VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.date=$(DATE)

BINARY := autopus-bridge

.PHONY: build test lint vet clean release-dry-run release

build:
	CGO_ENABLED=0 go build -ldflags '$(LDFLAGS)' -o $(BINARY) .

test:
	go test -v ./...

lint:
	golangci-lint run ./...

vet:
	go vet ./...

clean:
	rm -f $(BINARY)
	go clean

release-dry-run:
	goreleaser release --snapshot --clean

release:
	goreleaser release --clean
