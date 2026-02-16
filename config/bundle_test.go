// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package config_test

/*

func TestEvalBundle(t *testing.T) {
	t.Parallel()

	evalctx := eval.NewContext(map[string]function.Function{})

	inputsBlock := &ast.MergedBlock{
		Type: "inputs",
		Attributes: map[string]ast.Attribute{
			"test": {
				Attribute: &hhcl.Attribute{
					Name: "b",
					Expr: &hclsyntax.LiteralValueExpr{
						Val: cty.StringVal("value from inputs block"),
					},
				},
			},
		},
	}

	bundle := &prohcl.Bundle{
		Name: "my_bundle",
		Source: &ast.Attribute{
			Attribute: &hhcl.Attribute{
				Name: "source",
				Expr: &hclsyntax.LiteralValueExpr{
					Val: cty.StringVal("source"),
				},
			},
		},
		InputsAttr: &ast.Attribute{
			Attribute: &hhcl.Attribute{
				Name: "inputs",
				Expr: &hclsyntax.ObjectConsExpr{
					Items: []hclsyntax.ObjectConsItem{
						{
							KeyExpr: &hclsyntax.LiteralValueExpr{
								Val: cty.StringVal("a"),
							},
							ValueExpr: &hclsyntax.LiteralValueExpr{
								Val: cty.StringVal("test"),
							},
						},
						{
							KeyExpr: &hclsyntax.LiteralValueExpr{
								Val: cty.StringVal("b"),
							},
							ValueExpr: &hclsyntax.LiteralValueExpr{
								Val: cty.StringVal("test"),
							},
						},
					},
				},
			},
		},
		Inputs: inputsBlock,
	}

	evaluated, err := config.EvalBundle(evalctx, bundle)
	if err != nil {
		t.Fatalf("failed to evaluate bundle: %s", err)
	}

	expected := config.Bundle{
		Name:   "my_bundle",
		Source: "source",
		Inputs: map[string]cty.Value{
			"a": cty.StringVal("test"),
			"b": cty.StringVal("value from inputs block"),
		},
	}
	if diff := cmp.Diff(expected, evaluated, cmpopts.IgnoreFields(config.Bundle{}, "Info", "Inputs")); diff != "" {
		t.Fatalf("expected %v, got %v", expected, evaluated)
	}
}
*/
