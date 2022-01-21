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

## tidy up go modules
.PHONY: mod
mod:
	go mod tidy

## generates coverage report
.PHONY: coverage
coverage: 
	go test -count=1 -coverprofile=$(COVERAGE_REPORT) -coverpkg=./...  ./...

## generates coverage report and shows it on the browser locally
.PHONY: coverage/show
coverage/show: coverage
	go tool cover -html=$(COVERAGE_REPORT)

## test code
.PHONY: test
test: 
	go test -count=1 -race ./...

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

## Runs pre-commit hook
.PHONY: pre-commit
pre-commit:
	./hack/hooks/pre-commit

## Sets up pre-commit hook on the local repo
.PHONY: pre-commit/setup
pre-commit/setup:
	cp ./hack/hooks/pre-commit .git/hooks

## Cleans up pre-commit hook on the local repo
.PHONY: pre-commit/cleanup
pre-commit/cleanup:
	rm .git/hooks/pre-commit

## creates a new release tag
.PHONY: release/tag
release/tag: VERSION?=v$(shell cat VERSION)
release/tag:
	git tag -a $(VERSION) -m "Release $(VERSION)"
	git push origin $(VERSION)

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
