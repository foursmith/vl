.PHONY: build run test check

build:
	go build -o bin/elegit ./cmd/elegit

run:
	go run ./cmd/elegit

test:
	go test ./...

check:
	go vet ./...
	go test -race ./...
