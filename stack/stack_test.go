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
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/test/sandbox"
	"github.com/rs/zerolog"
)

func TestLoadAllFailsIfStacksIDIsNotUnique(t *testing.T) {
	s := sandbox.New(t)
	s.BuildTree([]string{
		"s:stacks/stack-1:id=id",
		"s:stacks/stack-2:id=id",
	})
	cfg, err := config.LoadTree(s.RootDir(), s.RootDir())
	assert.NoError(t, err)
	_, err = config.LoadAllStacks(cfg)
	assert.IsError(t, err, errors.E(config.ErrStackDuplicatedID))
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
