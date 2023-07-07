// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package stack

import (
	"os"
	"strings"

	"github.com/google/uuid"
	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/fs"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/project"
	"github.com/zclconf/go-cty/cty"
)

const (
	// ErrCloneDestDirExists indicates that the dest dir on a clone
	// operation already exists.
	ErrCloneDestDirExists errors.Kind = "clone dest dir exists"
)

// Clone will clone the stack at srcdir into destdir.
//
// - srcdir must be a stack (fail otherwise)
// - destdir must not exist (fail otherwise)
// - All files and directories are copied  (except dotfiles/dirs)
// - If cloned stack has an ID it will be adjusted to a generated UUID.
// - If cloned stack has no ID the cloned stack also won't have an ID.
func Clone(root *config.Root, destdir, srcdir string) error {
	rootdir := root.HostDir()

	logger := log.With().
		Str("action", "stack.Clone()").
		Str("rootdir", rootdir).
		Str("destdir", destdir).
		Str("srcdir", srcdir).
		Logger()

	logger.Trace().Msg("cloning stack, checking invariants")

	if !strings.HasPrefix(srcdir, rootdir) {
		return errors.E(ErrInvalidStackDir, "src dir %q must be inside project root %q", srcdir, rootdir)
	}

	if !strings.HasPrefix(destdir, rootdir) {
		return errors.E(ErrInvalidStackDir, "dest dir %q must be inside project root %q", destdir, rootdir)
	}

	if _, err := os.Stat(destdir); err == nil {
		return errors.E(ErrCloneDestDirExists, destdir)
	}

	srcStack, err := config.LoadStack(root, project.PrjAbsPath(root.HostDir(), srcdir))
	if err != nil {
		return errors.E(ErrInvalidStackDir, err, "src dir %q must be a valid stack", srcdir)
	}

	logger.Trace().Msg("copying stack files")

	if err := fs.CopyDir(destdir, srcdir, filterDotFiles); err != nil {
		return err
	}

	if srcStack.ID == "" {
		logger.Trace().Msg("stack has no ID, nothing else to do")
		return nil
	}

	logger.Trace().Msg("stack has ID, updating ID of the cloned stack")
	_, err = UpdateStackID(destdir)
	if err != nil {
		return err
	}

	return root.LoadSubTree(project.PrjAbsPath(rootdir, destdir))
}

func filterDotFiles(_ string, entry os.DirEntry) bool {
	return !strings.HasPrefix(entry.Name(), ".")
}

// UpdateStackID updates the stack.id of the given stack directory.
// The functions updates just the file which defines the stack block.
// The updated file will lose all comments.
func UpdateStackID(stackdir string) (string, error) {
	logger := log.With().
		Str("action", "stack.updateStackID()").
		Str("stack", stackdir).
		Logger()

	logger.Trace().Msg("parsing stack")

	parser, err := hcl.NewTerramateParser(stackdir, stackdir)
	if err != nil {
		return "", err
	}

	if err := parser.AddDir(stackdir); err != nil {
		return "", err
	}

	if err := parser.Parse(); err != nil {
		return "", err
	}

	logger.Trace().Msg("finding file containing stack definition")

	stackFilePath := getStackFilepath(parser)
	if stackFilePath == "" {
		return "", errors.E("stack does not have a stack block")
	}

	st, err := os.Lstat(stackFilePath)
	if err != nil {
		return "", errors.E(err, "stating the stack file")
	}

	originalFileMode := st.Mode()

	// Parsing HCL always delivers an AST that
	// has no comments on it, so building a new HCL file from the parsed
	// AST will lose all comments from the original code.

	logger.Trace().Msg("reading stack file")

	stackContents, err := os.ReadFile(stackFilePath)
	if err != nil {
		return "", errors.E(err, "reading stack definition file")
	}

	logger.Trace().Msg("parsing stack file")

	parsed, diags := hclwrite.ParseConfig([]byte(stackContents), stackFilePath, hhcl.InitialPos)
	if diags.HasErrors() {
		return "", errors.E(diags, "parsing stack configuration")
	}

	blocks := parsed.Body().Blocks()

	logger.Trace().Msg("searching for stack ID attribute")

	for _, block := range blocks {
		if block.Type() != hcl.StackBlockType {
			continue
		}

		uuid, err := uuid.NewRandom()
		if err != nil {
			return "", errors.E(err, "creating new ID for stack")
		}

		id := uuid.String()

		body := block.Body()
		body.SetAttributeValue("id", cty.StringVal(id))

		logger.Trace().Msg("saving updated file")

		err = os.WriteFile(stackFilePath, parsed.Bytes(), originalFileMode)
		if err != nil {
			return "", err
		}
		return id, nil
	}

	return "", errors.E("stack block not found")
}

func getStackFilepath(parser *hcl.TerramateParser) string {
	for filepath, body := range parser.ParsedBodies() {
		for _, block := range body.Blocks {
			if block.Type == hcl.StackBlockType {
				return filepath
			}
		}
	}
	return ""
}
