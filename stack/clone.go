// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package stack

import (
	"os"
	"path/filepath"
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
// - srcdir must contain at least one stack directly, or in subdirs unless skipChildStacks is set (fail otherwise)
// - destdir must not exist (fail otherwise)
// - if skipChildStacks is true, child stacks are ignored
// - All files and directories are copied  (except dotfiles/dirs)
// - If cloned stack has an ID it will be adjusted to a generated UUID.
// - If cloned stack has no ID the cloned stack also won't have an ID.
func Clone(root *config.Root, destdir, srcdir string, skipChildStacks bool) (int, error) {
	rootdir := root.HostDir()

	logger := log.With().
		Str("action", "stack.Clone()").
		Str("rootdir", rootdir).
		Str("destdir", destdir).
		Str("srcdir", srcdir).
		Bool("skipChildStacks", skipChildStacks).
		Logger()

	if !strings.HasPrefix(srcdir, rootdir) {
		return 0, errors.E(ErrInvalidStackDir, "src dir %q must be inside project root %q", srcdir, rootdir)
	}

	if !strings.HasPrefix(destdir, rootdir) {
		return 0, errors.E(ErrInvalidStackDir, "dest dir %q must be inside project root %q", destdir, rootdir)
	}

	if _, err := os.Stat(destdir); err == nil {
		return 0, errors.E(ErrCloneDestDirExists, destdir)
	}

	needsCleanup := true
	defer func() {
		if !needsCleanup {
			return
		}

		if err := os.RemoveAll(destdir); err != nil {
			logger.Debug().Err(err).Msg("failed to cleanup destdir after error")
		}
	}()

	srcpath := project.PrjAbsPath(rootdir, srcdir)

	// Get all stacks in srcpath (including children)
	tree, found := root.Lookup(srcpath)
	if !found {
		return 0, errors.E(ErrInvalidStackDir, "src dir %q must contain valid stacks", srcdir)
	}

	stackTrees := tree.Stacks()
	if len(stackTrees) == 0 {
		return 0, errors.E(ErrInvalidStackDir, "src dir %q must contain valid stacks", srcdir)
	}

	type cloneTask struct {
		Srcdir         string
		Destdir        string
		ShouldUpdateID bool
	}
	tasks := []cloneTask{}

	// Use this set to identify stack root dirs and don't recurse into them when copying
	stackset := map[string]struct{}{}

	for _, e := range stackTrees {
		stackSrcdir := e.HostDir()
		rel, _ := filepath.Rel(srcdir, stackSrcdir)

		stackDestdir := filepath.Join(destdir, rel)

		// If destdir is within srcdir, we could encounter a stackDestdir in the source dir
		// created by a previous cloneTask. They must be ignored, too.
		stackset[stackSrcdir] = struct{}{}
		stackset[stackDestdir] = struct{}{}

		if skipChildStacks && rel != "." {
			continue
		}

		tasks = append(tasks, cloneTask{
			Srcdir:         stackSrcdir,
			Destdir:        stackDestdir,
			ShouldUpdateID: e.Node.Stack.ID != "",
		})
	}

	if len(tasks) == 0 {
		return 0, errors.E(ErrInvalidStackDir, "no stacks to clone in %q", srcdir)
	}

	for _, st := range tasks {
		filter := func(dir string, entry os.DirEntry) bool {
			if strings.HasPrefix(entry.Name(), ".") {
				return false
			}

			abspath := filepath.Join(dir, entry.Name())
			_, found := stackset[abspath]
			return !found
		}

		if err := fs.CopyDir(st.Destdir, st.Srcdir, filter); err != nil {
			return 0, err
		}

		if !st.ShouldUpdateID {
			continue
		}

		if _, err := UpdateStackID(root, st.Destdir); err != nil {
			return 0, err
		}
	}

	needsCleanup = false
	return len(tasks), root.LoadSubTree(project.PrjAbsPath(rootdir, destdir))
}

// UpdateStackID updates the stack.id of the given stack directory.
// The functions updates just the file which defines the stack block.
// The updated file will lose all comments.
func UpdateStackID(root *config.Root, stackdir string) (string, error) {
	parser, err := hcl.NewTerramateParser(root.HostDir(), stackdir)
	if err != nil {
		return "", err
	}

	if err := parser.AddDir(stackdir); err != nil {
		return "", err
	}

	if err := parser.Parse(); err != nil {
		return "", err
	}

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

	stackContents, err := os.ReadFile(stackFilePath)
	if err != nil {
		return "", errors.E(err, "reading stack definition file")
	}

	parsed, diags := hclwrite.ParseConfig([]byte(stackContents), stackFilePath, hhcl.InitialPos)
	if diags.HasErrors() {
		return "", errors.E(diags, "parsing stack configuration")
	}

	blocks := parsed.Body().Blocks()

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
