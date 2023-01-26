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

package stdlib

import (
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/ext/customdecode"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	tflang "github.com/hashicorp/terraform/lang"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/event"
	"github.com/mineiros-io/terramate/modvendor"
	"github.com/mineiros-io/terramate/project"
	"github.com/mineiros-io/terramate/tf"
	"github.com/rs/zerolog/log"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

// Functions returns all the Terramate default functions.
// The `basedir` must be an absolute path for an existent directory or it panics.
func Functions(basedir string) map[string]function.Function {
	if !filepath.IsAbs(basedir) {
		panic(errors.E(errors.ErrInternal, "context created with relative path: %q", basedir))
	}

	st, err := os.Stat(basedir)
	if err != nil {
		panic(errors.E(errors.ErrInternal, err, "failed to stat context basedir %q", basedir))
	}
	if !st.IsDir() {
		panic(errors.E(errors.ErrInternal, "context basedir (%s) must be a directory", basedir))
	}

	scope := &tflang.Scope{BaseDir: basedir}
	tffuncs := scope.Functions()

	tmfuncs := map[string]function.Function{}
	for name, function := range tffuncs {
		tmfuncs["tm_"+name] = function
	}

	// fix terraform broken abspath()
	tmfuncs["tm_abspath"] = AbspathFunc(basedir)

	// sane ternary
	tmfuncs["tm_ternary"] = TernaryFunc()
	return tmfuncs

}

// Name converts the function name into the exported Terramate name.
func Name(name string) string { return "tm_" + name }

// AbspathFunc returns the `tm_abspath()` hcl function.
func AbspathFunc(basedir string) function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "path",
				Type: cty.String,
			},
		},
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			path := args[0].AsString()
			var abspath string
			if filepath.IsAbs(path) {
				abspath = path
			} else {
				abspath = filepath.Join(basedir, path)
			}

			return cty.StringVal(filepath.Clean(abspath)), nil
		},
	})
}

// VendorFunc returns the `tm_vendor` function.
// The basedir defines what tm_vendor will use to define the relative paths
// of vendored dependencies.
// The vendordir defines where modules are vendored inside the project.
// The stream defines the event stream for tm_vendor, one event is produced
// per successful function call.
func VendorFunc(basedir, vendordir project.Path, stream chan<- event.VendorRequest) function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "modsrc",
				Type: cty.String,
			},
		},
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			// Param spec already enforce modsrc to be string.
			source := args[0].AsString()
			modsrc, err := tf.ParseSource(source)
			if err != nil {
				return cty.NilVal, errors.E(err, "tm_vendor: invalid module source")
			}
			targetPath := modvendor.TargetDir(vendordir, modsrc)
			result, err := filepath.Rel(basedir.String(), targetPath.String())
			if err != nil {
				panic(errors.E(
					errors.ErrInternal, err,
					"tm_vendor: target dir cant be relative to basedir"))
			}
			// Because Windows
			result = filepath.ToSlash(result)

			if stream != nil {
				logger := log.With().
					Str("action", "tm_vendor").
					Str("source", source).
					Logger()

				logger.Debug().Msg("calculated path with success, sending event")

				stream <- event.VendorRequest{
					Source:    modsrc,
					VendorDir: vendordir,
				}

				log.Debug().Msg("event sent")
			}

			return cty.StringVal(result), nil
		},
	})
}

// HCLExpressionFunc returns the tm_hcl_expression function.
// This function interprets the `expr` argument as a string and returns the
// parsed expression.
func HCLExpressionFunc() function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "expr",
				Type: cty.String,
			},
		},
		Type: func(args []cty.Value) (cty.Type, error) {
			return customdecode.ExpressionType, nil
		},
		Impl: func(args []cty.Value, _ cty.Type) (cty.Value, error) {
			return hclExpr(args[0])
		},
	})
}

func hclExpr(arg cty.Value) (cty.Value, error) {
	exprParsed, diags := hclsyntax.ParseExpression([]byte(arg.AsString()), "gen.hcl", hcl.InitialPos)
	if diags.HasErrors() {
		return cty.NilVal, errors.E(diags, "argument is not valid HCL expression")
	}
	return customdecode.ExpressionVal(exprParsed), nil
}
