// Copyright 2022 Mineiros GmbH
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

package generate_test

import (
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/generate"
	tmstack "github.com/mineiros-io/terramate/stack"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestStackGeneratedFilesListing(t *testing.T) {
	s := sandbox.New(t)

	stackEntry1 := s.CreateStack("stacks/stack-1")
	stackEntry2 := s.CreateStack("stacks/stack-2")

	stack1 := stackEntry1.Load()
	stack2 := stackEntry2.Load()

	assertStackGenFiles := func(stack tmstack.S, want []string) {
		t.Helper()

		gen, err := generate.ListStackGenFiles(s.RootDir(), stack)
		assert.NoError(t, err)
		assertEqualStringList(t, gen, want)
	}

	assertStackGenFiles(stack1, []string{})
	assertStackGenFiles(stack2, []string{})
}
