.PHONY: build run test check

build:
	go build -trimpath -ldflags="-s -w" -o bin/vl ./cmd/vl

run:
	go run ./cmd/vl

test:
	go test ./...

check:
	go vet ./...
	go test -race ./...
