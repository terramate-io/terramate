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
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2/hclsyntax"
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
		// TODO(katcipis): test no config found, so no code generated
		tfcode, err := generateStackConfig(basedir, stack.Dir)
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

func generateStackConfig(basedir string, configdir string) ([]byte, error) {
	if !strings.HasPrefix(configdir, basedir) {
		// check if we are outside of basedir
		return nil, nil
	}

	configfile := filepath.Join(configdir, ConfigFilename)

	if _, err := os.Stat(configfile); err != nil {
		return generateStackConfig(basedir, filepath.Dir(configdir))
	}

	config, err := os.ReadFile(configfile)
	if err != nil {
		return nil, fmt.Errorf("reading config: %v", err)
	}

	parser := hcl.NewParser()
	parsed, err := parser.Parse(configfile, config)
	if err != nil {
		return nil, fmt.Errorf("parsing config: %v", err)
	}

	if parsed.Backend == nil {
		return generateStackConfig(basedir, filepath.Dir(configdir))
	}

	gen := hclwrite.NewEmptyFile()
	rootBody := gen.Body()
	tfBlock := rootBody.AppendNewBlock("terraform", nil)
	tfBody := tfBlock.Body()
	backendBlock := tfBody.AppendNewBlock("backend", parsed.Backend.Labels)
	backendBody := backendBlock.Body()

	if err := copyBody(backendBody, parsed.Backend.Body); err != nil {
		return nil, err
	}

	return append([]byte(GeneratedCodeHeader+"\n\n"), gen.Bytes()...), nil
}

func copyBody(target *hclwrite.Body, src *hclsyntax.Body) error {
	if src == nil || target == nil {
		return nil
	}

	// Avoid generating code randomly different (random attr order)
	attrs := sortedAttributes(src.Attributes)

	for _, attr := range attrs {
		val, err := attr.Expr.Value(nil)
		if err != nil {
			return fmt.Errorf("parsing attribute %q: %v", attr.Name, err)
		}

		target.SetAttributeValue(attr.Name, val)
	}

	for _, block := range src.Blocks {
		targetBlock := target.AppendNewBlock(block.Type, block.Labels)
		targetBody := targetBlock.Body()

		if err := copyBody(targetBody, block.Body); err != nil {
			return err
		}
	}

	return nil
}

func sortedAttributes(attrs hclsyntax.Attributes) []*hclsyntax.Attribute {
	names := make([]string, 0, len(attrs))

	for name := range attrs {
		names = append(names, name)
	}

	sort.Strings(names)

	sorted := make([]*hclsyntax.Attribute, len(names))
	for i, name := range names {
		sorted[i] = attrs[name]
	}

	return sorted
}
