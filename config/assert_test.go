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

package config_test

import (
	"fmt"
	"testing"

	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/test"
	"github.com/zclconf/go-cty/cty"
)

func TestAssertConfigEval(t *testing.T) {
	type testcase struct {
		name       string
		assert     hcl.AssertConfig
		namespaces namespaces
		want       config.Assert
		wantErr    error
	}

	expr := func(s string) hhcl.Expression {
		return test.NewExpr(t, s)
	}

	tcases := []testcase{
		{
			name: "using literals",
			assert: hcl.AssertConfig{
				Assertion: expr(`"a" == "terramate"`),
				Message:   expr(`"something"`),
			},
			want: config.Assert{
				Assertion: false,
				Message:   "something",
			},
		},
		{
			name: "accessing namespace values",
			namespaces: namespaces{
				"ns": nsvalues{
					"a": "terramate",
				},
				"ns2": nsvalues{
					"msg": "message",
				},
			},
			assert: hcl.AssertConfig{
				Assertion: expr(`ns.a == "terramate"`),
				Message:   expr(`ns2.msg`),
			},
			want: config.Assert{
				Assertion: true,
				Message:   "message",
			},
		},
		{
			name: "warning defined",
			namespaces: namespaces{
				"warning": nsvalues{
					"val": true,
				},
			},
			assert: hcl.AssertConfig{
				Assertion: expr(`true`),
				Message:   expr(`"msg"`),
				Warning:   expr("warning.val"),
			},
			want: config.Assert{
				Assertion: true,
				Message:   "msg",
				Warning:   true,
			},
		},
		{
			name: "assertion undefined fails",
			assert: hcl.AssertConfig{
				Message: expr(`"something"`),
			},
			wantErr: errors.E(config.ErrSchema),
		},
		{
			name: "message undefined fails",
			assert: hcl.AssertConfig{
				Assertion: expr(`true`),
			},
			wantErr: errors.E(config.ErrSchema),
		},
		{
			name: "assertion is not boolean fails",
			assert: hcl.AssertConfig{
				Assertion: expr(`[]`),
				Message:   expr(`"something"`),
			},
			wantErr: errors.E(config.ErrSchema),
		},
		{
			name: "assertion eval fails",
			assert: hcl.AssertConfig{
				Assertion: expr(`unknown.access`),
				Message:   expr(`"something"`),
			},
			wantErr: errors.E(eval.ErrEval),
		},
		{
			name: "message is not string fails",
			assert: hcl.AssertConfig{
				Assertion: expr(`true`),
				Message:   expr(`false`),
			},
			wantErr: errors.E(config.ErrSchema),
		},
		{
			name: "message eval fails",
			assert: hcl.AssertConfig{
				Assertion: expr(`true`),
				Message:   expr(`access.unknown`),
			},
			wantErr: errors.E(eval.ErrEval),
		},
		{
			name: "warning is not boolean fails",
			assert: hcl.AssertConfig{
				Assertion: expr(`true`),
				Message:   expr(`"msg"`),
				Warning:   expr("[]"),
			},
			wantErr: errors.E(config.ErrSchema),
		},
		{
			name: "warning eval fails",
			assert: hcl.AssertConfig{
				Assertion: expr(`true`),
				Message:   expr(`"msg"`),
				Warning:   expr("access.warning"),
			},
			wantErr: errors.E(eval.ErrEval),
		},
		{
			name: "multiple errors",
			assert: hcl.AssertConfig{
				Assertion: expr(`unknown.val`),
				Message:   expr(`false`),
				Warning:   expr("access.warning"),
			},
			wantErr: errors.L(
				errors.E(eval.ErrEval),
				errors.E(config.ErrSchema),
				errors.E(eval.ErrEval),
			),
		},
		{
			name: "using funcalls",
			assert: hcl.AssertConfig{
				Assertion: expr(`"A" == tm_upper("a")`),
				Message:   expr(`tm_upper("func")`),
			},
			want: config.Assert{
				Assertion: true,
				Message:   "FUNC",
			},
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			hclctx, err := eval.NewContext(t.TempDir())
			assert.NoError(t, err)

			for k, v := range tcase.namespaces {
				hclctx.SetNamespace(k, v.asCtyMap())
			}

			got, err := config.EvalAssert(hclctx, tcase.assert)
			assert.IsError(t, err, tcase.wantErr)
			if tcase.want != got {
				t.Fatalf("got %#v != want %#v", got, tcase.want)
			}
		})
	}
}

type namespaces map[string]nsvalues

type nsvalues map[string]interface{}

func (e nsvalues) asCtyMap() map[string]cty.Value {
	vals := make(map[string]cty.Value)
	for k, v := range e {
		// For now we don't support a lot of types
		// Just the basics for some basic testing
		switch v := v.(type) {
		case string:
			vals[k] = cty.StringVal(v)
		case bool:
			vals[k] = cty.BoolVal(v)
		default:
			panic(fmt.Errorf("unsupported type %T", v))
		}
	}
	return vals
}
