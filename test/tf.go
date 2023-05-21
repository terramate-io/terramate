// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package test

import (
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/tf"
)

// ParseSource calls [tf.ParseSource] failing the test if it fails.
func ParseSource(t *testing.T, source string) tf.Source {
	t.Helper()

	modsrc, err := tf.ParseSource(source)
	assert.NoError(t, err)
	return modsrc
}
