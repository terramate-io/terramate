// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package terramate

import (
	_ "embed"
	"strings"
)

//go:embed VERSION
var version string

// ErrVersion indicates failure when checking Terramate version.

// Version of terramate.
func Version() string {
	return strings.TrimSpace(version)
}
