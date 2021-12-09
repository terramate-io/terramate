# Set default shell to bash
SHELL := /bin/bash -o pipefail -o errexit -o nounset

addlicense=go run github.com/google/addlicense@v1.0.0
version?=0.0.3-$(shell git rev-parse --short HEAD)
buildflags=-ldflags "-X github.com/mineiros-io/terramate.Version=$(version)"

.PHONY: default
default: help

## Format go code
.PHONY: fmt
fmt:
	go run golang.org/x/tools/cmd/goimports@v0.1.7 -w .

## lint code
.PHONY: lint
lint:
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.43.0 run ./...

## add license to code
.PHONY: license
license:
	$(addlicense) -c "Mineiros GmbH" .

## check if code is licensed properly
.PHONY: license/check
license/check:
	$(addlicense) --check .

## test code
.PHONY: test
test: 
	go test -race -coverprofile=coverage.txt -covermode=atomic ./...

## Build terramate into bin directory
.PHONY: build
build:
	go build $(buildflags) -o bin/terramate ./cmd/terramate

## Install terramate on the host
.PHONY: install
install:
	go install $(buildflags) ./cmd/terramate

## remove build artifacts
.PHONY: clean
clean:
	rm -rf bin/*

## Display help for all targets
.PHONY: help
help:
	@awk '/^.PHONY: / { \
		msg = match(lastLine, /^## /); \
			if (msg) { \
				cmd = substr($$0, 9, 100); \
				msg = substr(lastLine, 4, 1000); \
				printf "  ${GREEN}%-30s${RESET} %s\n", cmd, msg; \
			} \
	} \
	{ lastLine = $$0 }' $(MAKEFILE_LIST)
