# Set default shell to bash
SHELL := /bin/bash -o pipefail -o errexit -o nounset

COVERAGE_REPORT ?= coverage.txt

addlicense=go run github.com/google/addlicense@v1.0.0

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

## check go modules are tidy
.PHONY: mod/check
mod/check:
	@./hack/mod-check

## generates coverage report
.PHONY: coverage
coverage: 
	go test -coverprofile=$(COVERAGE_REPORT) -coverpkg=./...  ./...

## generates coverage report and shows it on the browser locally
.PHONY: coverage/show
coverage/show: coverage
	go tool cover -html=$(COVERAGE_REPORT)

## test code
.PHONY: test
test: 
	go test -race ./...

## Build terramate into bin directory
.PHONY: build
build:
	go build -o bin/terramate ./cmd/terramate

## Install terramate on the host
.PHONY: install
install:
	go install ./cmd/terramate

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
