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

type outerOuter struct {
	outer

	MyCommand struct {
	} `cmd:"" help:"My override 2."`
}

func (r outerOuter) UnwrapFlagSpec() any {
	return &r.outer
}

func TestAsFlagSpec(t *testing.T) {
	t.Parallel()

	t.Run("with unwrap (1 level)", func(t *testing.T) {
		var spec any = &outer{
			inner: inner{
				Flag: true,
			},
		}

		asInner := AsFlagSpec[inner](spec)
		assert.IsTrue(t, asInner != nil)
		assert.IsTrue(t, asInner.Flag)
	})

	t.Run("with unwrap (2 level)", func(t *testing.T) {
		var spec any = &outerOuter{
			outer: outer{
				inner: inner{
					Flag: true,
				},
			},
		}

		asInner := AsFlagSpec[inner](spec)
		assert.IsTrue(t, asInner != nil)
		assert.IsTrue(t, asInner.Flag)

		asOuter := AsFlagSpec[outer](spec)
		assert.IsTrue(t, asOuter != nil)
		assert.IsTrue(t, asOuter.Flag)
	})

	t.Run("without unwrap", func(t *testing.T) {
		var spec any = &outerNoUnwrap{
			inner: inner{
				Flag: true,
			},
		}

		asInner := AsFlagSpec[inner](spec)
		assert.IsTrue(t, asInner == nil)
	})
}
