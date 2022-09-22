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
			name: "accessing ctx values",
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
		default:
			panic(fmt.Errorf("unsupported type %T", v))
		}
	}
	return vals
}
