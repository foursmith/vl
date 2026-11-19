.PHONY: build run test check package

VERSION ?= $(patsubst v%,%,$(shell git describe --tags --match 'v[0-9]*' --always --dirty 2>/dev/null || printf '%s' dev))
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
