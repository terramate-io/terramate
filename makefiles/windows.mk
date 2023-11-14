# set SHELL to system prompt.
ifdef ComSpec
SHELL := $(ComSpec)
endif
ifdef COMSPEC
SHELL := $(COMSPEC)
endif

BUILD_ENV=
EXEC_SUFFIX=.exe
DEPS = awk git go gcc
$(foreach dep,$(DEPS),\
    $(if $(shell where $(dep)),,$(error "Program $(dep) not found in PATH")))

## build a test binary -- not static, telemetry sent to localhost, etc
.PHONY: test/build
test/build: test/fakecloud
	go build -tags localhostEndpoints -o bin/test-terramate.exe ./cmd/terramate

## build bin/fakecloud
.PHONY: test/fakecloud
test/fakecloud:
	go build -o bin/fakecloud.exe ./cloud/testserver/cmd/fakecloud

## build the helper binary
.PHONY: test/helper
test/helper:
	go build -o bin/helper.exe ./cmd/terramate/e2etests/cmd/helper

## test code
.PHONY: test
.ONESHELL:
tempdir=$(shell .\bin\helper.exe tempdir)
test: test/helper build
	set TM_TEST_ROOT_TEMPDIR=$(tempdir)
	go test -timeout 20m -p 100 ./...
	.\bin\helper.exe rm $(tempdir)
	.\bin\terramate.exe run -- helper.exe true

 ## remove build artifacts
.PHONY: clean
clean:
	del /Q /F .\bin\*
