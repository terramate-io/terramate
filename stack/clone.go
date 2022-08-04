// Copyright 2022 Mineiros GmbH
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

package stack

import (
	"os"
	"strings"

	"github.com/google/uuid"
	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/fs"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/rs/zerolog/log"
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
func Clone(rootdir, destdir, srcdir string) error {
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

	srcStack, err := Load(rootdir, srcdir)
	if err != nil {
		return errors.E(ErrInvalidStackDir, err, "src dir %q must be a valid stack", srcdir)
	}

	logger.Trace().Msg("copying stack files")

	if err := fs.CopyTree(destdir, srcdir, filterDotFiles); err != nil {
		return err
	}

	if _, ok := srcStack.ID(); !ok {
		logger.Trace().Msg("stack has no ID, nothing else to do")
		return nil
	}

	logger.Trace().Msg("stack has ID, updating ID of the cloned stack")
	return updateStackID(destdir)
}

func filterDotFiles(entry os.DirEntry) bool {
	return !strings.HasPrefix(entry.Name(), ".")
}

func updateStackID(stackdir string) error {
	logger := log.With().
		Str("action", "stack.updateStackID()").
		Str("stack", stackdir).
		Logger()

	logger.Trace().Msg("parsing stack")

	parser, err := hcl.NewTerramateParser(stackdir, stackdir)
	if err != nil {
		return err
	}

	if err := parser.AddDir(stackdir); err != nil {
		return err
	}

	if err := parser.Parse(); err != nil {
		return err
	}

	logger.Trace().Msg("finding file containing stack definition")

	stackFilePath := getStackFilepath(parser)
	if stackFilePath == "" {
		return errors.E("cloned stack does not have a stack block")
	}

	// Parsing HCL always delivers an AST that
	// has no comments on it, so building a new HCL file from the parsed
	// AST will lose all comments from the original code.

	logger.Trace().Msg("reading cloned stack file")

	stackContents, err := os.ReadFile(stackFilePath)
	if err != nil {
		return errors.E(err, "reading cloned stack definition file")
	}

	logger.Trace().Msg("parsing cloned stack file")

	parsed, diags := hclwrite.ParseConfig([]byte(stackContents), stackFilePath, hhcl.InitialPos)
	if diags.HasErrors() {
		return errors.E(diags, "parsing cloned stack configuration")
	}

	blocks := parsed.Body().Blocks()

	logger.Trace().Msg("searching for stack ID attribute")

updateStackID:
	for _, block := range blocks {
		if block.Type() != hcl.StackBlockType {
			continue
		}

		body := block.Body()
		attrs := body.Attributes()
		for name := range attrs {
			if name != hcl.StackIDField {
				continue
			}

			id, err := uuid.NewRandom()
			if err != nil {
				return errors.E(err, "creating new ID for cloned stack")
			}

			logger.Trace().
				Str("newID", id.String()).
				Msg("found stack ID attribute, updating")

			body.SetAttributeValue(name, cty.StringVal(id.String()))
			break updateStackID
		}
	}

	logger.Trace().Msg("saving updated file")

	// Since we just created the clones stack files they have the default
	// permissions given by Go on os.Create, 0666.
	return os.WriteFile(stackFilePath, parsed.Bytes(), 0666)
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
