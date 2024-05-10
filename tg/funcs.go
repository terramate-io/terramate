// The MIT License (MIT)
// Copyright (c) 2016 Gruntwork, LLC
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to
// deal in the Software without restriction, including without limitation the
// rights to use, copy, modify, merge, publish, distribute, sublicense, and/or
// sell copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
// FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS
// IN THE SOFTWARE.

package tg

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gruntwork-io/go-commons/errors"
	tgconfig "github.com/gruntwork-io/terragrunt/config"
	"github.com/gruntwork-io/terragrunt/util"
	"github.com/terramate-io/terramate/project"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

type tgFunction func(ctx *tgconfig.ParsingContext, rootdir string, mod *Module, args []string) (string, error)

// findInParentFoldersFunc implements the Terragrunt `find_in_parent_folders` function.
func findInParentFoldersFunc(pctx *tgconfig.ParsingContext, rootdir string, mod *Module) function.Function {
	return wrapStringSliceToStringAsFuncImpl(pctx, rootdir, mod, findInParentFoldersImpl)
}

// findInParentFoldersImpl searches for a file in the parent directories of the caller's scope,
// or in other words, the value provided in the `ctx.TerragruntOptions.TerragruntConfigPath`
// option.
// Note:
//
//	  This function was modified from Terragrunt repository to fit the needs of Terramate.
//	  Check the original version here: https://github.com/gruntwork-io/terragrunt/blob/b47b57ae0cd2c8644ca5625fceed0a2258b1a763/config/config_helpers.go#L388-L445
//	  The important changes are:
//	  	- The function signature was changed to accept a `rootdir`` and `*Module` as parameters.
//		- When a file is successfully found, the abspath of the file is appended to `mod.DependsOn`.
//		- The `ctx.TerragruntOptions.MaxFoldersToCheck` limitation is removed and now it walks
//	      upwards up until project root is reached.
//		- The code was simplified by using Terramate project.Path.
func findInParentFoldersImpl(
	ctx *tgconfig.ParsingContext,
	rootdir string,
	mod *Module,
	params []string,
) (abspath string, err error) {
	defer func() {
		if err == nil {
			// keep track of the dependency if found successfully
			mod.DependsOn = append(mod.DependsOn, project.PrjAbsPath(rootdir, abspath))
		}
	}()

	numParams := len(params)
	if numParams > 2 {
		return "", errors.WithStackTrace(tgconfig.WrongNumberOfParamsError{Func: "find_in_parent_folders", Expected: "0, 1, or 2", Actual: numParams})
	}

	fileToFindStr := tgconfig.DefaultTerragruntConfigPath
	var fallbackParam string

	if numParams > 0 {
		fileToFindStr = params[0]
	}
	if numParams > 1 {
		fallbackParam = params[1]
	}

	currentHostDir, err := filepath.Abs(filepath.Dir(ctx.TerragruntOptions.TerragruntConfigPath))
	if err != nil {
		return "", errors.WithStackTrace(err)
	}

	currentDir := project.PrjAbsPath(rootdir, currentHostDir)
	for {
		parentDir := currentDir.Dir()
		if parentDir == currentDir { // only happens when we are at the project root
			if fallbackParam != "" {
				return fallbackParam, nil
			}
			return "", errors.WithStackTrace(tgconfig.ParentFileNotFoundError{Path: ctx.TerragruntOptions.TerragruntConfigPath, File: fileToFindStr, Cause: "Traversed all the way to the root"})
		}

		var fileToFind string
		if numParams > 0 {
			fileToFind = parentDir.Join(fileToFindStr).HostPath(rootdir)
		} else {
			fileToFind = tgconfig.GetDefaultConfigPath(parentDir.HostPath(rootdir))
		}

		if util.FileExists(fileToFind) {
			return fileToFind, nil
		}

		currentDir = parentDir
	}
}

// readTerragruntConfigFunc implements the Terragrunt `read_terragrunt_config` function.
func readTerragruntConfigFunc(ctx *tgconfig.ParsingContext, rootdir string, mod *Module) function.Function {
	return function.New(&function.Spec{
		// Takes one required string param
		Params: []function.Parameter{
			{
				Type: cty.String,
			},
		},
		// And optional param that takes anything
		VarParam: &function.Parameter{Type: cty.DynamicPseudoType},
		// We don't know the return type until we parse the terragrunt config, so we use a dynamic type
		Type: function.StaticReturnType(cty.DynamicPseudoType),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			numParams := len(args)
			if numParams == 0 || numParams > 2 {
				return cty.NilVal, errors.WithStackTrace(tgconfig.WrongNumberOfParamsError{Func: "read_terragrunt_config", Expected: "1 or 2", Actual: numParams})
			}

			targetConfigArg := args[0]
			if targetConfigArg.Type() != cty.String {
				return cty.NilVal, errors.WithStackTrace(tgconfig.InvalidParameterTypeError{Expected: "string", Actual: targetConfigArg.Type().FriendlyName()})
			}

			var defaultVal *cty.Value
			if numParams == 2 {
				defaultVal = &args[1]
			}
			return readTerragruntConfigImpl(ctx, targetConfigArg.AsString(), defaultVal, rootdir, mod)
		},
	})
}

// readTerragruntConfigImpl reads a terragrunt config file and returns the parsed config as a cty.Value.
// Note:
// This function was modified from Terragrunt repository to fit the needs of Terramate.
// Check the original version here: https://github.com/gruntwork-io/terragrunt/blob/b47b57ae0cd2c8644ca5625fceed0a2258b1a763/config/config_helpers.go#L578-L612
// The important changes are:
//   - The read file is added to the `mod.DependsOn` slice.
func readTerragruntConfigImpl(ctx *tgconfig.ParsingContext, configPath string, defaultVal *cty.Value, rootdir string, mod *Module) (cty.Value, error) {
	targetConfig := getCleanedTargetConfigPath(configPath, ctx.TerragruntOptions.TerragruntConfigPath)
	targetConfigFileExists := util.FileExists(targetConfig)
	if !targetConfigFileExists && defaultVal == nil {
		return cty.NilVal, errors.WithStackTrace(tgconfig.TerragruntConfigNotFoundError{Path: targetConfig})
	} else if !targetConfigFileExists {
		return *defaultVal, nil
	}

	mod.DependsOn = append(mod.DependsOn, project.PrjAbsPath(rootdir, targetConfig))

	// We update the ctx of terragruntOptions to the config being read in.
	ctx = ctx.WithTerragruntOptions(ctx.TerragruntOptions.Clone(targetConfig))
	cfg, err := tgconfig.ParseConfigFile(ctx, targetConfig, nil)
	if err != nil {
		return cty.NilVal, err
	}

	return tgconfig.TerragruntConfigAsCty(cfg)
}

// readTFVarsFile reads a *.tfvars or *.tfvars.json file and returns the contents as a JSON encoded string
func readTFVarsFile(ctx *tgconfig.ParsingContext, rootdir string, mod *Module, args []string) (string, error) {
	if len(args) != 1 {
		return "", errors.WithStackTrace(tgconfig.WrongNumberOfParamsError{Func: "read_tfvars_file", Expected: "1", Actual: len(args)})
	}

	varFile := args[0]
	varFile, err := util.CanonicalPath(varFile, ctx.TerragruntOptions.WorkingDir)
	if err != nil {
		return "", errors.WithStackTrace(err)
	}

	if !util.FileExists(varFile) {
		return "", errors.WithStackTrace(tgconfig.TFVarFileNotFoundError{File: varFile})
	}

	mod.DependsOn = append(mod.DependsOn, project.PrjAbsPath(rootdir, varFile))

	fileContents, err := os.ReadFile(varFile)
	if err != nil {
		return "", errors.WithStackTrace(fmt.Errorf("could not read file %q: %w", varFile, err))
	}

	if strings.HasSuffix(varFile, "json") {
		var variables map[string]interface{}
		// just want to be sure that the file is valid json
		if err := json.Unmarshal(fileContents, &variables); err != nil {
			return "", errors.WithStackTrace(fmt.Errorf("could not unmarshal json body of tfvar file: %w", err))
		}
		return string(fileContents), nil
	}

	var variables map[string]interface{}
	if err := tgconfig.ParseAndDecodeVarFile(varFile, fileContents, &variables); err != nil {
		return "", err
	}

	data, err := json.Marshal(variables)
	if err != nil {
		return "", errors.WithStackTrace(fmt.Errorf("could not marshal json body of tfvar file: %w", err))
	}

	return string(data), nil
}

// getCleanedTargetConfigPath returns a cleaned path to the target config (the `terragrunt.hcl` or
// `terragrunt.hcl.json` file), handling relative paths correctly. This will automatically append
// `terragrunt.hcl` or `terragrunt.hcl.json` to the path if the target path is a directory.
func getCleanedTargetConfigPath(configPath string, workingPath string) string {
	cwd := filepath.Dir(workingPath)
	targetConfig := configPath
	if !filepath.IsAbs(targetConfig) {
		targetConfig = util.JoinPath(cwd, targetConfig)
	}
	if util.IsDir(targetConfig) {
		targetConfig = tgconfig.GetDefaultConfigPath(targetConfig)
	}
	return util.CleanPath(targetConfig)
}

// wrapStringSliceToStringAsFuncImpl wraps a tgFunction and converts it into a function.Function
// with a variadic parameter of type string.
// The returned function.Function has an implementation that converts the input arguments to a string slice,
// calls the wrapped tgFunction with the converted arguments, and returns the result as a string.
func wrapStringSliceToStringAsFuncImpl(
	ctx *tgconfig.ParsingContext,
	rootdir string,
	mod *Module,
	toWrap tgFunction,
) function.Function {
	return function.New(&function.Spec{
		VarParam: &function.Parameter{Type: cty.String},
		Type:     function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			params, err := ctySliceToStringSlice(args)
			if err != nil {
				return cty.StringVal(""), err
			}
			out, err := toWrap(ctx, rootdir, mod, params)
			if err != nil {
				return cty.StringVal(""), err
			}
			return cty.StringVal(out), nil
		},
	})
}

// ctySliceToStringSlice converts a slice of cty.Value to a slice of strings.
// If an element is not of type cty.String, it returns an error with the details of the invalid parameter type.
func ctySliceToStringSlice(args []cty.Value) ([]string, error) {
	var out []string
	for _, arg := range args {
		if arg.Type() != cty.String {
			return nil, errors.WithStackTrace(tgconfig.InvalidParameterTypeError{Expected: "string", Actual: arg.Type().FriendlyName()})
		}
		out = append(out, arg.AsString())
	}
	return out, nil
}
