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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/hcl/ast"
	"github.com/mineiros-io/terramate/hcl/eval"
)

// ParseTerramateConfig parses the Terramate configuration found
// on the given dir, returning the parsed configuration.
func ParseTerramateConfig(t *testing.T, dir string) hcl.Config {
	t.Helper()

	parser, err := hcl.NewTerramateParser(dir, dir)
	assert.NoError(t, err)

	err = parser.AddDir(dir)
	assert.NoError(t, err)

	cfg, err := parser.ParseConfig()
	assert.NoError(t, err)

	return cfg
}

// AssertGenHCLEquals checks if got gen code equals want. Since go
// is generated by Terramate it will be stripped of its Terramate
// header before comparing with want.
func AssertGenHCLEquals(t *testing.T, got string, want string) {
	t.Helper()

	const trimmedChars = "\n "

	// Terramate header validation is done separately, here we check only code.
	// So headers are removed.
	got = removeTerramateHCLHeader(got)
	got = strings.Trim(got, trimmedChars)
	want = strings.Trim(want, trimmedChars)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Error("generated code doesn't match expectation")
		t.Errorf("want:\n%q", want)
		t.Errorf("got:\n%q", got)
		t.Fatalf("diff:\n%s", diff)
	}
}

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

	if (want == nil) != (got == nil) {
		t.Fatalf("want[%v] != got[%v]", want, got)
	}

	if want == nil {
		t.Fatalf("want[nil] but got[%+v]", got)
	}

	assert.EqualStrings(t, want.RequiredVersion, got.RequiredVersion,
		"required_version mismatch")

	if (want.Config == nil) != (got.Config == nil) {
		t.Fatalf("want.Config[%+v] != got.Config[%+v]",
			want.Config, got.Config)
	}

	assertTerramateConfig(t, got.Config, want.Config)

	if (want.Manifest == nil) != (got.Manifest == nil) {
		t.Fatalf("want.Manifest[%+v] != got.Manifest[%+v]",
			want.Manifest, got.Manifest)
	}

	assertTerramateManifest(t, got.Manifest, want.Manifest)
}

func assertTerramateManifest(t *testing.T, got, want *hcl.ManifestConfig) {
	t.Helper()

	if want == nil {
		return
	}
}

func assertTerramateConfig(t *testing.T, got, want *hcl.RootConfig) {
	t.Helper()

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

	assert.IsTrue(t, want.CheckGenCode == got.CheckGenCode,
		"want.Run.CheckGenCode %v != got.Run.CheckGenCode %v",
		want.CheckGenCode, got.CheckGenCode)

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

// hclFromAttributes ensures that we always build the same HCL document
// given an hcl.Attributes.
func hclFromAttributes(t *testing.T, attrs ast.Attributes) string {
	t.Helper()

	file := hclwrite.NewEmptyFile()
	body := file.Body()

	attrList := attrs.SortedList()

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

	for _, attr := range attrList {
		tokens, err := eval.GetExpressionTokens(
			readFile(attr.Origin),
			attr.Origin,
			attr.Expr,
		)
		assert.NoError(t, err)
		body.SetAttributeRaw(attr.Name, tokens)
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

// WriteRootConfig writes a basic terramate root config.
func WriteRootConfig(t *testing.T, rootdir string) {
	WriteFile(t, rootdir, "root.config.tm", `
terramate {
	config {

	}
}
			`)
}

func removeTerramateHCLHeader(code string) string {
	lines := []string{}

	for _, line := range strings.Split(code, "\n") {
		if strings.HasPrefix(line, "//") && strings.Contains(line, "TERRAMATE") {
			continue
		}
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}
