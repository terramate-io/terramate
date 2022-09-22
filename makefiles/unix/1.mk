# Set default shell to bash
SHELL := /bin/bash -o pipefail -o errexit -o nounset
VERSION ?= v$(shell cat VERSION)
GOLANGCI_LINT_VERSION ?= v1.41.0
