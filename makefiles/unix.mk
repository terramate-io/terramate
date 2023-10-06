# Set default shell to bash
SHELL := /bin/bash -o pipefail -o errexit -o nounset
VERSION ?= v$(shell cat VERSION)
GO_BUILD_FLAGS=--ldflags '-extldflags "-static"'
BUILD_ENV=CGO_ENABLED=0
EXEC_SUFFIX=

## build a test binary -- not static, telemetry sent to localhost, etc
.PHONY: test/build
test/build: test/fakecloud
	go build -tags localhostEndpoints -o bin/test-terramate ./cmd/terramate

## build bin/fakecloud
.PHONY: test/fakecloud
test/fakecloud:
	go build -o bin/fakecloud ./cloud/testserver/cmd/fakecloud

## build the helper binary
.PHONY: test/helper
test/helper:
	go build -o bin/helper ./cmd/terramate/e2etests/cmd/test
	cp bin/helper bin/terraform

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
	git tag -s -a $(VERSION) -m "Release $(VERSION)"
	git push origin $(VERSION)

## remove build artifacts
.PHONY: clean
clean:
	rm -rf bin/*
