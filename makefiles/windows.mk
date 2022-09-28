# set SHELL to system prompt.
ifdef ComSpec
SHELL := $(ComSpec)
endif
ifdef COMSPEC
SHELL := $(COMSPEC)
endif

DEPS = awk git go gcc
$(foreach dep,$(DEPS),\
    $(if $(shell where $(dep)),,$(error "Program $(dep) not found in PATH")))

## Build terramate into bin directory
.PHONY: build
build:
	go build -o .\bin\terramate.exe ./cmd/terramate

## Install terramate on the host
.PHONY: install
install:
	go install ./cmd/terramate

## test code
.PHONY: test
test:
	go test ./... -timeout=20m

 ## remove build artifacts
.PHONY: clean
clean:
	del /Q /F .\bin\*
