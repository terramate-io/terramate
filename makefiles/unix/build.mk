GO_BUILD_FLAGS=--ldflags '-extldflags "-static"'

## Build terramate into bin directory
.PHONY: build
build:
	CGO_ENABLED=0 go build $(GO_BUILD_FLAGS) -o bin/terramate ./cmd/terramate

## Install terramate on the host
.PHONY: install
install:
	CGO_ENABLED=0 go install $(GO_BUILD_FLAGS) ./cmd/terramate

## remove build artifacts
.PHONY: clean
clean:
	rm -rf bin/*
