# set SHELL to system prompt.
ifdef ComSpec
SHELL := $(ComSpec)
endif
ifdef COMSPEC
SHELL := $(COMSPEC)
endif

VERSION ?= v$(shell type VERSION)
GOLANGCI_LINT_VERSION ?= v1.49.0
