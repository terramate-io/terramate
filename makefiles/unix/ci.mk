## test if terramate works with CI git environment.
.PHONY: test/ci
test/ci: build
	./bin/terramate list --changed
