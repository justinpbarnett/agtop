.PHONY: build run install lint clean

build:
	go build -o bin/agtop ./cmd/agtop

run:
	go run ./cmd/agtop

install:
	go install ./cmd/agtop

lint:
	go vet ./...

clean:
	rm -rf bin/
