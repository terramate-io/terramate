// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package yaml_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/madlambda/spells/assert"
	"github.com/zclconf/go-cty/cty"

	"github.com/terramate-io/hcl/v2"
	"github.com/terramate-io/hcl/v2/hclsyntax"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/yaml"
)

func TestEncode(t *testing.T) {
	type testcase struct {
		name        string
		input       yaml.BundleInstance
		want        string
		wantErrKind errors.Kind
	}

	for _, tc := range []testcase{
		{
			name: "valid document without inputs",
			input: yaml.BundleInstance{
				Name:   yaml.Attr("the-repo"),
				Source: yaml.Attr[any]("/bundles/terramate.io/tf-github-repository/v1"),
			},
			want: `apiVersion: terramate.io/cli/v1
kind: BundleInstance
metadata:
  name: the-repo
spec:
  source: /bundles/terramate.io/tf-github-repository/v1
`,
		},
		{
			name: "valid document simple inputs",
			input: yaml.BundleInstance{
				Name:   yaml.Attr("the-repo"),
				Source: yaml.Attr[any]("/bundles/terramate.io/tf-github-repository/v1"),
				Inputs: yaml.Attr(yaml.Map[any]{
					{
						Key:   yaml.Attribute[string]{V: "key", HeadComment: "\n# Head"},
						Value: yaml.Attribute[any]{V: "value", LineComment: "# Line"},
					},
				}),
			},
			want: `apiVersion: terramate.io/cli/v1
kind: BundleInstance
metadata:
  name: the-repo
spec:
  source: /bundles/terramate.io/tf-github-repository/v1
  inputs:
    
    # Head
    key: value # Line
`,
		},
		{
			name: "valid document with complex inputs",
			input: yaml.BundleInstance{
				Name:   yaml.Attr("the-repo"),
				Source: yaml.Attr[any]("/bundles/terramate.io/tf-github-repository/v1"),
				Inputs: yaml.Attr(yaml.Map[any]{
					{
						Key:   yaml.Attr("description"),
						Value: yaml.Attr[any]("the desc"),
					},
					{
						Key:   yaml.Attribute[string]{V: "visibility", HeadComment: "# Head comment"},
						Value: yaml.Attribute[any]{V: "private", LineComment: "# Line comment"},
					},
					{
						Key:   yaml.Attr("import"),
						Value: yaml.Attr[any](true),
					},
					{
						Key: yaml.Attr("hclexpr"),
						Value: yaml.Attr[any](&hclsyntax.FunctionCallExpr{
							Name: "do_something",
							Args: []hclsyntax.Expression{
								&hclsyntax.TemplateExpr{
									Parts: []hclsyntax.Expression{
										&hclsyntax.LiteralValueExpr{Val: cty.StringVal("something")},
									},
								},
							},
						}),
					},
					{
						Key:   yaml.Attr("num"),
						Value: yaml.Attr[any](1),
					},
					{
						Key:   yaml.Attr("floaty"),
						Value: yaml.Attr[any](0.124),
					},
					{
						Key: yaml.Attr("a_list"),
						Value: yaml.Attr[any](yaml.Seq[any]{
							{Value: yaml.Attr[any]("one")},
							{Value: yaml.Attribute[any]{V: true, HeadComment: "# This is true"}},
							{Value: yaml.Attribute[any]{V: 5, LineComment: "# Cool story"}},
							{Value: yaml.Attr[any](6.6)},
							{Value: yaml.Attr[any](&hclsyntax.LiteralValueExpr{Val: cty.NumberIntVal(123)})},
						}),
					},
					{
						Key: yaml.Attr("a_map"),
						Value: yaml.Attr[any](yaml.Map[any]{
							{
								Key:   yaml.Attr("hello"),
								Value: yaml.Attr[any]("world"),
							},
							{
								Key: yaml.Attr("another"),
								Value: yaml.Attr[any](yaml.Seq[any]{
									{Value: yaml.Attr[any]("a")},
									{Value: yaml.Attr[any](2)},
								}),
							},
							{
								Key: yaml.Attr("more"),
								Value: yaml.Attr[any](yaml.Map[any]{
									{
										Key: yaml.Attr("and"),
										Value: yaml.Attr[any](&hclsyntax.ScopeTraversalExpr{
											Traversal: hcl.Traversal{
												hcl.TraverseRoot{Name: "globals"},
												hcl.TraverseAttr{Name: "help_me"},
											},
										}),
									},
									{
										Key:   yaml.Attr("or"),
										Value: yaml.Attr[any]("globals.help_me"),
									},
								}),
							},
						}),
					},
				}),
			},
			want: `apiVersion: terramate.io/cli/v1
kind: BundleInstance
metadata:
  name: the-repo
spec:
  source: /bundles/terramate.io/tf-github-repository/v1
  inputs:
    description: the desc
    # Head comment
    visibility: private # Line comment
    import: true
    hclexpr: !hcl do_something("something")
    num: 1
    floaty: 0.124
    a_list:
      - one
      # This is true
      - true
      - 5 # Cool story
      - 6.6
      - !hcl 123
    a_map:
      hello: world
      another:
        - a
        - 2
      more:
        and: !hcl globals.help_me
        or: globals.help_me
`,
		},
		{
			name: "valid document with environments",
			input: yaml.BundleInstance{
				Name:   yaml.Attr("my-service"),
				Source: yaml.Attr[any]("/bundles/terramate.io/tf-service/v1"),
				Inputs: yaml.Attr(yaml.Map[any]{
					{
						Key:   yaml.Attr("name"),
						Value: yaml.Attr[any]("my-service"),
					},
				}),
				Environments: yaml.Attr(yaml.Map[*yaml.BundleEnvironment]{
					{
						Key: yaml.Attr("dev"),
						Value: yaml.Attr(&yaml.BundleEnvironment{
							Inputs: yaml.Attr(yaml.Map[any]{
								{
									Key:   yaml.Attr("region"),
									Value: yaml.Attr[any]("us-east-1"),
								},
								{
									Key:   yaml.Attr("instance_count"),
									Value: yaml.Attr[any](1),
								},
							}),
						}),
					},
					{
						Key: yaml.Attr("prod"),
						Value: yaml.Attr(&yaml.BundleEnvironment{
							Inputs: yaml.Attr(yaml.Map[any]{
								{
									Key: yaml.Attr("region"),
									Value: yaml.Attr[any](&hclsyntax.ScopeTraversalExpr{
										Traversal: hcl.Traversal{
											hcl.TraverseRoot{Name: "global"},
											hcl.TraverseAttr{Name: "prod_region"},
										},
									}),
								},
								{
									Key:   yaml.Attribute[string]{V: "instance_count", HeadComment: "# Production needs more instances"},
									Value: yaml.Attr[any](5),
								},
							}),
						}),
					},
				}),
			},
			want: `apiVersion: terramate.io/cli/v1
kind: BundleInstance
metadata:
  name: my-service
spec:
  source: /bundles/terramate.io/tf-service/v1
  inputs:
    name: my-service
environments:
  dev:
    inputs:
      region: us-east-1
      instance_count: 1
  prod:
    inputs:
      region: !hcl global.prod_region
      # Production needs more instances
      instance_count: 5
`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var b strings.Builder
			err := yaml.Encode(&tc.input, &b)
			assert.NoError(t, err)
			got := b.String()

			if tc.wantErrKind == "" {
				assert.NoError(t, err)
				if diff := cmp.Diff(tc.want, got); diff != "" {
					t.Fatalf("diff:\n%s", diff)
				}
			} else {
				assert.IsTrue(t, errors.IsKind(err, tc.wantErrKind))
			}

		})
	}
}
