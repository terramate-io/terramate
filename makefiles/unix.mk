# Set default shell to bash
SHELL := /bin/bash -o pipefail -o errexit -o nounset
VERSION ?= v$(shell cat VERSION)
GO_BUILD_FLAGS=--ldflags '-extldflags "-static"'
BUILD_ENV=CGO_ENABLED=0
EXEC_SUFFIX=

GIT_MIN_MAJOR_VERSION=2
GIT_MIN_MINOR_VERSION=28
GIT_MIN_VERSION=$(GIT_MIN_MAJOR_VERSION).$(GIT_MIN_MINOR_VERSION).0
GIT_CURRENT_VERSION=$(shell git --version | grep ^git | cut -d' ' -f3)
GIT_CURRENT_MAJOR_VERSION=$(shell echo $(GIT_CURRENT_VERSION) | cut -d. -f1)
GIT_CURRENT_MINOR_VERSION=$(shell echo $(GIT_CURRENT_VERSION) | cut -d. -f2)

IS_VALID_GIT_VERSION=$(shell expr $(GIT_CURRENT_MAJOR_VERSION) '>=' $(GIT_MIN_MAJOR_VERSION) '&' $(GIT_CURRENT_MINOR_VERSION) '>=' $(GIT_MIN_MINOR_VERSION))


ifneq "$(IS_VALID_GIT_VERSION)" "1"
$(error "$(IS_VALID_GIT_VERSION) Unsupported git version $(GIT_CURRENT_VERSION). Minimum supported version: $(GIT_MIN_VERSION)")
endif


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
	go build -o bin/helper ./cmd/terramate/e2etests/cmd/helper

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
