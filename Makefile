.PHONY: build run test check package

VERSION ?= $(patsubst v%,%,$(shell git describe --tags --match 'v[0-9]*' --always --dirty 2>/dev/null || printf '%s' dev))
OUTPUT ?= bin/vl
GO_PACKAGES = ./cmd/... ./internal/...

build:
	go build -trimpath -ldflags="-s -w -X main.buildVersion=$(VERSION)" -o "$(OUTPUT)" ./cmd/vl

package:
	bash scripts/package-release.sh "$(VERSION)"

run:
	go run ./cmd/vl

test:
	go test $(GO_PACKAGES)

check:
	go vet $(GO_PACKAGES)
	go test -race $(GO_PACKAGES)
