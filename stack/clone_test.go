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

package stack_test

import (
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/stack"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestStackClone(t *testing.T) {
	type testcase struct {
		name    string
		layout  []string
		src     string
		target  string
		wantErr error
	}

	testcases := []testcase{}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			s := sandbox.New(t)
			s.BuildTree(tc.layout)

			srcdir := filepath.Join(s.RootDir(), tc.src)
			targetdir := filepath.Join(s.RootDir(), tc.target)
			err := stack.Clone(s.RootDir(), targetdir, srcdir)
			assert.IsError(t, err, tc.wantErr)

			if tc.wantErr != nil {
				return
			}
			// Validate cloning process
		})
	}
}

func TestStackCloneSrcDirMustBeInsideRootdir(t *testing.T) {
	s := sandbox.New(t)
	srcdir := t.TempDir()
	targetdir := filepath.Join(s.RootDir(), "new-stack")
	err := stack.Clone(s.RootDir(), targetdir, srcdir)
	assert.IsError(t, err, errors.E(stack.ErrInvalidStackDir))
}
