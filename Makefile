VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BINARY  := voicaa
GOFLAGS := -ldflags "-X github.com/Krunal96369/voicaa/internal/cli.Version=$(VERSION)"

.PHONY: build install clean

build:
	go build $(GOFLAGS) -o $(BINARY) ./cmd/voicaa

install:
	go install $(GOFLAGS) ./cmd/voicaa

clean:
	rm -f $(BINARY)

.PHONY: fmt vet test
fmt:
	go fmt ./...

vet:
	go vet ./...

test:
	go test ./...
