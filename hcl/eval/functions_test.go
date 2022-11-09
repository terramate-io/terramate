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
	"github.com/mineiros-io/terramate/project"
	"github.com/mineiros-io/terramate/test"
)

func TestContextDontHaveTmVendor(t *testing.T) {
	// TODO(KATCIPIS)
}

func TestTmVendor(t *testing.T) {
	type testcase struct {
		name      string
		expr      string
		vendorDir string
		targetDir string
		want      string
		wantEvent eval.TmVendorEvent
		wantErr   bool
	}

	tcases := []testcase{
		{
			name:      "simple target dir",
			vendorDir: "/vendor",
			targetDir: "/dir",
			expr:      `tm_vendor("github.com/mineiros-io/terramate?ref=main")`,
			want:      "../vendor/github.com/mineiros-io/terramate/main",
			wantEvent: eval.TmVendorEvent{
				Source: "github.com/mineiros-io/terramate?ref=main",
			},
		},
		{
			name:      "nested target dir",
			vendorDir: "/modules",
			targetDir: "/dir/subdir/again",
			expr:      `tm_vendor("github.com/mineiros-io/terramate?ref=v1")`,
			want:      "../../../modules/github.com/mineiros-io/terramate/v1",
			wantEvent: eval.TmVendorEvent{
				Source: "github.com/mineiros-io/terramate?ref=v1",
			},
		},
		{
			name:      "nested vendor dir",
			vendorDir: "/vendor/dir/nested",
			targetDir: "/dir",
			expr:      `tm_vendor("github.com/mineiros-io/terramate?ref=main")`,
			want:      "../vendor/dir/nested/github.com/mineiros-io/terramate/main",
			wantEvent: eval.TmVendorEvent{
				Source: "github.com/mineiros-io/terramate?ref=main",
			},
		},
		{
			name:      "target is on root",
			vendorDir: "/modules",
			targetDir: "/",
			expr:      `tm_vendor("github.com/mineiros-io/terramate?ref=main")`,
			want:      "modules/github.com/mineiros-io/terramate/main",
			wantEvent: eval.TmVendorEvent{
				Source: "github.com/mineiros-io/terramate?ref=main",
			},
		},
		{
			name:      "vendor and target are on root",
			vendorDir: "/",
			targetDir: "/",
			expr:      `tm_vendor("github.com/mineiros-io/terramate?ref=main")`,
			want:      "github.com/mineiros-io/terramate/main",
			wantEvent: eval.TmVendorEvent{
				Source: "github.com/mineiros-io/terramate?ref=main",
			},
		},
		{
			name:      "fails on invalid module src",
			vendorDir: "/modules",
			targetDir: "/dir",
			expr:      `tm_vendor("not a valid module src")`,
			wantErr:   true,
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			rootdir := t.TempDir()
			events := make(chan eval.TmVendorEvent)
			vendordir := project.NewPath(tcase.vendorDir)
			basedir := filepath.Join(rootdir, tcase.targetDir)

			test.MkdirAll(t, basedir)

			ctx, err := eval.NewExtContext(rootdir, basedir, vendordir, events)
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

			close(events)
			<-done

			if tcase.wantErr {
				assert.Error(t, err)
				assert.EqualInts(t, 0, len(gotEvents), "expected no events on error")
				return
			}

			assert.NoError(t, err)

			assert.EqualStrings(t, tcase.want, val.AsString())
			assert.EqualInts(t, 1, len(gotEvents), "expected single event")
			test.AssertDiff(t, gotEvents[0], tcase.wantEvent)
		})
	}
}
