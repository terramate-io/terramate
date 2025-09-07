//go:build tools

package tools

import (
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
	_ "github.com/google/addlicense"
	_ "github.com/madlambda/benchcheck/cmd/benchcheck"
	_ "golang.org/x/tools/cmd/goimports"
)