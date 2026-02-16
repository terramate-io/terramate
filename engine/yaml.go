// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package engine

import (
	"io/fs"
	"os"
	"strings"

	hhcl "github.com/terramate-io/hcl/v2"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/hcl/info"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/yaml"
)

func loadYAMLConfigs(rootcfg *config.Root) error {
	rootdir := rootcfg.HostDir()

	dirFS := os.DirFS(rootdir)
	err := fs.WalkDir(dirFS, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !isYAMLConfig(p) {
			return nil
		}

		r, err := dirFS.Open(p)
		if err != nil {
			return err
		}
		defer func() {
			_ = r.Close()
		}()

		var bundle yaml.BundleInstance
		err = yaml.Decode(r, &bundle)
		if err != nil {
			loc := hhcl.Range{
				Filename: p,
			}
			var yamlErr yaml.Error
			if errors.As(err, &yamlErr) {
				loc.Start = hhcl.Pos{Line: yamlErr.Line, Column: yamlErr.Column}
				loc.End = hhcl.Pos{Line: yamlErr.Line, Column: yamlErr.Column}
			}

			return errors.E(err, loc)
		}

		if err := mergeYAMLConfig(rootcfg, project.NewPath("/"+p), &bundle); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func isYAMLConfig(fn string) bool {
	return strings.HasSuffix(fn, ".tm.yml") || strings.HasSuffix(fn, ".tm.yaml")
}

func mergeYAMLConfig(rootcfg *config.Root, fn project.Path, bundle *yaml.BundleInstance) error {
	tree, found := rootcfg.Lookup(fn.Dir())
	if !found {
		return nil
	}

	rootdir := rootcfg.HostDir()
	abspath := fn.HostPath(rootdir)

	for _, existingBundle := range tree.Node.Bundles {
		if existingBundle.Name == bundle.Name.V {
			return errors.E("a bundle with name %q is already defined in %q", existingBundle.Name, existingBundle.Info)
		}
	}

	sourceAttr, err := convertAttribute("source", abspath, rootdir, bundle.Source)
	if err != nil {
		return err
	}
	inputsAttr, err := convertAttribute("inputs", abspath, rootdir, bundle.Inputs)
	if err != nil {
		return err
	}

	var envValues []*hcl.BundleEnvValues
	for _, envItem := range bundle.Environments.V {
		envIDAttribute, err := convertAttribute("environment_id", abspath, rootdir, envItem.Key)
		if err != nil {
			return err
		}

		var envSourceAttr *ast.Attribute
		envInputs := ast.NewMergedBlock("inputs", []string{})

		if envItem.Value.V != nil {
			if envItem.Value.V.Source.V != nil {
				envSourceAttr, err = convertAttribute("source", abspath, rootdir, envItem.Value.V.Source)
				if err != nil {
					return err
				}
			}

			for _, inputItem := range envItem.Value.V.Inputs.V {
				inputAttr, err := convertAttribute(inputItem.Key.V, abspath, rootdir, inputItem.Value)
				if err != nil {
					return err
				}
				envInputs.Attributes[inputItem.Key.V] = *inputAttr
			}
		}

		envValues = append(envValues, &hcl.BundleEnvValues{
			EnvID:  envIDAttribute,
			Source: envSourceAttr,
			Inputs: envInputs,
			Info:   info.NewRange(rootdir, hhcl.Range{Filename: abspath, Start: hhcl.Pos{Line: envItem.Key.Line, Column: envItem.Key.Column}}),
		})
	}

	var uuidAttr *ast.Attribute
	if bundle.UUID.V != "" {
		uuidAttr, err = convertAttribute("uuid", abspath, rootdir, bundle.UUID)
		if err != nil {
			return err
		}
	}

	bundleHCL := &hcl.BundleTemplate{
		Name:       bundle.Name.V,
		UUID:       uuidAttr,
		Workdir:    fn.Dir(),
		Info:       info.NewRange(rootdir, hhcl.Range{Filename: abspath}),
		Source:     sourceAttr,
		InputsAttr: inputsAttr,
		EnvValues:  envValues,
	}

	tree.Node.Bundles = append(tree.Node.Bundles, bundleHCL)
	return nil
}

func convertAttribute[T any](attrname, filename, rootdir string, attr yaml.Attribute[T]) (*ast.Attribute, error) {
	rng := hhcl.Range{
		Filename: filename,
		Start:    hhcl.Pos{Line: attr.Line, Column: attr.Column},
	}

	expr, err := yaml.ConvertToHCL(attr, rng)
	if err != nil {
		return nil, err
	}

	hclAttr := ast.NewAttribute(
		rootdir,
		&hhcl.Attribute{
			Name:  attrname,
			Expr:  expr,
			Range: rng,
		},
	)
	return &hclAttr, nil
}
