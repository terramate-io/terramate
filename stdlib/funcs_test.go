// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package stdlib_test

import (
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/rs/zerolog"
	"github.com/terramate-io/terramate/event"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/stdlib"
	"github.com/terramate-io/terramate/test"
	"github.com/terramate-io/terramate/test/errors"
	"github.com/terramate-io/terramate/tf"
)

func TestTmVendor(t *testing.T) {
	t.Parallel()
	type testcase struct {
		name      string
		expr      string
		vendorDir string
		targetDir string
		want      string
		wantEvent event.VendorRequest
		wantErr   bool
	}

	src := func(source string) tf.Source {
		return test.ParseSource(t, source)
	}

	tcases := []testcase{
		{
			name:      "simple target dir",
			vendorDir: "/vendor",
			targetDir: "/dir",
			expr:      `tm_vendor("github.com/terramate-io/terramate?ref=main")`,
			want:      "../vendor/github.com/terramate-io/terramate/main",
			wantEvent: event.VendorRequest{
				Source:    src("github.com/terramate-io/terramate?ref=main"),
				VendorDir: project.NewPath("/vendor"),
			},
		},
		{
			name:      "nested target dir",
			vendorDir: "/modules",
			targetDir: "/dir/subdir/again",
			expr:      `tm_vendor("github.com/terramate-io/terramate?ref=v1")`,
			want:      "../../../modules/github.com/terramate-io/terramate/v1",
			wantEvent: event.VendorRequest{
				Source:    src("github.com/terramate-io/terramate?ref=v1"),
				VendorDir: project.NewPath("/modules"),
			},
		},
		{
			name:      "nested vendor dir",
			vendorDir: "/vendor/dir/nested",
			targetDir: "/dir",
			expr:      `tm_vendor("github.com/terramate-io/terramate?ref=main")`,
			want:      "../vendor/dir/nested/github.com/terramate-io/terramate/main",
			wantEvent: event.VendorRequest{
				Source:    src("github.com/terramate-io/terramate?ref=main"),
				VendorDir: project.NewPath("/vendor/dir/nested"),
			},
		},
		{
			name:      "target is on root",
			vendorDir: "/modules",
			targetDir: "/",
			expr:      `tm_vendor("github.com/terramate-io/terramate?ref=main")`,
			want:      "modules/github.com/terramate-io/terramate/main",
			wantEvent: event.VendorRequest{
				Source:    src("github.com/terramate-io/terramate?ref=main"),
				VendorDir: project.NewPath("/modules"),
			},
		},
		{
			name:      "vendor and target are on root",
			vendorDir: "/",
			targetDir: "/",
			expr:      `tm_vendor("github.com/terramate-io/terramate?ref=main")`,
			want:      "github.com/terramate-io/terramate/main",
			wantEvent: event.VendorRequest{
				Source:    src("github.com/terramate-io/terramate?ref=main"),
				VendorDir: project.NewPath("/"),
			},
		},
		{
			name:      "fails on invalid module src",
			vendorDir: "/modules",
			targetDir: "/dir",
			expr:      `tm_vendor("not a valid module src")`,
			wantErr:   true,
		},
		{
			name:      "fails on parameter missing",
			vendorDir: "/modules",
			targetDir: "/dir",
			expr:      `tm_vendor()`,
			wantErr:   true,
		},
		{
			name:      "fails on parameter with wrong type",
			vendorDir: "/modules",
			targetDir: "/dir",
			expr:      `tm_vendor([])`,
			wantErr:   true,
		},
		{
			name:      "fails on extra parameter",
			vendorDir: "/modules",
			targetDir: "/dir",
			expr:      `tm_vendor("github.com/terramate-io/terramate?ref=main", "")`,
			wantErr:   true,
		},
	}

	for _, tcase := range tcases {
		tcase := tcase
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()
			rootdir := test.TempDir(t)
			events := make(chan event.VendorRequest)
			vendordir := project.NewPath(tcase.vendorDir)
			targetdir := project.NewPath(tcase.targetDir)

			funcs := stdlib.Functions(rootdir)
			funcs[stdlib.Name("vendor")] = stdlib.VendorFunc(targetdir, vendordir, events)
			ctx := eval.NewContext(funcs)

			gotEvents := []event.VendorRequest{}
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

			// piggyback on the current tests to validate that
			// it also works with a nil channel (no interest on events).
			t.Run("works with nil events channel", func(t *testing.T) {
				funcs := stdlib.Functions(rootdir)
				funcs["tm_vendor"] = stdlib.VendorFunc(targetdir, vendordir, nil)
				ctx := eval.NewContext(funcs)

				val, err := ctx.Eval(test.NewExpr(t, tcase.expr))
				assert.NoError(t, err)
				assert.EqualStrings(t, tcase.want, val.AsString())
			})
		})
	}
}

func TestStdlibTmVersionMatch(t *testing.T) {
	t.Parallel()

	type testcase struct {
		expr    string
		wantErr error
		want    bool
	}

	for _, tc := range []testcase{
		{
			expr: `tm_version_match("1.0.0", "1.0.0")`,
			want: true,
		},
		{
			expr: `tm_version_match("1.0.0", "1.0.0", {allow_prereleases = true})`,
			want: true,
		},
		{
			expr: `tm_version_match("1.0.0", "= 1.0.0")`,
			want: true,
		},
		{
			expr: `tm_version_match("1.0.0", "= 1.0.0", {allow_prereleases = true})`,
			want: true,
		},
		{
			expr: `tm_version_match("1.0.0", ">= 1.0.0")`,
			want: true,
		},
		{
			expr: `tm_version_match("1.0.0", ">= 1.0.0", {allow_prereleases = true})`,
			want: true,
		},
		{
			expr: `tm_version_match("1.0.0", "<= 1.0.0")`,
			want: true,
		},
		{
			expr: `tm_version_match("1.0.0", "<= 1.0.0", {allow_prereleases = true})`,
			want: true,
		},
		{
			expr: `tm_version_match("1.0.0", "< 1.0.0")`,
			want: false,
		},
		{
			expr: `tm_version_match("1.0.0", "< 1.0.0", {allow_prereleases = true})`,
			want: false,
		},
		{
			expr: `tm_version_match("1.0.0", "> 1.0.0")`,
			want: false,
		},
		{
			expr: `tm_version_match("1.0.0", "> 1.0.0", {allow_prereleases = true})`,
			want: false,
		},
		{
			expr: `tm_version_match("1.0.0", "!= 1.0.0")`,
			want: false,
		},
		{
			expr: `tm_version_match("1.0.0", "!= 1.0.0", {allow_prereleases = true})`,
			want: false,
		},
		{
			expr: `tm_version_match("2.0.0", "> 1")`,
			want: true,
		},
		{
			expr: `tm_version_match("2.0.0", "> 1", {allow_prereleases = true})`,
			want: true,
		},
		{
			expr: `tm_version_match("2", "> 1")`,
			want: true,
		},
		{
			expr: `tm_version_match("2.0.0-dev", "> 1")`,
			want: false,
		},
		{
			expr: `tm_version_match("2.0.0-dev", "> 1", {allow_prereleases = true})`,
			want: true,
		},
		{
			expr: `tm_version_match("2.0.0-dev", "~> 1")`,
			want: false,
		},
		{
			expr: `tm_version_match("2.0.0-dev", "~> 1", {allow_prereleases = true})`,
			want: true,
		},
		{
			expr: `tm_version_match("2.0.0-dev", "~> 1.0", {allow_prereleases = true})`,
			// 2.0.0-dev < 2.0.0
			want: true,
		},
		{
			expr: `tm_version_match("2.0.0-dev", "~> 1.0", {allow_prereleases = false})`,
			want: false,
		},
		{
			expr: `tm_version_match("2.0.1-dev", "~> 1.0", {allow_prereleases = true})`,
			want: false,
		},
	} {
		tc := tc
		t.Run(tc.expr, func(t *testing.T) {
			rootdir := test.TempDir(t)
			ctx := eval.NewContext(stdlib.Functions(rootdir))
			val, err := ctx.Eval(test.NewExpr(t, tc.expr))
			errors.Assert(t, err, tc.wantErr)
			if err != nil {
				return
			}
			gotBool := val.True()
			assert.IsTrue(t, gotBool == tc.want)
		})
	}
}

func TestStdlibNewFunctionsMustPanicIfRelativeBaseDir(t *testing.T) {
	defer func() {
		err := recover()
		if err == nil {
			t.Fatal("eval.NewContext() did not panic with relative basedir")
		}
	}()
	_ = stdlib.Functions("relative")
}

func TestStdlibNewFunctionsMustPanicIfBasedirIsNonExistent(t *testing.T) {
	defer func() {
		err := recover()
		if err == nil {
			t.Fatal("eval.NewContext() did not panic with non existent basedir")
		}
	}()

	stdlib.Functions(filepath.Join(test.TempDir(t), "non-existent"))
}

func TestStdlibNewFunctionsFailIfBasedirIsNotADirectory(t *testing.T) {
	defer func() {
		err := recover()
		if err == nil {
			t.Fatal("eval.NewContext() did not panic if basedir is not a dir")
		}
	}()

	path := test.WriteFile(t, test.TempDir(t), "somefile.txt", ``)
	_ = stdlib.Functions(path)
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
