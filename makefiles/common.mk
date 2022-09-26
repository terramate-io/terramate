GO_RELEASER_VERSION=v1.2.5
COVERAGE_REPORT ?= coverage.txt
RUN_ADD_LICENSE=go run github.com/google/addlicense@v1.0.0 -ignore **/*.yml

## Format go code
.PHONY: fmt
fmt:
	go run golang.org/x/tools/cmd/goimports@v0.1.7 -w .

## lint code
.PHONY: lint
lint:
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run ./...

## tidy up go modules
.PHONY: mod
mod:
	go mod tidy

## add license to code
.PHONY: license
license:
	$(RUN_ADD_LICENSE) -c "Mineiros GmbH" .

## check if code is licensed properly
.PHONY: license/check
license/check:
	$(RUN_ADD_LICENSE) --check .

## generates coverage report
.PHONY: coverage
coverage: 
	go test -count=1 -coverprofile=$(COVERAGE_REPORT) -coverpkg=./...  ./...

## generates coverage report and shows it on the browser locally
.PHONY: coverage/show
coverage/show: coverage
	go tool cover -html=$(COVERAGE_REPORT)

## start fuzzying to generate some new corpus/find errors on partial eval
.PHONY: test/fuzz/eval
test/fuzz/eval:
	go test ./hcl/eval -fuzz=FuzzPartialEval

## start fuzzying to generate some new corpus/find errors on formatting
.PHONY: test/fuzz/fmt
test/fuzz/fmt:
	go test ./hcl -fuzz=FuzzFormatMultiline

## creates a new release tag
.PHONY: release/tag
release/tag:
	git tag -a $(VERSION) -m "Release $(VERSION)"
	git push origin $(VERSION)

## executes a dry run of the release process
.PHONY: release/dry-run
release/dry-run: 
	go run github.com/goreleaser/goreleaser@$(GO_RELEASER_VERSION) release --snapshot --rm-dist

## generates a terramate release
.PHONY: release
release: 
	go run github.com/goreleaser/goreleaser@$(GO_RELEASER_VERSION) release --rm-dist

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
