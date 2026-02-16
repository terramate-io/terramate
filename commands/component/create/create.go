// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package create provides the component create command.
package create

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/apparentlymart/go-versions/versions"
	"github.com/rs/zerolog/log"
	"github.com/zclconf/go-cty/cty"

	"github.com/terramate-io/hcl/v2"
	"github.com/terramate-io/hcl/v2/hclparse"
	"github.com/terramate-io/hcl/v2/hclsyntax"
	"github.com/terramate-io/hcl/v2/hclwrite"
	"github.com/terramate-io/terramate/commands"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/git"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/printer"
)

// Spec is the command specification for the package publish command.
type Spec struct {
	Path       string
	workingDir string
	engine     *engine.Engine
	printers   printer.Printers
}

// Name returns the name of the command.
func (s *Spec) Name() string { return "component create" }

// Requirements returns the requirements of the command.
func (s *Spec) Requirements(context.Context, commands.CLI) any { return commands.RequireEngine() }

// Exec executes the create-component command.
func (s *Spec) Exec(_ context.Context, cli commands.CLI) error {
	s.engine = cli.Engine()
	s.printers = cli.Printers()

	// Get the project root directory - this is not affected by -C flag
	root := s.engine.Config()
	rootDir := root.HostDir()

	workingDir := cli.WorkingDir()
	log.Debug().
		Str("cli.WorkingDir()", workingDir).
		Str("root.HostDir()", rootDir).
		Str("s.Path", s.Path).
		Msg("component create: path resolution")

	// Resolve the target path relative to the working directory.
	// Absolute paths (e.g. /modules/s3-module) are project-relative and joined with rootDir.
	// Relative paths are joined with workingDir.
	var targetPath string
	if s.Path == "" || s.Path == "." {
		targetPath = workingDir
	} else if filepath.IsAbs(s.Path) {
		targetPath = filepath.Clean(filepath.Join(rootDir, s.Path))
	} else {
		targetPath = filepath.Clean(filepath.Join(workingDir, s.Path))
	}

	// Ensure the resolved path is within the project root.
	if targetPath != rootDir && !strings.HasPrefix(targetPath, rootDir+string(filepath.Separator)) {
		return errors.E("target path %s is outside the project root %s", targetPath, rootDir)
	}

	log.Debug().
		Str("final_targetPath", targetPath).
		Msg("component create: final resolved path")

	// Validate that the path exists and is a directory
	info, err := os.Stat(targetPath)
	if err != nil {
		return errors.E(err, "path does not exist: %s", targetPath)
	}
	if !info.IsDir() {
		return errors.E("path is not a directory: %s", targetPath)
	}

	s.workingDir = targetPath

	logger := log.With().Logger()

	parser := hclparse.NewParser()

	var vs []Variable

	err = filepath.WalkDir(s.workingDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == s.workingDir {
			return nil
		}
		if d.IsDir() {
			return filepath.SkipDir
		}
		if !d.Type().IsRegular() || !strings.HasSuffix(path, ".tf") {
			return nil
		}

		hclFile, diags := parser.ParseHCLFile(path)
		if diags.HasErrors() {
			return errors.E(diags)
		}

		body := hclFile.Body.(*hclsyntax.Body)

		for _, block := range body.Blocks {
			if block.Type != "variable" {
				continue
			}
			var v Variable

			if len(block.Labels) == 1 {
				v.Name = block.Labels[0]
			} else {
				logger.Debug().Msgf("ignoring variable block with %d labels", len(block.Labels))
				continue
			}

			typeAttr, ok := block.Body.Attributes["type"]
			if ok {
				v.Type = ast.TokensForExpression(typeAttr.Expr)
			}
			descAttr, ok := block.Body.Attributes["description"]
			if ok {
				v.Description = ast.TokensForExpression(descAttr.Expr)
			}
			defaultAttr, ok := block.Body.Attributes["default"]
			if ok {
				v.Default = ast.TokensForExpression(defaultAttr.Expr)
			}

			vs = append(vs, v)
		}
		return nil
	})
	if err != nil {
		return err
	}

	md, err := s.detectComponentMetadata()
	if err != nil {
		return err
	}

	fileData, err := buildComponentData(md, vs)
	if err != nil {
		return err
	}

	outfile := filepath.Join(s.workingDir, "component.tm.hcl")

	if _, err := os.Stat(outfile); err == nil || !os.IsNotExist(err) {
		return errors.E("output fil already exists: %s", outfile)
	}

	if err := os.WriteFile(outfile, fileData, 0644); err != nil {
		return errors.E(err, "failed to write manifest file")
	}

	return nil
}

// Metadata holds the auto-detected metadata for a component definition.
type Metadata struct {
	Name        string
	Class       string
	Description string
	Version     string
	Source      string
}

// Variable represents a Terraform variable extracted from a module.
type Variable struct {
	Name        string
	Type        hclwrite.Tokens
	Description hclwrite.Tokens
	Default     hclwrite.Tokens
}

func (s *Spec) detectComponentMetadata() (*Metadata, error) {
	md := &Metadata{}

	g := s.engine.Project().Git.Wrapper

	rootDir, err := g.Root()
	if err != nil {
		return nil, err
	}

	repoURL, err := g.URL("origin")
	if err != nil {
		return nil, err
	}

	repo, err := git.NormalizeGitURI(repoURL)
	if err != nil {
		return nil, err
	}

	repoDir, err := filepath.Rel(rootDir, s.workingDir)
	if err != nil {
		return nil, err
	}
	repoDir = filepath.ToSlash(repoDir)

	if repoDir != "" {
		md.Source = fmt.Sprintf("%s//%s", repo.Repo, repoDir)
	} else {
		md.Source = repo.Repo
	}

	dirNames := strings.Split(repoDir, "/")

	var classParts []string

	ignoredDirNames := []string{
		"github.com",
	}

	for _, dir := range slices.Backward(dirNames) {
		if slices.Contains(ignoredDirNames, dir) {
			continue
		}
		// If we reach a modules folder, we stop, asssuming we leave the range
		// of anything relevant for module naming.
		if dir == "modules" {
			break
		}

		// Is this a version?
		if v, found := detectVersion(dir); found {
			if md.Version == "" {
				md.Version = v
			}
			continue
		}

		// First folder that is not a version is the name.
		if md.Name == "" {
			md.Name = dir
		}
		classParts = append(classParts, dir)
	}

	slices.Reverse(classParts)
	md.Class = strings.Join(classParts, "/")

	// Defaults
	if md.Version == "" {
		md.Version = "1.0.0"
	}
	if md.Name == "" {
		md.Name = "unnamed-component"
	}
	if md.Class == "" {
		md.Class = "unnamed-component-class"
	}

	md.Description = fmt.Sprintf("Auto-generated from Terraform module at %s", md.Source)

	return md, nil
}

func detectVersion(dir string) (string, bool) {
	dir, _ = strings.CutPrefix(dir, "v")
	v, err := versions.ParseVersion(dir)
	if err == nil {
		return v.String(), true
	}
	return "", false
}

func buildComponentData(md *Metadata, vs []Variable) ([]byte, error) {
	file := hclwrite.NewEmptyFile()
	fileBody := file.Body()

	metadataBlock := fileBody.AppendNewBlock("define", []string{"component", "metadata"}).Body()
	metadataBlock.SetAttributeValue("class", cty.StringVal(md.Class))
	metadataBlock.SetAttributeValue("version", cty.StringVal(md.Version))
	metadataBlock.SetAttributeValue("name", cty.StringVal(md.Name))
	metadataBlock.SetAttributeValue("description", cty.StringVal(md.Description))
	metadataBlock.SetAttributeValue("technologies", cty.TupleVal([]cty.Value{cty.StringVal("terraform")}))

	inputsBlock := fileBody.AppendNewBlock("define", []string{"component"}).Body()

	for _, v := range vs {
		inputBlocks := inputsBlock.AppendNewBlock("input", []string{v.Name}).Body()
		if v.Type != nil {
			inputBlocks.SetAttributeRaw("type", v.Type)
		}
		if v.Description != nil {
			inputBlocks.SetAttributeRaw("description", v.Description)
		}
		if v.Default != nil {
			inputBlocks.SetAttributeRaw("default", v.Default)
		}
	}

	genhclBlock := fileBody.AppendNewBlock("generate_hcl", []string{"main.tf"}).Body()
	contentBlock := genhclBlock.AppendNewBlock("content", []string{}).Body()
	moduleBlock := contentBlock.AppendNewBlock("module", []string{"name"}).Body()

	moduleBlock.SetAttributeValue("source", cty.StringVal(md.Source))

	for _, v := range vs {
		moduleBlock.SetAttributeTraversal(v.Name, hcl.Traversal{
			hcl.TraverseRoot{Name: "component"},
			hcl.TraverseAttr{Name: "input"},
			hcl.TraverseAttr{Name: v.Name},
			hcl.TraverseAttr{Name: "value"},
		})
	}

	return hclwrite.Format(file.Bytes()), nil
}
