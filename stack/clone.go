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
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/hcl/lex"
	"github.com/rs/zerolog/log"
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

	if err := copyDir(destdir, srcdir); err != nil {
		return err
	}

	if _, ok := srcStack.ID(); !ok {
		logger.Trace().Msg("stack has no ID, nothing else to do")
		return nil
	}

	logger.Trace().Msg("stack has ID, updating ID of the cloned stack")
	return updateStackID(destdir)
}

func copyDir(destdir, srcdir string) error {
	entries, err := os.ReadDir(srcdir)
	if err != nil {
		return errors.E(err, "reading src dir")
	}

	if err := os.MkdirAll(destdir, createDirMode); err != nil {
		return errors.E(err, "creating dest dir")
	}

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		srcpath := filepath.Join(srcdir, entry.Name())
		destpath := filepath.Join(destdir, entry.Name())

		if entry.IsDir() {
			if err := copyDir(destpath, srcpath); err != nil {
				return errors.E(err, "copying src to dest dir")
			}
			continue
		}

		if err := copyFile(destpath, srcpath); err != nil {
			return errors.E(err, "copying src to dest file")
		}
	}

	return nil
}

func copyFile(destfile, srcfile string) error {
	src, err := os.Open(srcfile)
	if err != nil {
		return errors.E(err, "opening source file")
	}
	dest, err := os.Create(destfile)
	if err != nil {
		return errors.E(err, "creating dest file")
	}
	_, err = io.Copy(dest, src)
	return err
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

	stackFilePath, body := getStackBody(parser)
	if body == nil {
		return errors.E("updating stack ID: stack block not found")
	}

	// WHY oh WHY do you ask ? Parsing HCL always delivers an AST that
	// has no comments on it, so building a new HCL file from the parsed
	// AST will lose all comments from the original code.
	//
	// - https://raw.githubusercontent.com/katcipis/memes/master/satan.jpg

	logger.Trace().Msg("reading cloned stack file")

	stackContents, err := os.ReadFile(stackFilePath)
	if err != nil {
		return errors.E(err, "reading cloned stack definition file")
	}

	logger.Trace().Msg("lexing cloned stack file")

	roTokens, err := lex.Config(stackContents, stackFilePath)
	if err != nil {
		return errors.E(err, "cloned stack is invalid HCL")
	}

	logger.Trace().Msg("finding stack block on tokens")

	tokens := lex.WriterTokens(roTokens)
	blockStart, ok := lex.FindTokenSequence(tokens, lex.TokenIdent(hcl.StackBlockType), lex.TokenOBrace())
	if !ok {
		return errors.E(err, "cloned stack doesn't have stack block")
	}
	blockStart += 2

	logger.Trace().Msg("finding id attribute")

	// We can assume at this point that the stack has an ID, since previous parsing checked that.
	// This is not generally safe, if we allow the stack block to have attributes that
	// have objects as values and those objects can have an "id" field this will fail.
	// If we allow multiple stack blocks, this will also fail.
	// For now we are assuming stack blocks are constrained, for something safer we need
	// a more proper parser instead of YOLO lexing.

	idAttributeOffset, ok := lex.FindTokenSequence(tokens[blockStart:], lex.TokenIdent(hcl.StackIDField), lex.TokenEqual())
	if !ok {
		return errors.E(err, "cloned stack doesn't have stack ID")
	}
	idAttributeOffset += 2

	logger.Trace().Msg("updating id attribute")

	// Here we assume that stack IDs are also on the form:
	// id = "id"
	// Id's are very constrained and are always strings, so we expect
	// the tokens: TokenOQuote + TokenQuotedLit + TokenCQuote
	idQuotedLiteral := tokens[blockStart+idAttributeOffset+1]
	id, err := uuid.NewRandom()
	if err != nil {
		return errors.E(err, "creating new ID for cloned stack")
	}
	idQuotedLiteral.Bytes = []byte(id.String())

	logger.Trace().Msg("saving updated tokens")

	// Since we just created the clones stack files they have the default
	// permissions given by Go on os.Create, 0666.
	return os.WriteFile(stackFilePath, tokens.Bytes(), 0666)
}

func getStackBody(parser *hcl.TerramateParser) (string, *hclsyntax.Body) {
	for filepath, body := range parser.ParsedBodies() {
		for _, block := range body.Blocks {
			if block.Type == hcl.StackBlockType {
				return filepath, body
			}
		}
	}
	return "", nil
}
