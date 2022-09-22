## check go modules are tidy
.PHONY: mod/check
mod/check:
	@./hack/mod-check
