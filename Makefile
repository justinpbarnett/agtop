VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-X github.com/justinpbarnett/agtop/internal/ui/panels.Version=$(VERSION)"

.PHONY: build run install lint clean

build:
	go build $(LDFLAGS) -o bin/agtop ./cmd/agtop

run:
	go run $(LDFLAGS) ./cmd/agtop

install:
	go install $(LDFLAGS) ./cmd/agtop

lint:
	go vet ./...

clean:
	rm -rf bin/
