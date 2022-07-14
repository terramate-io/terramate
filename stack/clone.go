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
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/hcl/ast"
	"github.com/mineiros-io/terramate/hcl/eval"
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

	if err := parser.MinimalParse(); err != nil {
		return err
	}

	logger.Trace().Msg("finding file containing stack definition")

	stackFilePath, body := getStackBody(parser)
	if body == nil {
		return errors.E("updating stack ID: stack block not found")
	}

	// WHY oh WHY do you ask ? Integrating hcl/hclsyntax types on the
	// hclwrite is remarkably hard, it only works with cty.Values or tokens.
	// Since we don't want to eval anything here, tokens it is.
	// Then you may ask... is it possible to get the tokens of an expression
	// easily ? The answer, to your dismay, will be no. Hence this wonderful
	// hack, enjoy.
	// - https://raw.githubusercontent.com/katcipis/memes/master/satan.jpg

	stackContents, err := os.ReadFile(stackFilePath)
	if err != nil {
		return errors.E(err, "reading cloned stack definition file")
	}

	getExprTokens := func(expr hclsyntax.Expression) (hclwrite.Tokens, error) {
		tokens, err := eval.GetExpressionTokens(stackContents, stackFilePath, expr)
		if err != nil {
			return nil, errors.E(err, "internal error converting expression to tokens")
		}
		return tokens, nil
	}

	newStackFile := hclwrite.NewEmptyFile()

	newBody := newStackFile.Body()

	// Today we don't have attributes on the root body, but if we add any
	// someday then the code will just keep working.
	copyBodyAttributes(newBody, body, getExprTokens)

	for _, block := range body.Blocks {
		newBlock := newBody.AppendNewBlock(block.Type, block.Labels)
		if block.Body == nil {
			continue
		}

		newBody := newBlock.Body()

		if block.Type != hcl.StackBlockType {
			if err := hcl.CopyBody(newBody, block.Body, getExprTokens); err != nil {
				return err
			}
			continue
		}

		attrs := ast.SortRawAttributes(block.Body.Attributes)
		for _, attr := range attrs {
			if attr.Name == hcl.StackIDField {
				id, err := uuid.NewRandom()
				if err != nil {
					return errors.E(err, "creating new UUID for cloned stack")
				}
				newBody.SetAttributeValue(attr.Name, cty.StringVal(id.String()))
				continue
			}

			tokens, err := getExprTokens(attr.Expr)
			if err != nil {
				panic(errors.E(err, "internal error getting expression tokens"))
			}
			newBody.SetAttributeRaw(attr.Name, tokens)
		}
	}

	// Since we just created the clones stack files they have the default
	// permissions given by Go on os.Create, 0666.
	return os.WriteFile(stackFilePath, newStackFile.Bytes(), 0666)
}

func copyBodyAttributes(
	dest *hclwrite.Body,
	src *hclsyntax.Body,
	getExprTokens func(hclsyntax.Expression) (hclwrite.Tokens, error),
) {
	attrs := ast.SortRawAttributes(src.Attributes)
	for _, attr := range attrs {
		tokens, err := getExprTokens(attr.Expr)
		if err != nil {
			panic(errors.E(err, "internal error getting expression tokens"))
		}
		dest.SetAttributeRaw(attr.Name, tokens)
	}
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
