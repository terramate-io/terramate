// Copyright 2021 Mineiros GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package test

import (
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/hcl"
)

func AssertTerramateConfig(t *testing.T, got, want hcl.Config) {
	t.Helper()

	assertTerramateBlock(t, got.Terramate, want.Terramate)
	assertStackBlock(t, got.Stack, want.Stack)
}

func assertTerramateBlock(t *testing.T, got, want *hcl.Terramate) {
	t.Helper()

	if want == got {
		// same pointer, or both nil
		return
	}

	if want == nil {
		t.Fatalf("want[nil] but got[%+v]", got)
	}

	assert.EqualStrings(t, want.RequiredVersion, got.RequiredVersion,
		"required_version mismatch")

	if (want.RootConfig == nil) != (got.RootConfig == nil) {
		t.Fatalf("want.RootConfig[%+v] != got.RootConfig[%+v]",
			want.RootConfig, got.RootConfig)
	}

	assertTerramateConfigBlock(t, want.RootConfig, got.RootConfig)
}

func assertTerramateConfigBlock(t *testing.T, got, want *hcl.RootConfig) {
	t.Helper()

	if (want == nil) != (got == nil) {
		t.Fatalf("want.Git[%+v] != got.Git[%+v]", want, got)
	}

	if want == nil {
		return
	}

	if (want.Git == nil) != (got.Git == nil) {
		t.Fatalf(
			"want.Git[%+v] != got.Git[%+v]",
			want.Git,
			got.Git,
		)
	}

	if want.Git != nil {
		if *want.Git != *got.Git {
			t.Fatalf("want.Git[%+v] != got.Git[%+v]", want.Git, got.Git)
		}
	}
}

func assertStackBlock(t *testing.T, got, want *hcl.Stack) {
	if (got == nil) != (want == nil) {
		t.Fatalf("want[%+v] != got[%+v]", want, got)
	}

	if want == nil {
		return
	}

	assert.EqualInts(t, len(got.After), len(want.After), "After length mismatch")

	for i, w := range want.After {
		assert.EqualStrings(t, w, got.After[i], "stack after mismatch")
	}
}
