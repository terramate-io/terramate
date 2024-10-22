// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package config_test

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/hcl/info"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/stdlib"
	"github.com/terramate-io/terramate/test"
	errtest "github.com/terramate-io/terramate/test/errors"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
	"github.com/zclconf/go-cty/cty"
)

func TestEvalSharingBackendInput(t *testing.T) {
	type testcase struct {
		name         string
		config       fmt.Stringer
		globals      map[string]cty.Value
		outputs      map[string]cty.Value
		want         config.Input
		wantValue    cty.Value
		wantErr      error
		wantValueErr error
	}
	t.Parallel()
	falsy := false
	truthy := true
	for _, tc := range []testcase{
		{
			name: "invalid backend attribute",
			config: Input(
				Labels("var_name"),
				Expr("value", `outputs.var_name`),
				Str("from_stack_id", "other-stack"),
				Expr("backend", `1`),
			),
			outputs: map[string]cty.Value{
				"var_name": cty.StringVal("test"),
			},
			wantErr: errors.E(config.ErrSchema, "input.backend must be string, got number"),
		},
		{
			name: "empty from_stack_id attribute",
			config: Input(
				Labels("var_name"),
				Expr("value", `outputs.var_name`),
				Str("from_stack_id", ""),
				Str("backend", `my-backend`),
			),
			outputs: map[string]cty.Value{
				"var_name": cty.StringVal("test"),
			},
			wantErr: errors.E(`"input.from_stack_id" "" doesn't match "^[a-zA-Z0-9_-]{1,64}$"`),
		},
		{
			name: "invalid from_stack_id attribute",
			config: Input(
				Labels("var_name"),
				Expr("value", `outputs.var_name`),
				Str("from_stack_id", "id cannot contain spaces"),
				Str("backend", `my-backend`),
			),
			outputs: map[string]cty.Value{
				"var_name": cty.StringVal("test"),
			},
			wantErr: errors.E(`"input.from_stack_id" "id cannot contain spaces" doesn't match "^[a-zA-Z0-9_-]{1,64}$"`),
		},
		{
			name: "complete working input - sensitive=(unset)",
			globals: map[string]cty.Value{
				"my_backend":  cty.StringVal("my-backend"),
				"other_stack": cty.StringVal("other-stack"),
				"val":         cty.StringVal("from-global"),
			},
			config: Input(
				Labels("var_name"),
				Expr("value", `"${outputs.var_name}-${global.val}"`),
				Expr("from_stack_id", `global.other_stack`),
				Expr("backend", `global.my_backend`),
			),
			outputs: map[string]cty.Value{
				"var_name": cty.StringVal("test"),
			},
			want: config.Input{
				Name:        "var_name",
				FromStackID: "other-stack",
				Backend:     "my-backend",
			},
			wantValue: cty.StringVal("test-from-global"),
		},
		{
			name: "complete working input - sensitive=false",
			globals: map[string]cty.Value{
				"my_backend":  cty.StringVal("my-backend"),
				"other_stack": cty.StringVal("other-stack"),
				"val":         cty.StringVal("from-global"),
				"is_secret":   cty.BoolVal(false),
			},
			config: Input(
				Labels("var_name"),
				Expr("value", `"${outputs.var_name}-${global.val}"`),
				Expr("from_stack_id", `global.other_stack`),
				Expr("backend", `global.my_backend`),
				Expr("sensitive", `global.is_secret`),
			),
			outputs: map[string]cty.Value{
				"var_name": cty.StringVal("test"),
			},
			want: config.Input{
				Name:        "var_name",
				FromStackID: "other-stack",
				Backend:     "my-backend",
				Sensitive:   &falsy,
			},
			wantValue: cty.StringVal("test-from-global"),
		},
		{
			name: "complete working input - sensitive=true",
			globals: map[string]cty.Value{
				"my_backend":  cty.StringVal("my-backend"),
				"other_stack": cty.StringVal("other-stack"),
				"val":         cty.StringVal("from-global"),
				"is_secret":   cty.BoolVal(true),
			},
			config: Input(
				Labels("var_name"),
				Expr("value", `"${outputs.var_name}-${global.val}"`),
				Expr("from_stack_id", `global.other_stack`),
				Expr("backend", `global.my_backend`),
				Expr("sensitive", `global.is_secret`),
			),
			outputs: map[string]cty.Value{
				"var_name": cty.StringVal("test"),
			},
			want: config.Input{
				Name:        "var_name",
				FromStackID: "other-stack",
				Backend:     "my-backend",
				Sensitive:   &truthy,
			},
			wantValue: cty.StringVal("test-from-global"),
		},
	} {
		tcase := tc
		t.Run(tcase.name, func(t *testing.T) {
			tempdir := test.TempDir(t)
			test.AppendFile(t, tempdir, "stack.tm", Block("stack").String())
			test.AppendFile(t, tempdir, "input.tm", tcase.config.String())
			test.AppendFile(t, tempdir, "terramate.tm", Terramate(
				Config(
					Experiments(hcl.SharingIsCaringExperimentName),
				),
			).String())

			cfg, err := config.LoadRoot(tempdir)
			if errors.IsAnyKind(tcase.wantErr, hcl.ErrHCLSyntax, hcl.ErrTerramateSchema) {
				errtest.Assert(t, err, tcase.wantErr)
				return
			}

			assert.NoError(t, err)

			rootTree, ok := cfg.Lookup(project.NewPath("/"))
			if !ok {
				panic("root tree not found")
			}

			st, err := rootTree.Stack()
			assert.NoError(t, err)
			hclctx := eval.NewContext(stdlib.Functions(tempdir, []string{}))
			hclctx.SetNamespace("global", tcase.globals)
			runtime := cfg.Runtime()
			runtime.Merge(st.RuntimeValues(cfg))
			hclctx.SetNamespace("terramate", runtime)
			outputs := tcase.outputs
			if outputs == nil {
				outputs = make(map[string]cty.Value)
			}
			hclctx.SetNamespace("outputs", outputs)

			if len(rootTree.Node.Inputs) != 1 {
				panic("test expects one input")
			}
			got, err := config.EvalInput(hclctx, rootTree.Node.Inputs[0])
			errtest.Assert(t, err, tcase.wantErr)
			if err != nil {
				return
			}
			// ignoring info.Range comparisons for now
			if diff := cmp.Diff(tcase.want, got, cmpopts.IgnoreUnexported(info.Range{}, config.Input{})); diff != "" {
				t.Fatalf("unexpected result\n%s", diff)
			}
			gotval, err := got.Value(hclctx)
			errtest.Assert(t, err, tcase.wantValueErr)
			if err != nil {
				return
			}
			assert.EqualStrings(t, string(ast.TokensForValue(tcase.wantValue).Bytes()), string(ast.TokensForValue(gotval).Bytes()))
		})
	}
}

func TestEvalSharingBackendOutput(t *testing.T) {
	type testcase struct {
		name      string
		config    fmt.Stringer
		globals   map[string]cty.Value
		want      config.Output
		wantValue string
		wantErr   error
	}
	t.Parallel()
	falsy := false
	for _, tc := range []testcase{
		{
			name: "invalid backend attribute",
			config: Output(
				Labels("var_name"),
				Expr("value", `module.test.var_name`),
				Expr("backend", `1`),
			),
			wantErr: errors.E(config.ErrSchema, "output.backend must be string, got number"),
		},
		{
			name: "complete working output",
			globals: map[string]cty.Value{
				"my_backend":  cty.StringVal("my-backend"),
				"description": cty.StringVal("my output description"),
				"is_secret":   cty.BoolVal(false),
			},
			config: Output(
				Labels("var_name"),
				Expr("value", `module.test.var_name`),
				Expr("backend", `global.my_backend`),
				Expr("sensitive", `global.is_secret`),
				Expr("description", `global.description`),
			),
			want: config.Output{
				Name:        "var_name",
				Description: "my output description",
				Backend:     "my-backend",
				Sensitive:   &falsy,
			},
			wantValue: `module.test.var_name`,
		},
	} {
		tcase := tc
		t.Run(tcase.name, func(t *testing.T) {
			tempdir := test.TempDir(t)
			test.AppendFile(t, tempdir, "stack.tm", Block("stack").String())
			test.AppendFile(t, tempdir, "input.tm", tcase.config.String())
			test.AppendFile(t, tempdir, "terramate.tm", Terramate(
				Config(
					Experiments(hcl.SharingIsCaringExperimentName),
				),
			).String())

			cfg, err := config.LoadRoot(tempdir)
			if errors.IsAnyKind(tcase.wantErr, hcl.ErrHCLSyntax, hcl.ErrTerramateSchema) {
				errtest.Assert(t, err, tcase.wantErr)
				return
			}

			assert.NoError(t, err)

			rootTree, ok := cfg.Lookup(project.NewPath("/"))
			if !ok {
				panic("root tree not found")
			}

			st, err := rootTree.Stack()
			assert.NoError(t, err)
			hclctx := eval.NewContext(stdlib.Functions(tempdir, []string{}))
			hclctx.SetNamespace("global", tcase.globals)
			runtime := cfg.Runtime()
			runtime.Merge(st.RuntimeValues(cfg))
			hclctx.SetNamespace("terramate", runtime)

			if len(rootTree.Node.Outputs) != 1 {
				panic("test expects one output")
			}
			got, err := config.EvalOutput(hclctx, rootTree.Node.Outputs[0])
			errtest.Assert(t, err, tcase.wantErr)
			if err != nil {
				return
			}
			// ignoring info.Range comparisons for now
			if diff := cmp.Diff(tcase.want, got, cmpopts.IgnoreUnexported(info.Range{}, config.Output{}), cmpopts.IgnoreFields(config.Output{}, "Value")); diff != "" {
				t.Fatalf("unexpected result\n%s", diff)
			}
			assert.EqualStrings(t, tcase.wantValue, string(ast.TokensForExpression(got.Value).Bytes()))
		})
	}
}
