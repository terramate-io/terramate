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

		if (want.Backend == nil) != (got.Backend == nil) {
			t.Fatalf("want.Backend[%+v] != got.Backend[%+v]",
				want.Backend, got.Backend)
		}

		if want.Backend != nil {
			assert.EqualStrings(t, want.Backend.Type, got.Backend.Type, "type differs")
			assert.EqualInts(t, len(want.Backend.Labels), len(got.Backend.Labels), "labels length")
			for i, wl := range want.Backend.Labels {
				assert.EqualStrings(t, wl, got.Backend.Labels[i], "label differ")
			}

			// TODO(i4k): compare the rest?
		}
	}
}
