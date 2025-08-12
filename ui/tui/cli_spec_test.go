// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"testing"

	"github.com/madlambda/spells/assert"
)

type inner struct {
	Flag bool

	MyCommand struct {
	} `cmd:"" help:"My base."`
}

type outer struct {
	inner

	MyCommand struct {
	} `cmd:"" help:"My override."`
}

func (r outer) UnwrapFlagSpec() any {
	return &r.inner
}

type outerNoUnwrap struct {
	inner

	MyCommand struct {
	} `cmd:"" help:"My override."`
}

func TestAsFlagSpec(t *testing.T) {
	t.Parallel()

	t.Run("with unwrap", func(t *testing.T) {
		var spec any = &outer{
			inner: inner{
				Flag: true,
			},
		}

		target := AsFlagSpec[inner](spec)
		assert.IsTrue(t, target != nil)
		assert.IsTrue(t, target.Flag)
	})

	t.Run("without unwrap", func(t *testing.T) {
		var spec any = &outerNoUnwrap{
			inner: inner{
				Flag: true,
			},
		}

		target := AsFlagSpec[inner](spec)
		assert.IsTrue(t, target == nil)
	})
}
