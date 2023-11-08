// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package stack_test

import (
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/rs/zerolog"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestLoadAllFailsIfStacksIDIsNotUnique(t *testing.T) {
	t.Parallel()
	s := sandbox.NoGit(t, true)
	s.BuildTree([]string{
		"s:stacks/stack-1:id=terramate",
		"s:stacks/stack-2:id=TerraMate",
	})
	cfg, err := config.LoadTree(s.RootDir(), s.RootDir())
	assert.NoError(t, err)
	_, err = config.LoadAllStacks(cfg)
	assert.IsError(t, err, errors.E(config.ErrStackDuplicatedID))
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
