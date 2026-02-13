// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package typeschema

import (
	"reflect"
	"testing"

	"github.com/madlambda/spells/assert"
	hhcl "github.com/terramate-io/hcl/v2"
	"github.com/terramate-io/hcl/v2/hclsyntax"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

// makeDefaultAttr creates an ast.Attribute with a default expression for testing.
func makeDefaultAttr(exprStr string) *ast.Attribute {
	expr, diags := hclsyntax.ParseExpression([]byte(exprStr), "test", hhcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		panic("failed to parse test expression: " + diags.Error())
	}
	return &ast.Attribute{
		Attribute: &hhcl.Attribute{
			Name: "default",
			Expr: expr,
		},
	}
}

func TestParse(t *testing.T) {
	t.Parallel()

	type testcase struct {
		in         string
		wantString string
		wantType   Type
	}

	tests := []testcase{
		// Primitives
		{
			in:         "string",
			wantString: "string",
			wantType:   &PrimitiveType{Name: "string"},
		},
		{
			in:         "bool",
			wantString: "bool",
			wantType:   &PrimitiveType{Name: "bool"},
		},
		{
			in:         "any",
			wantString: "any",
			wantType:   &AnyType{},
		},

		// Collections
		{
			in:         "list(string)",
			wantString: "list(string)",
			wantType: &ListType{
				ValueType: &PrimitiveType{Name: "string"},
			},
		},
		{
			in:         "map(number)",
			wantString: "map(number)",
			wantType: &MapType{
				ValueType: &PrimitiveType{Name: "number"},
			},
		},
		{
			in:         "set(string)",
			wantString: "set(string)",
			wantType: &SetType{
				ValueType: &PrimitiveType{Name: "string"},
			},
		},
		{
			in:         "set(number)",
			wantString: "set(number)",
			wantType: &SetType{
				ValueType: &PrimitiveType{Name: "number"},
			},
		},
		{
			in:         "tuple(string, bool)",
			wantString: "tuple(string, bool)",
			wantType: &TupleType{
				Elems: []Type{
					&PrimitiveType{Name: "string"},
					&PrimitiveType{Name: "bool"},
				},
			},
		},

		// Objects & References
		{
			in:         "object",
			wantString: "object",
			wantType:   &ObjectType{},
		},
		{
			in:         "MyType",
			wantString: "MyType",
			wantType:   &ReferenceType{Name: "MyType"},
		},
		{
			in:         "has(NonStrictType)",
			wantString: "has(NonStrictType)",
			wantType:   &NonStrictType{Inner: &ReferenceType{Name: "NonStrictType"}},
		},

		// Object Merging (+ operator)
		{
			in:         "A + B",
			wantString: "A + B",
			wantType: &MergedObjectType{
				Objects: []Type{
					&ReferenceType{Name: "A"},
					&ReferenceType{Name: "B"},
				},
			},
		},
		{
			in:         "A + B + object",
			wantString: "A + B + object",
			wantType: &MergedObjectType{
				Objects: []Type{
					&ReferenceType{Name: "A"},
					&ReferenceType{Name: "B"},
					&ObjectType{},
				},
			},
		},

		// Variants (any_of)
		{
			in:         "any_of(string, number)",
			wantString: "any_of(string, number)",
			wantType: &VariantType{
				Options: []Type{
					&PrimitiveType{Name: "string"},
					&PrimitiveType{Name: "number"},
				},
			},
		},

		// Nested / Precedence
		// Input: "any_of(A + B, C)"
		{
			in:         "any_of(A + B, C)",
			wantString: "any_of(A + B, C)",
			wantType: &VariantType{
				Options: []Type{
					&MergedObjectType{
						Objects: []Type{
							&ReferenceType{Name: "A"},
							&ReferenceType{Name: "B"},
						},
					},
					&ReferenceType{Name: "C"},
				},
			},
		},
		// Input: "any_of(A, B + C)"
		{
			in:         "any_of(A, B + C)",
			wantString: "any_of(A, B + C)",
			wantType: &VariantType{
				Options: []Type{
					&ReferenceType{Name: "A"},
					&MergedObjectType{
						Objects: []Type{
							&ReferenceType{Name: "B"},
							&ReferenceType{Name: "C"},
						},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()

			got, err := Parse(tc.in, nil)
			assert.NoError(t, err)
			assert.EqualStrings(t, tc.wantString, got.String())

			if !reflect.DeepEqual(got, tc.wantType) {
				t.Errorf("\nAST Type mismatch for %q:\nGot:  %#v\nWant: %#v", tc.in, got, tc.wantType)
			}
		})
	}
}

func TestParseTypeString_Errors(t *testing.T) {
	t.Parallel()

	inputs := []string{
		"list(",                // Missing closing paren
		"tuple(string,)",       // Trailing comma (if not supported)
		"list(string, number)", // List takes only 1 argument
		"set(",                 // Missing closing paren
		"set()",                // Missing argument
		"set(string, number)",  // Set takes only 1 argument
		"has(list(string))",    // has() can only be used on references/objects
		"has(string)",          // has() can only be used on references/objects
		"any_of(string)",       // Variant usually needs >1 option, or maybe just syntax check
		"any_of(",              // Missing arguments
		"string + A",           // + operator requires strict references, 'string' is primitive
		"map()",                // Missing argument
	}

	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			_, err := Parse(input, nil)
			if err == nil {
				t.Errorf("expected error for input %q, but got nil", input)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()

	schemas := NewSchemaNamespaces()

	schemas.Set("schema", []*Schema{
		{
			Name: "A",
			Type: &ObjectType{
				Attributes: []*ObjectTypeAttribute{
					{
						Name:     "a",
						Type:     &PrimitiveType{Name: "string"},
						Required: true,
					},
				},
			},
		},
		{
			Name: "B",
			Type: &ObjectType{
				Attributes: []*ObjectTypeAttribute{
					{
						Name:     "b",
						Type:     &PrimitiveType{Name: "number"},
						Required: true,
					},
				},
			},
		},
		{
			Name: "Nested",
			Type: &ObjectType{
				Attributes: []*ObjectTypeAttribute{
					{
						Name:     "child",
						Type:     &ReferenceType{Name: "schema.A"},
						Required: true,
					},
				},
			},
		},
		{
			Name: "WithOptional",
			Type: &ObjectType{
				Attributes: []*ObjectTypeAttribute{
					{
						Name:     "required_field",
						Type:     &PrimitiveType{Name: "string"},
						Required: true,
					},
					{
						Name:     "optional_field",
						Type:     &PrimitiveType{Name: "string"},
						Required: false,
					},
				},
			},
		},
		{
			Name: "AllOptional",
			Type: &ObjectType{
				Attributes: []*ObjectTypeAttribute{
					{
						Name:     "opt1",
						Type:     &PrimitiveType{Name: "string"},
						Required: false,
					},
					{
						Name:     "opt2",
						Type:     &PrimitiveType{Name: "number"},
						Required: false,
					},
				},
			},
		},
		{
			Name: "WithDefaults",
			Type: &ObjectType{
				Attributes: []*ObjectTypeAttribute{
					{
						Name:     "has_default",
						Type:     &PrimitiveType{Name: "string"},
						Default:  makeDefaultAttr(`"default_value"`),
						Required: false,
					},
					{
						Name:     "required_field",
						Type:     &PrimitiveType{Name: "string"},
						Required: true,
					},
				},
			},
		},
		{
			Name: "AllDefaults",
			Type: &ObjectType{
				Attributes: []*ObjectTypeAttribute{
					{
						Name:    "str_default",
						Type:    &PrimitiveType{Name: "string"},
						Default: makeDefaultAttr(`"hello"`),
					},
					{
						Name:    "num_default",
						Type:    &PrimitiveType{Name: "number"},
						Default: makeDefaultAttr(`42`),
					},
					{
						Name:    "bool_default",
						Type:    &PrimitiveType{Name: "bool"},
						Default: makeDefaultAttr(`true`),
					},
				},
			},
		},
	})

	type testcase struct {
		name         string
		schema       string
		input        cty.Value
		expectErr    bool
		expectOutput cty.Value
	}

	tests := []testcase{
		// Primitives
		{
			name:   "primitive string success",
			schema: "string",
			input:  cty.StringVal("hello"),
		},
		{
			name:      "primitive string failure",
			schema:    "string",
			input:     cty.NumberIntVal(42),
			expectErr: true,
		},

		// Collections
		{
			name:   "list success",
			schema: "list(string)",
			input:  cty.ListVal([]cty.Value{cty.StringVal("one"), cty.StringVal("two")}),
		},
		{
			name:      "list failure (wrong element type)",
			schema:    "list(string)",
			input:     cty.TupleVal([]cty.Value{cty.StringVal("one"), cty.NumberIntVal(2)}),
			expectErr: true,
		},

		// Sets
		{
			name:   "set success",
			schema: "set(string)",
			input:  cty.ListVal([]cty.Value{cty.StringVal("one"), cty.StringVal("two")}),
		},
		{
			name:   "set removes duplicates",
			schema: "set(string)",
			input:  cty.ListVal([]cty.Value{cty.StringVal("a"), cty.StringVal("b"), cty.StringVal("a"), cty.StringVal("c"), cty.StringVal("b")}),
			expectOutput: cty.TupleVal([]cty.Value{
				cty.StringVal("a"),
				cty.StringVal("b"),
				cty.StringVal("c"),
			}),
		},
		{
			name:   "set removes duplicates (numbers)",
			schema: "set(number)",
			input:  cty.ListVal([]cty.Value{cty.NumberIntVal(1), cty.NumberIntVal(2), cty.NumberIntVal(1), cty.NumberIntVal(3)}),
			expectOutput: cty.TupleVal([]cty.Value{
				cty.NumberIntVal(1),
				cty.NumberIntVal(2),
				cty.NumberIntVal(3),
			}),
		},
		{
			name:   "set empty",
			schema: "set(string)",
			input:  cty.ListValEmpty(cty.String),
		},
		{
			name:      "set failure (wrong element type)",
			schema:    "set(string)",
			input:     cty.TupleVal([]cty.Value{cty.StringVal("one"), cty.NumberIntVal(2)}),
			expectErr: true,
		},
		{
			name:   "tuple success",
			schema: "tuple(string, number)",
			input:  cty.TupleVal([]cty.Value{cty.StringVal("s"), cty.NumberIntVal(1)}),
		},

		// Strict Objects (A)
		{
			name:   "object strict success",
			schema: "schema.A",
			input: cty.ObjectVal(map[string]cty.Value{
				"a": cty.StringVal("val"),
			}),
		},
		{
			name:   "object strict fail (extra field)",
			schema: "schema.A",
			input: cty.ObjectVal(map[string]cty.Value{
				"a":     cty.StringVal("val"),
				"extra": cty.StringVal("fail"),
			}),
			expectErr: true,
		},
		{
			name:      "object strict fail (missing field)",
			schema:    "schema.A",
			input:     cty.ObjectVal(map[string]cty.Value{}),
			expectErr: true,
		},

		// Non-strict Objects has(A)
		{
			name:   "object non-strict success",
			schema: "has(schema.A)",
			input: cty.ObjectVal(map[string]cty.Value{
				"a":     cty.StringVal("val"),
				"extra": cty.StringVal("ignored"),
			}),
		},

		// Nested non-strict has(Nested)
		// Note: The logic for nested looseness depends on how your AST handles 'has' recursively
		{
			name:   "nested strict success",
			schema: "schema.Nested",
			input: cty.ObjectVal(map[string]cty.Value{
				"child": cty.ObjectVal(map[string]cty.Value{"a": cty.StringVal("ok")}),
			}),
		},

		// Merging with + operator
		{
			name:   "merge strict success",
			schema: "schema.A + schema.B",
			input: cty.ObjectVal(map[string]cty.Value{
				"a": cty.StringVal("ok"),
				"b": cty.NumberIntVal(1),
			}),
		},
		{
			name:   "merge strict fail (extra field)",
			schema: "schema.A + schema.B",
			input: cty.ObjectVal(map[string]cty.Value{
				"a":     cty.StringVal("ok"),
				"b":     cty.NumberIntVal(1),
				"extra": cty.BoolVal(true),
			}),
			expectErr: true,
		},

		// Non-strict Merging has(A + B)
		{
			name:   "merge non-strict success",
			schema: "has(schema.A + schema.B)",
			input: cty.ObjectVal(map[string]cty.Value{
				"a":     cty.StringVal("ok"),
				"b":     cty.NumberIntVal(1),
				"extra": cty.BoolVal(true),
			}),
		},

		// Variants any_of(...)
		{
			name:   "variant match string",
			schema: "any_of(string, number)",
			input:  cty.StringVal("s"),
		},
		{
			name:      "variant no match",
			schema:    "any_of(string, number)",
			input:     cty.BoolVal(true),
			expectErr: true,
		},

		// Required attribute tests
		{
			name:   "required and optional - both provided",
			schema: "schema.WithOptional",
			input: cty.ObjectVal(map[string]cty.Value{
				"required_field": cty.StringVal("val"),
				"optional_field": cty.StringVal("opt"),
			}),
		},
		{
			name:   "required and optional - only required provided",
			schema: "schema.WithOptional",
			input: cty.ObjectVal(map[string]cty.Value{
				"required_field": cty.StringVal("val"),
			}),
		},
		{
			name:   "required and optional - only optional provided (missing required)",
			schema: "schema.WithOptional",
			input: cty.ObjectVal(map[string]cty.Value{
				"optional_field": cty.StringVal("opt"),
			}),
			expectErr: true,
		},
		{
			name:      "required and optional - neither provided (missing required)",
			schema:    "schema.WithOptional",
			input:     cty.ObjectVal(map[string]cty.Value{}),
			expectErr: true,
		},
		{
			name:   "all optional - all provided",
			schema: "schema.AllOptional",
			input: cty.ObjectVal(map[string]cty.Value{
				"opt1": cty.StringVal("val1"),
				"opt2": cty.NumberIntVal(42),
			}),
		},
		{
			name:   "all optional - some provided",
			schema: "schema.AllOptional",
			input: cty.ObjectVal(map[string]cty.Value{
				"opt1": cty.StringVal("val1"),
			}),
		},
		{
			name:   "all optional - none provided",
			schema: "schema.AllOptional",
			input:  cty.ObjectVal(map[string]cty.Value{}),
		},

		// Default attribute tests
		{
			name:   "with defaults - all provided (overrides default)",
			schema: "schema.WithDefaults",
			input: cty.ObjectVal(map[string]cty.Value{
				"has_default":    cty.StringVal("provided_value"),
				"required_field": cty.StringVal("req"),
			}),
			expectOutput: cty.ObjectVal(map[string]cty.Value{
				"has_default":    cty.StringVal("provided_value"),
				"required_field": cty.StringVal("req"),
			}),
		},
		{
			name:   "with defaults - only required provided (uses default)",
			schema: "schema.WithDefaults",
			input: cty.ObjectVal(map[string]cty.Value{
				"required_field": cty.StringVal("req"),
			}),
			expectOutput: cty.ObjectVal(map[string]cty.Value{
				"has_default":    cty.StringVal("default_value"),
				"required_field": cty.StringVal("req"),
			}),
		},
		{
			name:      "with defaults - missing required (error even with defaults)",
			schema:    "schema.WithDefaults",
			input:     cty.ObjectVal(map[string]cty.Value{}),
			expectErr: true,
		},
		{
			name:   "all defaults - none provided (all use defaults)",
			schema: "schema.AllDefaults",
			input:  cty.ObjectVal(map[string]cty.Value{}),
			expectOutput: cty.ObjectVal(map[string]cty.Value{
				"str_default":  cty.StringVal("hello"),
				"num_default":  cty.NumberIntVal(42),
				"bool_default": cty.True,
			}),
		},
		{
			name:   "all defaults - some provided (mix of provided and defaults)",
			schema: "schema.AllDefaults",
			input: cty.ObjectVal(map[string]cty.Value{
				"str_default": cty.StringVal("custom"),
			}),
			expectOutput: cty.ObjectVal(map[string]cty.Value{
				"str_default":  cty.StringVal("custom"),
				"num_default":  cty.NumberIntVal(42),
				"bool_default": cty.True,
			}),
		},
		{
			name:   "all defaults - all provided (no defaults used)",
			schema: "schema.AllDefaults",
			input: cty.ObjectVal(map[string]cty.Value{
				"str_default":  cty.StringVal("custom"),
				"num_default":  cty.NumberIntVal(100),
				"bool_default": cty.False,
			}),
			expectOutput: cty.ObjectVal(map[string]cty.Value{
				"str_default":  cty.StringVal("custom"),
				"num_default":  cty.NumberIntVal(100),
				"bool_default": cty.False,
			}),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			typ, err := Parse(tc.schema, nil)
			assert.NoError(t, err, "Parse failed for schema: %s", tc.schema)

			evalctx := eval.NewContext(map[string]function.Function{})
			output, err := typ.Apply(tc.input, evalctx, schemas, true)

			if tc.expectErr {
				assert.Error(t, err, "Expected validation error for input %s against %s", tc.input, tc.schema)
			} else {
				assert.NoError(t, err, "Expected success for input %s against %s", tc.input, tc.schema)
			}

			if tc.expectOutput != cty.NilVal {
				if !output.RawEquals(tc.expectOutput) {
					t.Errorf("Output mismatch:\nGot:  %#v\nWant: %#v", output, tc.expectOutput)
				}
			}
		})
	}
}
