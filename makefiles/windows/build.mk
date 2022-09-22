ifdef ComSpec
SHELL := $(ComSpec)
endif
ifdef COMSPEC
SHELL := $(COMSPEC)
endif

## Build terramate into bin directory
.PHONY: build
build:
	go build -o .\bin\terramate.exe ./cmd/terramate

## Install terramate on the host
.PHONY: install
install:
	go install ./cmd/terramate

 ## remove build artifacts
.PHONY: clean
clean:
	del /Q /F .\bin\*
