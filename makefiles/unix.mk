# Set default shell to bash
SHELL := /bin/bash -o pipefail -o errexit -o nounset
VERSION ?= v$(shell cat VERSION)
GO_BUILD_FLAGS=--ldflags '-extldflags "-static"'

## Build terramate into bin directory
.PHONY: build
build:
	CGO_ENABLED=0 go build $(GO_BUILD_FLAGS) -o bin/terramate ./cmd/terramate

## Install terramate on the host
.PHONY: install
install:
	CGO_ENABLED=0 go install $(GO_BUILD_FLAGS) ./cmd/terramate

## test code
.PHONY: test
test: 
	go test -count=1 -race ./...

## test if terramate works with CI git environment.
.PHONY: test/ci
test/ci: build
	./bin/terramate list --changed

## check go modules are tidy
.PHONY: mod/check
mod/check:
	@./hack/mod-check

## creates a new release tag
.PHONY: release/tag
release/tag: VERSION?=v$(shell cat VERSION)
release/tag:
	git tag -a $(VERSION) -m "Release $(VERSION)"
	git push origin $(VERSION)

## remove build artifacts
.PHONY: clean
clean:
	rm -rf bin/*
