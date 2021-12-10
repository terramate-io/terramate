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

package terramate

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/madlambda/spells/errutil"
	"github.com/mineiros-io/terramate/hcl"
)

const (
	// GeneratedTfFilename is the name of the terramate generated tf file.
	GeneratedTfFilename = "_gen_terramate.tm.tf"

	// GeneratedCodeHeader is the header added on all generated files.
	GeneratedCodeHeader = "// GENERATED BY TERRAMATE: DO NOT EDIT"
)

// Generate will walk all the directories starting from basedir generating
// code for any stack it finds as it goes along
//
// It will return an error if it finds any invalid terramate configuration files
// of if it can't generate the files properly for some reason.
//
// The provided basedir must be an absolute path to a directory.
func Generate(basedir string) error {
	if !filepath.IsAbs(basedir) {
		return fmt.Errorf("Generate(%q): basedir must be an absolute path", basedir)
	}

	info, err := os.Lstat(basedir)
	if err != nil {
		return fmt.Errorf("Generate(%q): checking basedir: %v", basedir, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("Generate(%q): basedir is not a directory", basedir)
	}

	stacks, err := ListStacks(basedir)
	if err != nil {
		return fmt.Errorf("Generate(%q): listing stack: %v", basedir, err)
	}

	var errs []error

	for _, stack := range stacks {
		tfcode, err := generateStackConfig(basedir, stack)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		genfile := filepath.Join(stack.Dir, GeneratedTfFilename)
		errs = append(errs, os.WriteFile(genfile, tfcode, 0666))
	}

	if err := errutil.Chain(errs...); err != nil {
		return fmt.Errorf("Generate(%q): %v", basedir, err)
	}

	return nil
}

func generateStackConfig(basedir string, stack Entry) ([]byte, error) {
	stackfile := filepath.Join(stack.Dir, ConfigFilename)
	stackconfig, err := os.ReadFile(stackfile)
	if err != nil {
		return nil, fmt.Errorf("reading stack config: %v", err)
	}

	parser := hcl.NewParser()
	parsed, err := parser.Parse(stackfile, stackconfig)
	if err != nil {
		return nil, fmt.Errorf("parsing stack config: %v", err)
	}

	// TODO(katcipis): handle no backend config + search through project dirs

	gen := hclwrite.NewEmptyFile()
	rootBody := gen.Body()
	tfBlock := rootBody.AppendNewBlock("terraform", nil)
	tfBody := tfBlock.Body()
	backendBlock := tfBody.AppendNewBlock("backend", parsed.Backend.Labels)
	backendBody := backendBlock.Body()

	if parsed.Backend.Body != nil {
		for name, attr := range parsed.Backend.Body.Attributes {
			val, err := attr.Expr.Value(nil)
			if err != nil {
				return nil, fmt.Errorf("parsing attribute %q: %v", name, err)
			}

			backendBody.SetAttributeValue(name, val)
		}
	}

	return append([]byte(GeneratedCodeHeader+"\n\n"), gen.Bytes()...), nil
}
