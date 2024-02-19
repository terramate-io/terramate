GO_RELEASER_VERSION=v1.14.0
GOLANGCI_LINT_VERSION ?= v1.52.2
COVERAGE_REPORT ?= coverage.txt
RUN_ADD_LICENSE=go run github.com/google/addlicense@v1.0.0 -l mpl -s=only -ignore 'docs/**'
BENCH_CHECK=go run github.com/madlambda/benchcheck/cmd/benchcheck@743137fbfd827958b25ab6b13fa1180e0e933eb1

## Build terramate tools into bin directory
.PHONY: build
build: build/terramate build/terramate-ls
	@echo "all built successfully"

## Build the terramate binary
.PHONY: build/terramate
build/terramate:
	$(BUILD_ENV) go build $(GO_BUILD_FLAGS) -o bin/terramate$(EXEC_SUFFIX) ./cmd/terramate

## Build the terramate-ls binary
.PHONY: build/terramate-ls
build/terramate-ls:
	$(BUILD_ENV) go build $(GO_BUILD_FLAGS) -o bin/terramate-ls$(EXEC_SUFFIX) ./cmd/terramate-ls

## Build tgdeps
.PHONY: build/tgdeps
build/tgdeps:
	$(BUILD_ENV) go build $(GO_BUILD_FLAGS) -o bin/tgdeps$(EXEC_SUFFIX) ./cmd/tgdeps

## Install terramate tools on the host
.PHONY: install
install: install/terramate install/terramate-ls
	@echo "all tools installed successfully"

## Install the terramate binary
.PHONY: install/terramate
install/terramate:
	$(BUILD_ENV) go install $(GO_BUILD_FLAGS) ./cmd/terramate

## Install the terramate-ls binary
.PHONY: install/terramate-ls
install/terramate-ls:
	$(BUILD_ENV) go install $(GO_BUILD_FLAGS) ./cmd/terramate-ls

## Install tgdeps
.PHONY: install/tgdeps
install/tgdeps:
	$(BUILD_ENV) go install $(GO_BUILD_FLAGS) ./cmd/tgdeps

.PHONY: generate
generate:
	./bin/terramate generate

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
	$(RUN_ADD_LICENSE) -c "Terramate GmbH" .

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

## run tests within docker
.PHONY: test/docker
test/docker:
	docker build --progress=plain --rm -f containers/test/Dockerfile .

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
bench/check: name=github.com/terramate-io/terramate
bench/check: pkg?=./...
bench/check: old?=main
bench/check: new?=$(shell git rev-parse HEAD)
bench/check:
	@$(BENCH_CHECK) -mod $(name) -pkg $(pkg) -go-test-flags "-benchmem,-count=20,-run=Bench" \
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

## sync the Terramate example stack with a success status.
.PHONY: cloud/sync/ok
cloud/sync/ok: build test/helper
	./bin/terramate --log-level=info		\
			--disable-check-git-untracked   \
			--disable-check-git-uncommitted \
			--tags test \
			run --cloud-sync-deployment --  \
			$(PWD)/bin/helper true

## sync the Terramate example stack with a failed status.
.PHONY: cloud/sync/failed
cloud/sync/failed: build test/helper
	./bin/terramate --log-level=info		\
			--disable-check-git-untracked   \
			--disable-check-git-uncommitted \
			--tags test \
			run --cloud-sync-deployment --  \
			$(PWD)/bin/helper false

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
	{ lastLine = $$0 }' $(MAKEFILE_LIST) | sort
