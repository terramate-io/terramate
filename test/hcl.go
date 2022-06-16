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
	"os"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/hcl/eval"
)

// AssertTerramateConfig checks if two given Terramate configs are equal.
func AssertTerramateConfig(t *testing.T, got, want hcl.Config) {
	t.Helper()

	assertTerramateBlock(t, got.Terramate, want.Terramate)
	assertStackBlock(t, got.Stack, want.Stack)
}

// AssertDiff will compare the two values and fail if they are not the same
// providing a comprehensive textual diff of the differences between them.
func AssertDiff(t *testing.T, got, want interface{}) {
	t.Helper()

	if diff := cmp.Diff(got, want); diff != "" {
		t.Fatalf("-(got) +(want):\n%s", diff)
	}
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

	assertTerramateConfigBlock(t, got.RootConfig, want.RootConfig)
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

	assertTerramateRunBlock(t, got.Run, want.Run)
}

func assertTerramateRunBlock(t *testing.T, got, want *hcl.RunConfig) {
	t.Helper()

	if (want == nil) != (got == nil) {
		t.Fatalf("want.Run[%+v] != got.Run[%+v]", want, got)
	}

	if want == nil {
		return
	}

	if (want.Env == nil) != (got.Env == nil) {
		t.Fatalf(
			"want.Run.Env[%+v] != got.Run.Env[%+v]",
			want.Env,
			got.Env,
		)
	}

	if want.Env == nil {
		return
	}

	// There is no easy way to compare hclsyntax.Attribute
	// (or hcl.Attribute, or hclsyntax.Expression, etc).
	// So we do this hack in an attempt of comparing the attributes
	// original expressions (no eval involved).

	gotHCL := hclFromAttributes(t, got.Env.Attributes)
	wantHCL := hclFromAttributes(t, want.Env.Attributes)

	AssertDiff(t, gotHCL, wantHCL)
}

func hclFromAttributes(t *testing.T, attrs hcl.Attributes) string {
	t.Helper()

	file := hclwrite.NewEmptyFile()
	body := file.Body()

	sort.Stable(attrs)

	filesRead := map[string][]byte{}
	readFile := func(filename string) []byte {
		t.Helper()

		if file, ok := filesRead[filename]; ok {
			return file
		}

		file, err := os.ReadFile(filename)
		assert.NoError(t, err, "reading origin file")

		filesRead[filename] = file
		return file
	}

	for _, attr := range attrs {
		tokens, err := eval.GetExpressionTokens(
			readFile(attr.Origin()),
			attr.Origin(),
			attr.Value().Expr,
		)
		assert.NoError(t, err)
		body.SetAttributeRaw(attr.Value().Name, tokens)
	}

	return string(file.Bytes())
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
