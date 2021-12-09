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
	"strings"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestBackendConfigOnLeafSingleStack(t *testing.T) {
	s := sandbox.New(t)
	stack := s.CreateStack("stack")

	backendBlock := `backend "type" {
    param = "value"
  }`

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
	_, err := parser.ParseBody(got, terramate.GeneratedTfFilename)

	assert.NoError(t, err)
	// TODO: test parsed body
}

func versionAttribute() string {
	return "required_version " + terramate.DefaultVersionConstraint()
}
