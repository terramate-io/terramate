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

package eval_test

import (
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/test"
)

func TestNewExtContextFailsOnNoRelPathFromBasedirToVendorDir(t *testing.T) {
	basedir := t.TempDir()
	vendordir := "vendor"

	_, err := eval.NewExtContext(basedir, vendordir, nil)
	assert.Error(t, err)
}

func TestContextDontHaveTmVendor(t *testing.T) {
}

func TestTmVendor(t *testing.T) {
	type testcase struct {
		name      string
		expr      string
		vendorDir string
		want      string
		events    []eval.TmVendorEvent
		wantErr   bool
	}

	tcases := []testcase{
		{
			name:      "valid github source",
			vendorDir: "vendor",
			expr:      `tm_vendor("github.com/mineiros-io/terramate?ref=main")`,
			want:      "../vendor/github.com/mineiros-io/terramate/main",
			events: []eval.TmVendorEvent{
				{
					Source: "github.com/mineiros-io/terramate?ref=main",
				},
			},
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			basedir := t.TempDir()
			vendordir := filepath.Join(basedir, tcase.vendorDir)
			events := make(chan eval.TmVendorEvent)

			ctx, err := eval.NewExtContext(basedir, vendordir, events)
			assert.NoError(t, err)

			gotEvents := []eval.TmVendorEvent{}
			done := make(chan struct{})
			go func() {
				for event := range events {
					gotEvents = append(gotEvents, event)
				}
				close(done)
			}()

			val, err := ctx.Eval(test.NewExpr(t, tcase.expr))
			if tcase.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.EqualStrings(t, tcase.want, val.AsString())

			close(events)
			<-done

			test.AssertDiff(t, gotEvents, tcase.events)
		})
	}
}
