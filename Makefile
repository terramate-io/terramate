# Set default shell to bash
SHELL := /bin/bash -o pipefail -o errexit -o nounset 

VERSION ?= v0.0.1

DOCKER_REPO = mineiros/terrastack
DOCKER_IMAGE ?= ${DOCKER_REPO}:${VERSION}
DOCKER_RUN_FLAGS += --rm
DOCKER_RUN_FLAGS += -v ${PWD}:/src

DOCKER_TOOLS_REPO   = mineiros/golang-tools
DOCKER_TOOLS_FLAGS += -v $(PWD):/src $(DOCKER_TOOLS_REPO)

DOCKER_FLAGS   += ${DOCKER_RUN_FLAGS}
DOCKER_RUN_CMD  = docker run ${DOCKER_FLAGS}

REVIVE_ARGS += -formatter friendly -config tools/revive.toml ./...

ifndef NOCOLOR
	GREEN  := $(shell tput -Txterm setaf 2)
	YELLOW := $(shell tput -Txterm setaf 3)
	WHITE  := $(shell tput -Txterm setaf 7)
	RESET  := $(shell tput -Txterm sgr0)
endif

.PHONY: default
default: help

## build tools image (used by fmt, lint, etc)
.PHONY: build-tools
build-tools:
	docker build -t $(DOCKER_TOOLS_REPO) -f Dockerfile.tools .

## Format go code
.PHONY: fmt
fmt: build-tools
	$(call docker-run,$(DOCKER_TOOLS_FLAGS) goimports -w /src)

## lint code
.PHONY: lint
lint: build-tools
	$(call docker-run,$(DOCKER_TOOLS_FLAGS) revive $(REVIVE_ARGS))

## test code
.PHONY: test
test: build-tools
	$(call docker-run,$(DOCKER_TOOLS_FLAGS) go test -coverprofile=coverage.txt \
											-covermode=atomic ./...)

## Build terrastack into bin directory
.PHONY: build
build:
	go build -o bin/terrastack ./cmd/terrastack

.PHONY: build-image
build-image:
	docker build -t $(DOCKER_IMAGE) .

## remove bin/*
.PHONY: clean
clean:
	$(call rm-command,bin/*)

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

quiet-command = $(if ${V},${1},$(if ${2},@echo ${2} && ${1}, @${1}))

docker-run = $(call quiet-command,${DOCKER_RUN_CMD} ${1} | cat,"${YELLOW}[DOCKER RUN] ${GREEN}${1}${RESET}")
rm-command = $(call quiet-command,rm -f ${1},"${YELLOW}[CLEAN] ${GREEN}${1}${RESET}")
