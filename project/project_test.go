// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package project_test

import (
	"testing"

	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/test"
)

func TestPrjAbsPathOnRoot(t *testing.T) {
	path := project.PrjAbsPath("/", "/file.hcl")
	test.AssertEqualPaths(t, path, project.NewPath("/file.hcl"))
}
