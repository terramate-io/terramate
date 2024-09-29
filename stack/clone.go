// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package stack

import (
	stdos "os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	hhcl "github.com/terramate-io/hcl/v2"
	"github.com/terramate-io/hcl/v2/hclwrite"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/fs"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/os"
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
func Clone(root *config.Root, destdir, srcdir os.Path, skipChildStacks bool) (int, error) {
	rootdir := root.Path()

	logger := log.With().
		Str("action", "stack.Clone()").
		Stringer("rootdir", rootdir).
		Stringer("destdir", destdir).
		Stringer("srcdir", srcdir).
		Bool("skipChildStacks", skipChildStacks).
		Logger()

	if !srcdir.HasPrefix(rootdir.String()) {
		return 0, errors.E(ErrInvalidStackDir, "src dir %q must be inside project root %q", srcdir, rootdir)
	}

	if !destdir.HasPrefix(rootdir.String()) {
		return 0, errors.E(ErrInvalidStackDir, "dest dir %q must be inside project root %q", destdir, rootdir)
	}

	if _, err := stdos.Stat(destdir.String()); err == nil {
		return 0, errors.E(ErrCloneDestDirExists, destdir)
	}

	needsCleanup := true
	defer func() {
		if !needsCleanup {
			return
		}

		if err := stdos.RemoveAll(destdir.String()); err != nil {
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
		Srcdir         os.Path
		Destdir        os.Path
		ShouldUpdateID bool
	}
	tasks := []cloneTask{}

	// Use this set to identify stack root dirs and don't recurse into them when copying
	stackset := map[os.Path]struct{}{}

	for _, e := range stackTrees {
		stackSrcdir := e.Path()
		rel, _ := filepath.Rel(srcdir.String(), stackSrcdir.String())

		stackDestdir := destdir.Join(rel)

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
		filter := func(dir os.Path, entry stdos.DirEntry) bool {
			if strings.HasPrefix(entry.Name(), ".") {
				return false
			}

			abspath := dir.Join(entry.Name())
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
func UpdateStackID(root *config.Root, stackdir os.Path) (string, error) {
	parser, err := hcl.NewTerramateParser(root.Path(), stackdir)
	if err != nil {
		return "", err
	}

	if err := parser.AddDir(stackdir); err != nil {
		return "", err
	}

	if err := parser.Parse(); err != nil {
		return "", err
	}

	stackFilePath, ok := getStackFilepath(parser)
	if !ok {
		return "", errors.E("stack does not have a stack block")
	}

	st, err := stdos.Lstat(stackFilePath.String())
	if err != nil {
		return "", errors.E(err, "stating the stack file")
	}

	originalFileMode := st.Mode()

	// Parsing HCL always delivers an AST that
	// has no comments on it, so building a new HCL file from the parsed
	// AST will lose all comments from the original code.

	stackContents, err := stdos.ReadFile(stackFilePath.String())
	if err != nil {
		return "", errors.E(err, "reading stack definition file")
	}

	parsed, diags := hclwrite.ParseConfig([]byte(stackContents), stackFilePath.String(), hhcl.InitialPos)
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

		err = stdos.WriteFile(stackFilePath.String(), parsed.Bytes(), originalFileMode)
		if err != nil {
			return "", err
		}
		return id, nil
	}

	return "", errors.E("stack block not found")
}

func getStackFilepath(parser *hcl.TerramateParser) (os.Path, bool) {
	for filepath, body := range parser.ParsedBodies() {
		for _, block := range body.Blocks {
			if block.Type == hcl.StackBlockType {
				return filepath, true
			}
		}
	}
	return "", false
}
