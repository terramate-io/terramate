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

package generate

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/madlambda/spells/errutil"
	"github.com/mineiros-io/terramate"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/generate/exportedtf"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/stack"
	"github.com/rs/zerolog/log"
)

const (
	ErrBackendConfigGen   errutil.Error = "generating backend config"
	ErrExportingLocalsGen errutil.Error = "generating locals"
	ErrLoadingGlobals     errutil.Error = "loading globals"
	ErrLoadingStackCfg    errutil.Error = "loading stack code gen config"
	ErrManualCodeExists   errutil.Error = "manually defined code found"
)

// Do will walk all the stacks inside the given working dir
// generating code for any stack it finds as it goes along.
//
// Code is generated based on configuration files spread around the entire
// project until it reaches the given root. So even though a configuration
// file may be outside the given working dir it may be used on code generation
// if it is in a dir that is a parent of a stack found inside the working dir.
//
// The provided root must be the project's root directory as an absolute path.
// The provided working dir must be an absolute path that is a child of the
// provided root (or the same as root, indicating that working dir is the project root).
//
// It will return an error if it finds any invalid Terramate configuration files
// or if it can't generate the files properly for some reason.
func Do(root string, workingDir string) error {
	errs := forEachStack(root, workingDir, func(
		stack stack.S,
		globals *terramate.Globals,
		cfg StackCfg,
	) error {
		stackpath := stack.AbsPath()
		logger := log.With().
			Str("action", "generate.Do()").
			Str("path", root).
			Str("stackpath", stackpath).
			Logger()

		logger.Debug().Msg("Generate stack backend config.")

		stackMeta := stack.Meta()

		targetBackendCfgFile := filepath.Join(stackpath, cfg.BackendCfgFilename)
		err := writeStackBackendConfig(root, stackpath, stackMeta, globals, targetBackendCfgFile)
		if err != nil {
			return err
		}

		logger.Debug().Msg("Generate stack locals.")

		targetLocalsFile := filepath.Join(stackpath, cfg.LocalsFilename)
		err = writeStackLocalsCode(root, stackpath, stackMeta, globals, targetLocalsFile)
		if err != nil {
			return err
		}

		logger.Debug().Msg("Generate stack terraform.")
		return writeStackTerraformCode(root, stackpath, stackMeta, globals, cfg)
	})

	// FIXME(katcipis): errutil.Chain produces a very hard to read string representation
	// for this case, we have a possibly big list of errors here, not an
	// actual chain (multiple levels of calls).
	// We do need the error wrapping for the error handling on tests (for now at least).
	if err := errutil.Chain(errs...); err != nil {
		return fmt.Errorf("failed to generate code: %w", err)
	}

	return nil
}

// CheckStack will verify if a given stack has outdated code and return a list
// of filenames that are outdated, ordered lexicographically.
// If the stack has an invalid configuration it will return an error.
//
// The provided root must be the project's root directory as an absolute path.
// The provided stack dir must be the stack dir relative to the project
// root, in the form of path/to/the/stack.
func CheckStack(root string, stack stack.S) ([]string, error) {
	logger := log.With().
		Str("action", "generate.CheckStack()").
		Str("path", root).
		Stringer("stack", stack).
		Logger()

	outdated := []string{}

	logger.Trace().Msg("Load stack code generation config.")

	cfg, err := LoadStackCfg(root, stack)
	if err != nil {
		return nil, fmt.Errorf("checking for outdated code: %v", err)
	}

	logger.Trace().Msg("Loading globals for stack.")

	globals, err := terramate.LoadStackGlobals(root, stack.Meta())
	if err != nil {
		return nil, fmt.Errorf("checking for outdated code: %v", err)
	}

	stackpath := stack.AbsPath()
	stackMeta := stack.Meta()

	outdatedBackendFiles, err := backendConfigOutdatedFiles(root, stackpath, stackMeta, globals, cfg)
	if err != nil {
		return nil, fmt.Errorf("checking for outdated backend config: %v", err)
	}
	outdated = append(outdated, outdatedBackendFiles...)

	outdatedLocalsFiles, err := exportedLocalsOutdatedFiles(root, stackpath, stackMeta, globals, cfg)
	if err != nil {
		return nil, fmt.Errorf("checking for outdated exported locals: %v", err)
	}
	outdated = append(outdated, outdatedLocalsFiles...)

	outdatedTerraformFiles, err := exportedTerraformOutdatedFiles(root, stackpath, stackMeta, globals, cfg)
	if err != nil {
		return nil, fmt.Errorf("checking for outdated exported terraform: %v", err)
	}
	outdated = append(outdated, outdatedTerraformFiles...)

	sort.Strings(outdated)

	return outdated, nil
}

func backendConfigOutdatedFiles(
	root, stackpath string,
	stackMeta stack.Metadata,
	globals *terramate.Globals,
	cfg StackCfg,
) ([]string, error) {
	logger := log.With().
		Str("action", "generate.backendConfigOutdatedFiles()").
		Str("root", root).
		Str("stackpath", stackpath).
		Logger()

	logger.Trace().Msg("Generating backend cfg code for stack.")

	genbackend, err := generateBackendCfgCode(root, stackpath, stackMeta, globals, stackpath)
	if err != nil {
		return nil, err
	}

	stackBackendCfgFile := filepath.Join(stackpath, cfg.BackendCfgFilename)
	currentbackend, err := loadGeneratedCode(stackBackendCfgFile)
	if err != nil {
		return nil, err
	}

	logger.Trace().Msg("Checking for outdated backend cfg code on stack.")

	if string(genbackend) != string(currentbackend) {
		logger.Trace().Msg("Detected outdated backend cfg.")
		return []string{cfg.BackendCfgFilename}, nil
	}

	logger.Trace().Msg("backend cfg is updated.")
	return nil, nil
}

func exportedTerraformOutdatedFiles(
	root, stackpath string,
	stackMeta stack.Metadata,
	globals *terramate.Globals,
	cfg StackCfg,
) ([]string, error) {
	logger := log.With().
		Str("action", "generate.exportedTerraformOutdatedFiles()").
		Str("root", root).
		Str("stackpath", stackpath).
		Logger()

	logger.Trace().Msg("Checking for outdated exported terraform code on stack.")

	loadedStackTf, err := exportedtf.Load(root, stackMeta, globals)
	if err != nil {
		return nil, err
	}

	logger.Trace().Msg("Loaded exported terraform code, checking")

	outdated := []string{}

	for name, tf := range loadedStackTf.ExportedCode() {
		exportedTfFilename := name
		targetpath := filepath.Join(stackpath, exportedTfFilename)
		logger := logger.With().
			Str("blockName", name).
			Str("targetpath", targetpath).
			Logger()

		logger.Trace().Msg("checking if code is updated.")

		tfcode := PrependHeader(tf.String())
		currentTfCode, err := loadGeneratedCode(targetpath)
		if err != nil {
			return nil, err
		}

		if tfcode != string(currentTfCode) {
			logger.Trace().Msg("Outdated code detected.")
			outdated = append(outdated, exportedTfFilename)
		}

	}

	return outdated, nil
}

func exportedLocalsOutdatedFiles(
	root, stackpath string,
	stackMeta stack.Metadata,
	globals *terramate.Globals,
	cfg StackCfg,
) ([]string, error) {
	logger := log.With().
		Str("action", "generate.exportedLocalsOutdatedFiles()").
		Str("root", root).
		Str("stackpath", stackpath).
		Logger()

	logger.Trace().Msg("Checking for outdated exported locals code on stack.")

	genlocals, err := generateStackLocalsCode(root, stackpath, stackMeta, globals)
	if err != nil {
		return nil, err
	}

	stackLocalsFile := filepath.Join(stackpath, cfg.LocalsFilename)
	currentlocals, err := loadGeneratedCode(stackLocalsFile)
	if err != nil {
		return nil, err
	}

	if string(genlocals) != string(currentlocals) {
		logger.Trace().Msg("Detected outdated exported locals.")
		return []string{cfg.LocalsFilename}, nil
	}

	logger.Trace().Msg("exported locals are updated.")

	return nil, nil
}

func writeStackTerraformCode(
	root string,
	stackpath string,
	meta stack.Metadata,
	globals *terramate.Globals,
	cfg StackCfg,
) error {
	logger := log.With().
		Str("action", "writeStackTerraformCode()").
		Str("root", root).
		Str("stackpath", stackpath).
		Logger()

	logger.Trace().Msg("generating terraform code.")

	loadedStackTf, err := exportedtf.Load(root, meta, globals)
	if err != nil {
		return err
	}

	logger.Trace().Msg("generated terraform code.")

	for name, tf := range loadedStackTf.ExportedCode() {
		targetpath := filepath.Join(stackpath, name)
		logger := logger.With().
			Str("blockName", name).
			Str("targetpath", targetpath).
			Logger()

		tfcode := tf.String()
		if tfcode == "" {
			logger.Debug().Msg("ignoring empty export_as_terraform block.")
			continue
		}

		logger.Debug().Msg("Stack has exported terraform, saving generated code.")

		tfcode = PrependHeader(tfcode)
		if err := writeGeneratedCode(targetpath, tfcode); err != nil {
			return fmt.Errorf("stack %q: writing code at %q: %v", stackpath, targetpath, err)
		}

		logger.Debug().Msg("Saved stack generated code.")
	}

	logger.Trace().Msg("all terraform code has been saved.")
	return nil
}

func writeStackLocalsCode(
	root string,
	stackpath string,
	stackMetadata stack.Metadata,
	globals *terramate.Globals,
	targetLocalsFile string,
) error {
	logger := log.With().
		Str("action", "writeStackLocalsCode()").
		Str("root", root).
		Str("stack", stackpath).
		Str("targetLocalsFile", targetLocalsFile).
		Logger()
	logger.Debug().Msg("Save stack locals.")

	stackLocalsCode, err := generateStackLocalsCode(root, stackpath, stackMetadata, globals)
	if err != nil {
		return fmt.Errorf("stack %q: %w", stackpath, errutil.Chain(ErrExportingLocalsGen, err))
	}

	if len(stackLocalsCode) == 0 {
		logger.Debug().Msg("Stack has no locals to be generated, nothing to do.")
		return nil
	}

	logger.Debug().Msg("Stack has locals, saving generated code.")

	if err := writeGeneratedCode(targetLocalsFile, stackLocalsCode); err != nil {
		err = errutil.Chain(ErrExportingLocalsGen, err)
		return fmt.Errorf(
			"stack %q: %w: saving code at %q",
			stackpath,
			err,
			targetLocalsFile,
		)
	}

	logger.Debug().Msg("Saved stack generated code.")
	return nil
}

func generateStackLocalsCode(
	rootdir string,
	stackpath string,
	metadata stack.Metadata,
	globals *terramate.Globals,
) (string, error) {
	logger := log.With().
		Str("action", "generateStackLocals()").
		Str("stack", stackpath).
		Logger()

	logger.Trace().Msg("Load stack exported locals.")

	stackLocals, err := terramate.LoadStackExportedLocals(rootdir, metadata, globals)
	if err != nil {
		return "", err
	}

	logger.Trace().Msg("Get stack attributes.")

	localsAttrs := stackLocals.Attributes()
	if len(localsAttrs) == 0 {
		return "", nil
	}

	logger.Trace().Msg("Sort attributes.")

	sortedAttrs := make([]string, 0, len(localsAttrs))
	for name := range localsAttrs {
		sortedAttrs = append(sortedAttrs, name)
	}
	// Avoid generating code with random attr order (map iteration is random)
	sort.Strings(sortedAttrs)

	logger.Trace().
		Msg("Append locals block to file.")
	gen := hclwrite.NewEmptyFile()
	body := gen.Body()
	localsBlock := body.AppendNewBlock("locals", nil)
	localsBody := localsBlock.Body()

	logger.Trace().
		Msg("Set attribute values.")
	for _, name := range sortedAttrs {
		localsBody.SetAttributeValue(name, localsAttrs[name])
	}

	tfcode := PrependHeader(string(gen.Bytes()))
	return tfcode, nil
}

func writeStackBackendConfig(
	root string,
	stackpath string,
	stackMetadata stack.Metadata,
	globals *terramate.Globals,
	targetBackendCfgFile string,
) error {
	logger := log.With().
		Str("action", "generateStackBackendConfig()").
		Str("stack", stackpath).
		Str("targetFile", targetBackendCfgFile).
		Logger()

	logger.Trace().Msg("Generating code.")
	tfcode, err := generateBackendCfgCode(root, stackpath, stackMetadata, globals, stackpath)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrBackendConfigGen, err)
	}

	if len(tfcode) == 0 {
		logger.Debug().Msg("Stack has no backend config to be generated, nothing to do.")
		return nil
	}

	logger.Debug().Msg("Stack has backend config, saving generated code.")

	if err := writeGeneratedCode(targetBackendCfgFile, tfcode); err != nil {
		return fmt.Errorf(
			"stack %q: %w: saving code at %q",
			stackpath,
			err,
			targetBackendCfgFile,
		)
	}

	return nil
}

func generateBackendCfgCode(
	root string,
	stackpath string,
	stackMetadata stack.Metadata,
	globals *terramate.Globals,
	configdir string,
) (string, error) {
	logger := log.With().
		Str("action", "loadStackBackendConfig()").
		Str("configDir", configdir).
		Logger()

	logger.Trace().
		Msg("Check if config dir outside of root dir.")

	if !strings.HasPrefix(configdir, root) {
		// check if we are outside of project's root, time to stop
		return "", nil
	}

	logger.Trace().
		Msg("Get config file path.")
	configfile := filepath.Join(configdir, config.Filename)

	logger = logger.With().
		Str("configFile", configfile).
		Logger()

	logger.Trace().
		Msg("Load stack backend config.")
	if _, err := os.Stat(configfile); err != nil {
		// FIXME(katcipis): use  os.IsNotExist(err) to handle errors properly.
		// Unknown stat errors will be ignored right now.
		return generateBackendCfgCode(root, stackpath, stackMetadata, globals, filepath.Dir(configdir))
	}

	logger.Debug().
		Msg("Read config file.")
	config, err := os.ReadFile(configfile)
	if err != nil {
		return "", fmt.Errorf("reading config: %v", err)
	}

	logger.Debug().
		Msg("Parse config file.")
	parsedConfig, err := hcl.Parse(configfile, config)
	if err != nil {
		return "", fmt.Errorf("parsing config: %w", err)
	}

	logger.Trace().
		Msg("Check if parsed is empty.")
	parsed := parsedConfig.Terramate
	if parsed == nil || parsed.Backend == nil {
		return generateBackendCfgCode(root, stackpath, stackMetadata, globals, filepath.Dir(configdir))
	}

	evalctx := eval.NewContext(stackpath)

	logger.Trace().Msg("Add stack metadata evaluation namespace.")

	err = evalctx.SetNamespace("terramate", stackMetadata.ToCtyMap())
	if err != nil {
		return "", fmt.Errorf("setting terramate namespace on eval context for stack %q: %v",
			stackpath, err)
	}

	logger.Trace().Msg("Add global evaluation namespace.")

	if err := evalctx.SetNamespace("global", globals.Attributes()); err != nil {
		return "", fmt.Errorf("setting global namespace on eval context for stack %q: %v",
			stackpath, err)
	}

	logger.Debug().
		Msg("Create new file and append parsed blocks.")
	gen := hclwrite.NewEmptyFile()
	rootBody := gen.Body()
	tfBlock := rootBody.AppendNewBlock("terraform", nil)
	tfBody := tfBlock.Body()
	backendBlock := tfBody.AppendNewBlock(parsed.Backend.Type, parsed.Backend.Labels)
	backendBody := backendBlock.Body()

	if err := hcl.CopyBody(backendBody, parsed.Backend.Body, evalctx); err != nil {
		return "", err
	}

	return PrependHeader(string(gen.Bytes())), nil
}

// PrependHeader will add a proper Terramate header indicating that code
// was generated by Terramate.
func PrependHeader(code string) string {
	return codeHeader + "\n\n" + code
}

const codeHeader = "// GENERATED BY TERRAMATE: DO NOT EDIT"

func writeGeneratedCode(target string, code string) error {
	logger := log.With().
		Str("action", "writeGeneratedCode()").
		Str("file", target).
		Logger()

	logger.Trace().Msg("Checking code can be written.")

	if err := checkFileCanBeOverwritten(target); err != nil {
		return err
	}

	logger.Trace().Msg("Writing code")
	return os.WriteFile(target, []byte(code), 0666)
}

func checkFileCanBeOverwritten(path string) error {
	_, err := loadGeneratedCode(path)
	return err
}

func loadGeneratedCode(path string) ([]byte, error) {
	logger := log.With().
		Str("action", "loadGeneratedCode()").
		Str("path", path).
		Logger()

	logger.Trace().Msg("Get file information.")

	_, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("loading code: can't stat %q: %v", path, err)
	}

	logger.Trace().Msg("Read file.")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("loading code, can't read %q: %v", path, err)
	}

	logger.Trace().Msg("Check if code has terramate header.")

	if !strings.HasPrefix(string(data), codeHeader) {
		return nil, fmt.Errorf("%w: at %q", ErrManualCodeExists, path)
	}

	return data, nil
}

type forEachStackCallback func(stack.S, *terramate.Globals, StackCfg) error

func forEachStack(root, workingDir string, callback forEachStackCallback) []error {
	logger := log.With().
		Str("action", "generate.forEachStack()").
		Str("root", root).
		Str("workingDir", workingDir).
		Logger()

	logger.Trace().Msg("List stacks.")

	stackEntries, err := terramate.ListStacks(root)
	if err != nil {
		return []error{err}
	}

	var errs []error

	for _, entry := range stackEntries {
		stack := entry.Stack

		logger := logger.With().
			Stringer("stack", stack).
			Logger()

		if !strings.HasPrefix(stack.AbsPath(), workingDir) {
			logger.Trace().Msg("discarding stack outside working dir")
			continue
		}

		logger.Trace().Msg("Load stack code generation config.")

		cfg, err := LoadStackCfg(root, stack)
		if err != nil {
			errs = append(errs, fmt.Errorf(
				"stack %q: %w: %v",
				stack.AbsPath(),
				ErrLoadingStackCfg,
				err))
			continue
		}

		logger.Trace().Msg("Load stack globals.")

		globals, err := terramate.LoadStackGlobals(root, stack.Meta())
		if err != nil {
			errs = append(errs, fmt.Errorf(
				"stack %q: %w: %v",
				stack.AbsPath(),
				ErrLoadingGlobals,
				err))
			continue
		}

		logger.Trace().Msg("Calling stack callback.")
		if err := callback(stack, globals, cfg); err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}
