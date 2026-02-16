// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package yaml_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/madlambda/spells/assert"
	"github.com/zclconf/go-cty/cty"

	"github.com/terramate-io/hcl/v2"
	"github.com/terramate-io/hcl/v2/hclsyntax"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/yaml"
)

func TestDecode(t *testing.T) {
	type testcase struct {
		name    string
		input   string
		want    yaml.BundleInstance
		wantErr error
	}

	for _, tc := range []testcase{
		{
			name: "valid document without inputs",
			input: `apiVersion: terramate.io/cli/v1
kind: BundleInstance
metadata:
    name: the-repo
spec:
    source: /bundles/terramate.io/tf-github-repository/v1
`,
			want: yaml.BundleInstance{
				Name:   yaml.Attr("the-repo", 4, 11),
				Source: yaml.Attr[any]("/bundles/terramate.io/tf-github-repository/v1", 6, 13),
			},
		},
		{
			name: "valid document simple inputs",
			input: `apiVersion: terramate.io/cli/v1
kind: BundleInstance
metadata:
    name: the-repo
spec:
    source: /bundles/terramate.io/tf-github-repository/v1
    # The inputs
    inputs:
        # It works!
        key: value
`,
			want: yaml.BundleInstance{
				Name:   yaml.Attr("the-repo", 4, 11),
				Source: yaml.Attr[any]("/bundles/terramate.io/tf-github-repository/v1", 6, 13),
				Inputs: yaml.Attr(yaml.Map[any]{
					{
						Key:   yaml.Attr("key", 10, 9, "# It works!"),
						Value: yaml.Attr[any]("value", 10, 14),
					},
				}, 8, 5, "# The inputs"),
			},
		},
		{
			name: "valid document with complex inputs",
			input: `apiVersion: terramate.io/cli/v1
kind: BundleInstance
metadata:
    name: the-repo
    uuid: 19bdb9a2-46d8-4db7-8db5-4735b0582700
spec:
    source: /bundles/terramate.io/tf-github-repository/v1
    inputs:
        description: "the desc"
        # Head comment
        visibility: private # Line comment
        import: true
        hclexpr: !hcl do_something("something")
        num: 1
        floaty: 0.124
        null_value: null
        a_list:
            - one
            # This is true
            - true
            - 5 # Cool story
            - 6.6
            - !hcl 123
        a_map:
            hello: world
            another: ["a", 2]
            more:
                and: !hcl globals.help_me
                or: globals.help_me
`,
			want: yaml.BundleInstance{
				Name:   yaml.Attr("the-repo", 4, 11),
				UUID:   yaml.Attr("19bdb9a2-46d8-4db7-8db5-4735b0582700", 5, 11),
				Source: yaml.Attr[any]("/bundles/terramate.io/tf-github-repository/v1", 7, 13),
				Inputs: yaml.Attr(yaml.Map[any]{
					{
						Key:   yaml.Attr("description", 9, 9),
						Value: yaml.Attr[any]("the desc", 9, 22),
					},
					{
						Key:   yaml.Attr("visibility", 11, 9, "# Head comment"),
						Value: yaml.Attr[any]("private", 11, 21, "", "# Line comment"),
					},
					{
						Key:   yaml.Attr("import", 12, 9),
						Value: yaml.Attr[any](true, 12, 17),
					},
					{
						Key: yaml.Attr("hclexpr", 13, 9),
						Value: yaml.Attr[any](&hclsyntax.FunctionCallExpr{
							Name: "do_something",
							Args: []hclsyntax.Expression{
								&hclsyntax.TemplateExpr{
									Parts: []hclsyntax.Expression{
										&hclsyntax.LiteralValueExpr{Val: cty.StringVal("something")},
									},
								},
							},
						}, 13, 18),
					},
					{
						Key:   yaml.Attr("num", 14, 9),
						Value: yaml.Attr[any](1, 14, 14),
					},
					{
						Key:   yaml.Attr("floaty", 15, 9),
						Value: yaml.Attr[any](0.124, 15, 17),
					},
					{
						Key:   yaml.Attr("null_value", 16, 9),
						Value: yaml.Attr[any](nil, 16, 21),
					},
					{
						Key: yaml.Attr("a_list", 17, 9),
						Value: yaml.Attr[any](yaml.Seq[any]{
							{Value: yaml.Attr[any]("one", 18, 15)},
							{Value: yaml.Attr[any](true, 20, 15, "# This is true")},
							{Value: yaml.Attr[any](5, 21, 15, "", "# Cool story")},
							{Value: yaml.Attr[any](6.6, 22, 15)},
							{Value: yaml.Attr[any](&hclsyntax.LiteralValueExpr{Val: cty.NumberIntVal(123)}, 23, 15)},
						}, 18, 13),
					},
					{
						Key: yaml.Attr("a_map", 24, 9),
						Value: yaml.Attr[any](yaml.Map[any]{
							{
								Key:   yaml.Attr("hello", 25, 13),
								Value: yaml.Attr[any]("world", 25, 20),
							},
							{
								Key: yaml.Attr("another", 26, 13),
								Value: yaml.Attr[any](yaml.Seq[any]{
									{Value: yaml.Attr[any]("a", 26, 23)},
									{Value: yaml.Attr[any](2, 26, 28)},
								}, 26, 22),
							},
							{
								Key: yaml.Attr("more", 27, 13),
								Value: yaml.Attr[any](yaml.Map[any]{
									{
										Key: yaml.Attr("and", 28, 17),
										Value: yaml.Attr[any](&hclsyntax.ScopeTraversalExpr{
											Traversal: hcl.Traversal{
												hcl.TraverseRoot{Name: "globals"},
												hcl.TraverseAttr{Name: "help_me"},
											},
										}, 28, 22),
									},
									{
										Key:   yaml.Attr("or", 29, 17),
										Value: yaml.Attr[any]("globals.help_me", 29, 21),
									},
								}, 28, 17),
							},
						}, 25, 13),
					},
				}, 8, 5),
			},
		},
		{
			name: "valid bundle document with environments",
			input: `apiVersion: terramate.io/cli/v1
kind: BundleInstance
metadata:
    name: the-repo
spec:
    source: /bundles/terramate.io/tf-github-repository/v1
    inputs:
        # It works!
        key: value
    
environments:
    dev:
        # Dev inputs
        inputs:
            # Dev works!
            key: dev_value
`,
			want: yaml.BundleInstance{
				Name:   yaml.Attr("the-repo", 4, 11),
				Source: yaml.Attr[any]("/bundles/terramate.io/tf-github-repository/v1", 6, 13),
				Inputs: yaml.Attr(yaml.Map[any]{
					{
						Key:   yaml.Attr("key", 9, 9, "# It works!"),
						Value: yaml.Attr[any]("value", 9, 14),
					},
				}, 7, 5),
				Environments: yaml.Attr(yaml.Map[*yaml.BundleEnvironment]{
					{
						Key: yaml.Attr("dev", 12, 5),
						Value: yaml.Attr(&yaml.BundleEnvironment{
							Inputs: yaml.Attr(yaml.Map[any]{
								{
									Key:   yaml.Attr("key", 16, 13, "# Dev works!"),
									Value: yaml.Attr[any]("dev_value", 16, 18),
								},
							}, 14, 9, "# Dev inputs"),
						}, 12, 5),
					},
				}, 12, 5),
			},
		},
		{
			name: "invalid syntax",
			input: `apiVersion: terramate.io/cli/v1
kind? Bundle
`,
			wantErr: &yaml.Error{Line: 2},
		},
		{
			name: "invalid attribute type",
			input: `apiVersion: 123
kind: BundleInstance
metadata:
    name: the-repo
spec:
    source: src`,
			wantErr: &yaml.Error{Line: 1, Column: 13},
		},
		{
			name: "invalid version",
			input: `apiVersion: terramate.io/cli/v2
kind: BundleInstance
metadata:
    name: the-repo
spec:
    source: src`,
			wantErr: &yaml.Error{Line: 1, Column: 13},
		},
		{
			name: "invalid kind",
			input: `apiVersion: terramate.io/cli/v1
kind: BundleInstancex
metadata:
    name: the-repo
spec:
    source: src`,
			wantErr: &yaml.Error{Line: 2, Column: 7},
		},
		{
			name: "invalid name",
			input: `apiVersion: terramate.io/cli/v1
kind: BundleInstance
metadata:
    name: ""
spec:
    source: src`,
			wantErr: &yaml.Error{Line: 4, Column: 11},
		},
		/*{
					name: "missing source",
					input: `apiVersion: terramate.io/cli/v1
		kind: BundleInstance
		metadata:
		    name: hello
		spec: {}`,
					wantErrKind: yaml.ErrSchema,
				},'*/
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got yaml.BundleInstance
			err := yaml.Decode(strings.NewReader(tc.input), &got)
			if tc.wantErr == nil {
				assert.NoError(t, err)
				if diff := cmp.Diff(&tc.want, &got, cmpCtyValue, cmpTraverser, cmpopts.IgnoreTypes(hcl.Pos{}, hcl.Range{})); diff != "" {
					t.Fatalf("diff:\n%s", diff)
				}
			} else {
				switch tc.wantErr.(type) {
				case *yaml.Error:
					var gotYAMLErr yaml.Error
					assert.IsTrue(t, errors.As(err, &gotYAMLErr))

					opts := []cmp.Option{
						// We will not test the actual error wording here.
						// Just the line/column.
						cmpopts.IgnoreFields(yaml.Error{}, "Err"),
					}
					if diff := cmp.Diff(tc.wantErr, &gotYAMLErr, opts...); diff != "" {
						t.Fatalf("diff:\n%s", diff)
					}
				default:
					assert.EqualErrs(t, tc.wantErr, err)
				}
			}
		})
	}
}

var cmpCtyValue = cmp.Comparer(func(a, b cty.Value) bool {
	return a.Equals(b).True()
})

var cmpTraverser = cmp.Comparer(func(a, b hcl.Traverser) bool {
	aTyp := reflect.TypeOf(a)
	bTyp := reflect.TypeOf(b)
	if aTyp != bTyp {
		return false
	}
	getName := func(x hcl.Traverser) string {
		switch x := x.(type) {
		case hcl.TraverseRoot:
			return x.Name
		case hcl.TraverseAttr:
			return x.Name
		}
		return ""
	}
	return getName(a) == getName(b)
})
