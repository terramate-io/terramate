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
