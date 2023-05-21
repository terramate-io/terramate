// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package test

import (
	"os/exec"
	"testing"
)

// LookPath is identical to exec.LookPath except it will
// fail the test if the lookup fails
func LookPath(t *testing.T, file string) string {
	t.Helper()

	path, err := exec.LookPath(file)
	if err != nil {
		t.Fatalf("exec.LookPath(%q) = %v", file, err)
	}
	return path
}
