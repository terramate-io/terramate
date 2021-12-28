// Copyright 2021 Mineiros GmbH
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

package eval

import (
	"fmt"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/gocty"

	hhcl "github.com/hashicorp/hcl/v2"
	tflang "github.com/hashicorp/terraform/lang"
)

// Context is used to evaluate HCL code.
type Context struct {
	hclctx *hhcl.EvalContext
}

// NewContext creates a new HCL evaluation context.
// basedir is the base directory used by any interpolation functions that
// accept filesystem paths as arguments.
func NewContext(basedir string) *Context {
	scope := &tflang.Scope{BaseDir: basedir}
	hclctx := &hhcl.EvalContext{
		Functions: scope.Functions(),
		Variables: map[string]cty.Value{},
	}
	return &Context{
		hclctx: hclctx,
	}
}

// SetNamespace will set the given values inside the given namespace on the
// evaluation context.
func (c *Context) SetNamespace(name string, vals map[string]cty.Value) error {
	obj, err := fromMapToObject(vals)
	if err != nil {
		return fmt.Errorf("setting namespace %q:%v", name, err)
	}
	c.hclctx.Variables[name] = obj
	return nil
}

// Eval will evaluate an expression given its context.
func (c *Context) Eval(expr hclsyntax.Expression) (cty.Value, error) {
	val, diag := expr.Value(c.hclctx)
	if diag.HasErrors() {
		return cty.NilVal, fmt.Errorf("evaluating expression: %v", diag)
	}
	return val, nil
}

func fromMapToObject(m map[string]cty.Value) (cty.Value, error) {
	ctyTypes := map[string]cty.Type{}
	for key, value := range m {
		ctyTypes[key] = value.Type()
	}
	ctyObject := cty.Object(ctyTypes)
	ctyVal, err := gocty.ToCtyValue(m, ctyObject)
	if err != nil {
		return cty.Value{}, err
	}
	return ctyVal, nil
}
