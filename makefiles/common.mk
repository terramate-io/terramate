GO_RELEASER_VERSION=v1.14.0
GOLANGCI_LINT_VERSION ?= v1.49.0
COVERAGE_REPORT ?= coverage.txt
RUN_ADD_LICENSE=go run github.com/google/addlicense@v1.0.0 -ignore **/*.yml
BENCH_CHECK=go run github.com/madlambda/benchcheck/cmd/benchcheck@743137fbfd827958b25ab6b13fa1180e0e933eb1

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

## start fuzzying to generate some new corpus/find errors on ast.TokensForExpression
.PHONY: test/fuzz/tokens-for-expr
test/fuzz/tokens-for-expr:
	go test ./hcl/ast -fuzz=FuzzTokensForExpression

## runs all benchmarks on the given 'pkg', or a specific benchmark inside 'pkg'
.PHONY: bench
bench: name?=.
bench: pkg?=.
bench: time=1s
bench:
	@go test -bench=$(name) -count=5 -benchtime=$(time) -benchmem $(pkg)

## benchmark all packages
.PHONY: bench/all
bench/all: time=1s
bench/all: dir=.
bench/all:
	@for benchfile in $(shell find $(dir) | grep _bench_); do \
		go test -bench=. -benchtime=$(time) -benchmem $$(dirname $$benchfile); \
	done

## check benchmark
.PHONY: bench/check
bench/check: allocdelta="+20%"
bench/check: timedelta="+20%"
bench/check: name=github.com/mineiros-io/terramate
bench/check: pkg=./...
bench/check: old=main
bench/check: new?=$(shell git rev-parse HEAD)
bench/check:
	@$(BENCH_CHECK) -mod $(name) -pkg $(pkg) -go-test-flags "-benchmem,-count=5" \
		-old $(old) -new $(new) \
		-check allocs/op=$(allocdelta) \
		-check time/op=$(timedelta)

## cleanup artifacts produced by the benchmarking process
bench/cleanup:
	rm -f *.prof
	rm -f *.test

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
