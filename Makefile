.PHONY: build run test check package

VERSION ?= 0.1.0-dev
OUTPUT ?= bin/vl

build:
	go build -trimpath -ldflags="-s -w -X main.buildVersion=$(VERSION)" -o "$(OUTPUT)" ./cmd/vl

package:
	bash scripts/package-release.sh "$(VERSION)"

run:
	go run ./cmd/vl

test:
	go test ./...

check:
	go vet ./...
	go test -race ./...
