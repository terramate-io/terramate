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

package cli_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestBackendConfigOnLeafSingleStack(t *testing.T) {
	const (
		backendLabel   = "sometype"
		backendAttr    = "attr"
		backendAttrVal = "value"
	)

	s := sandbox.New(t)
	stack := s.CreateStack("stack")

	backendBlock := fmt.Sprintf(`backend "%s" {
    %s = "%s"
  }`, backendLabel, backendAttr, backendAttrVal)

	stack.CreateConfig(`terramate {
  %s
  %s
}`, versionAttribute(), backendBlock)

	ts := newCLI(t, s.BaseDir())

	assertRunResult(t, ts.run("generate"), runResult{IgnoreStdout: true})

	got := stack.ReadGeneratedTf()

	if !strings.HasPrefix(string(got), terramate.GeneratedCodeHeader) {
		t.Fatal("generated code missing header")
	}

	parser := hcl.NewParser()
	body, err := parser.ParseBody(got, terramate.GeneratedTfFilename)

	assert.NoError(t, err, "can't parse:\n%s", string(got))
	assert.EqualInts(t, 1, len(body.Blocks), "wrong amount of blocks on root on:\n%s", string(got))
	assert.EqualInts(t, 1, len(body.Blocks[0].Body.Blocks), "wrong amount of blocks inside terraform on:\n%s", string(got))

	parsedBackend := body.Blocks[0].Body.Blocks[0]

	assert.EqualStrings(t, "backend", parsedBackend.Type)
	assert.EqualInts(t, 1, len(parsedBackend.Labels), "wrong amount of labels on:\n%s", string(got))
	assert.EqualStrings(
		t,
		backendLabel,
		parsedBackend.Labels[0],
		"wrong backend block label on:\n%s",
		string(got),
	)
	// TODO: check attributes
	//assert.EqualInts(t, 1, len(parsedBackend.Body.Attributes), "wrong amount of attrs")
	//assert.EqualStrings(
	//t,
	//backendAttrVal,
	//tfEvalString(t, parsedBackend.Body.Attributes[backendAttr]),
	//"wrong attr value on:\n%s",
	//string(got),
	//)
}

func tfEvalString(t *testing.T, attr *hclsyntax.Attribute) string {
	t.Helper()

	val, diag := attr.Expr.Value(nil)
	if diag.HasErrors() {
		t.Fatal(diag.Error())
	}

	return val.AsString()
}

func versionAttribute() string {
	return fmt.Sprintf("required_version = %q", terramate.DefaultVersionConstraint())
}
