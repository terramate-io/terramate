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
