package test

import (
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terrastack/hcl"
)

func AssertTerrastackBlock(t *testing.T, got, want hcl.Terrastack) {
	t.Helper()

	assert.EqualStrings(t, got.RequiredVersion, want.RequiredVersion)
	assert.EqualInts(t, len(got.After), len(want.After), "After length mismatch")
	assert.EqualInts(t, len(got.Before), len(want.Before), "Before length mismatch")

	for i, w := range want.After {
		assert.EqualStrings(t, w, got.After[i], "stack mismatch")
	}

	for i, w := range want.Before {
		assert.EqualStrings(t, w, got.Before[i], "stack mismatch")
	}
}
