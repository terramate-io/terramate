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

package cli

import (
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/test/sandbox"
	"github.com/rs/zerolog"
)

func TestLocalDefaultIsOutdated(t *testing.T) {
	// This is a regression test for some internal issues on local default
	// is outdated checking. Validating this behavior is important but
	// the way it is done here is non-ideal, it tests private things in
	// a very clumsy way. When we get better modeled git functions this
	// can be improved drastically.
	s := sandbox.New(t)

	stack := s.CreateStack("stack-1")

	git := s.Git()
	git.Add(".")
	git.Commit("all")

	// dance below makes makes local main branch behind origin/main by 1 commit.
	//   - a "temp" branch is created to record current commit.
	//   - go back to main and create 1 additional commit and push to origin/main.
	//   - switch to "temp" and delete "main" reference.
	//   - create "main" branch again based on temp.

	git.CheckoutNew("temp")
	git.Checkout("main")
	stack.CreateFile("tempfile", "any content")
	git.CommitAll("additional commit")
	git.Push("main")
	git.Checkout("temp")
	git.DeleteBranch("main")
	git.CheckoutNew("main")

	prj, foundRoot, err := lookupProject(s.RootDir())

	assert.NoError(t, err)
	if !foundRoot {
		t.Fatal("unable to find root")
	}

	assert.NoError(t, prj.setDefaults(&cliSpec{}))

	assert.IsError(t, prj.checkLocalDefaultIsUpdated(), errors.E(ErrOutdatedLocalRev))
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
