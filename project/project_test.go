// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

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
