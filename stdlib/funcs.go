// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package stdlib

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	resyntax "regexp/syntax"

	"github.com/hashicorp/hcl/v2/ext/customdecode"
	tflang "github.com/hashicorp/terraform/lang"
	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/event"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/modvendor"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/tf"
	"github.com/terramate-io/terramate/versions"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

var regexCache map[string]*regexp.Regexp

func init() {
	regexCache = map[string]*regexp.Regexp{}
}

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

	// not supported functions
	delete(tffuncs, "sensitive")
	delete(tffuncs, "nonsensitive")

	tmfuncs := map[string]function.Function{}
	for name, function := range tffuncs {
		tmfuncs["tm_"+name] = function
	}

	// optimized regex
	tmfuncs["tm_regex"] = Regex()

	// fix terraform broken abspath()
	tmfuncs["tm_abspath"] = AbspathFunc(basedir)

	// sane ternary
	tmfuncs["tm_ternary"] = TernaryFunc()

	tmfuncs["tm_version_match"] = VersionMatch()
	return tmfuncs
}

// NoFS returns all Terramate functions but excluding fs-related
// functions.
func NoFS(basedir string) map[string]function.Function {
	funcs := Functions(basedir)
	fsFuncNames := []string{
		"tm_abspath",
		"tm_file",
		"tm_fileexists",
		"tm_fileset",
		"tm_filebase64",
		"tm_filebase64sha256",
		"tm_filebase64sha512",
		"tm_filemd5",
		"tm_filesha1",
		"tm_filesha256",
		"tm_filesha512",
		"tm_templatefile",
	}
	for _, name := range fsFuncNames {
		delete(funcs, name)
	}
	return funcs
}

// Regex is a copy of Terraform [stdlib.RegexFunc] but with cached compiled
// patterns.
func Regex() function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "pattern",
				Type: cty.String,
			},
			{
				Name: "string",
				Type: cty.String,
			},
		},
		Type: func(args []cty.Value) (cty.Type, error) {
			if !args[0].IsKnown() {
				// We can't predict our type without seeing the pattern
				return cty.DynamicPseudoType, nil
			}

			retTy, err := regexPatternResultType(args[0].AsString())
			if err != nil {
				err = function.NewArgError(0, err)
			}
			return retTy, err
		},
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			if retType == cty.DynamicPseudoType {
				return cty.DynamicVal, nil
			}

			re, ok := regexCache[args[0].AsString()]
			if !ok {
				panic("should be in the cache")
			}

			str := args[1].AsString()

			captureIdxs := re.FindStringSubmatchIndex(str)
			if captureIdxs == nil {
				return cty.NilVal, fmt.Errorf("pattern did not match any part of the given string")
			}

			return regexPatternResult(re, str, captureIdxs, retType), nil
		},
	})
}

// regexPatternResultType parses the given regular expression pattern and
// returns the structural type that would be returned to represent its
// capture groups.
//
// Returns an error if parsing fails or if the pattern uses a mixture of
// named and unnamed capture groups, which is not permitted.
func regexPatternResultType(pattern string) (cty.Type, error) {
	re, ok := regexCache[pattern]
	if !ok {
		var rawErr error
		re, rawErr = regexp.Compile(pattern)
		switch err := rawErr.(type) {
		case *resyntax.Error:
			return cty.NilType, fmt.Errorf("invalid regexp pattern: %s in %s", err.Code, err.Expr)
		case error:
			// Should never happen, since all regexp compile errors should
			// be resyntax.Error, but just in case...
			return cty.NilType, fmt.Errorf("error parsing pattern: %s", err)
		}

		regexCache[pattern] = re
	}

	allNames := re.SubexpNames()[1:]
	var names []string
	unnamed := 0
	for _, name := range allNames {
		if name == "" {
			unnamed++
		} else {
			if names == nil {
				names = make([]string, 0, len(allNames))
			}
			names = append(names, name)
		}
	}
	switch {
	case unnamed == 0 && len(names) == 0:
		// If there are no capture groups at all then we'll return just a
		// single string for the whole match.
		return cty.String, nil
	case unnamed > 0 && len(names) > 0:
		return cty.NilType, fmt.Errorf("invalid regexp pattern: cannot mix both named and unnamed capture groups")
	case unnamed > 0:
		// For unnamed captures, we return a tuple of them all in order.
		etys := make([]cty.Type, unnamed)
		for i := range etys {
			etys[i] = cty.String
		}
		return cty.Tuple(etys), nil
	default:
		// For named captures, we return an object using the capture names
		// as keys.
		atys := make(map[string]cty.Type, len(names))
		for _, name := range names {
			atys[name] = cty.String
		}
		return cty.Object(atys), nil
	}
}

func regexPatternResult(re *regexp.Regexp, str string, captureIdxs []int, retType cty.Type) cty.Value {
	switch {
	case retType == cty.String:
		start, end := captureIdxs[0], captureIdxs[1]
		return cty.StringVal(str[start:end])
	case retType.IsTupleType():
		captureIdxs = captureIdxs[2:] // index 0 is the whole pattern span, which we ignore by skipping one pair
		vals := make([]cty.Value, len(captureIdxs)/2)
		for i := range vals {
			start, end := captureIdxs[i*2], captureIdxs[i*2+1]
			if start < 0 || end < 0 {
				vals[i] = cty.NullVal(cty.String) // Did not match anything because containing group didn't match
				continue
			}
			vals[i] = cty.StringVal(str[start:end])
		}
		return cty.TupleVal(vals)
	case retType.IsObjectType():
		captureIdxs = captureIdxs[2:] // index 0 is the whole pattern span, which we ignore by skipping one pair
		vals := make(map[string]cty.Value, len(captureIdxs)/2)
		names := re.SubexpNames()[1:]
		for i, name := range names {
			start, end := captureIdxs[i*2], captureIdxs[i*2+1]
			if start < 0 || end < 0 {
				vals[name] = cty.NullVal(cty.String) // Did not match anything because containing group didn't match
				continue
			}
			vals[name] = cty.StringVal(str[start:end])
		}
		return cty.ObjectVal(vals)
	default:
		// Should never happen
		panic(fmt.Sprintf("invalid return type %#v", retType))
	}
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

// VersionMatch returns the `tm_version_match` function spec, which checks if
// the provided version matches the constraint.
// If the third argument is provided, then it uses the flags to customize the
// version matcher. At the moment, only the propery `allow_prereleases` is
// supported, which enables matchs against prereleases using default Semver
// ordering semantics.
func VersionMatch() function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "version",
				Type: cty.String,
			},
			{
				Name: "constraint",
				Type: cty.String,
			},
		},
		VarParam: &function.Parameter{
			Name: "optional_flags",
			Type: cty.Object(map[string]cty.Type{
				"allow_prereleases": cty.Bool,
			}),
		},
		Type: function.StaticReturnType(cty.Bool),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			version := args[0].AsString()
			constraint := args[1].AsString()

			if len(args) > 3 {
				return cty.NilVal, errors.E("invalid number of arguments")
			}

			var allowPrereleases bool
			if len(args) == 3 {
				v := args[2].GetAttr("allow_prereleases")
				allowPrereleases = v.True()
			}
			match, err := versions.Match(version, constraint, allowPrereleases)
			if err != nil {
				return cty.NilVal, err
			}
			return cty.BoolVal(match), nil
		},
	})
}

func hclExpr(arg cty.Value) (cty.Value, error) {
	exprParsed, err := ast.ParseExpression(arg.AsString(), "<tm_ternary>")
	if err != nil {
		return cty.NilVal, errors.E(err, "argument is not valid HCL expression")
	}
	return customdecode.ExpressionVal(exprParsed), nil
}
