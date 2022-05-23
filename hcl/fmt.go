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

package hcl

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/mineiros-io/terramate/errors"
	"github.com/rs/zerolog/log"
)

// FormatResult represents the result of a formatting operation.
type FormatResult struct {
	path      string
	formatted string
}

// Format will format the given source code. It returns an error if the given
// source is invalid HCL.
func Format(src, filename string) (string, error) {
	parsed, diags := hclwrite.ParseConfig([]byte(src), filename, hcl.InitialPos)
	if err := errors.L(diags).AsError(); err != nil {
		return "", errors.E(ErrHCLSyntax, err)
	}
	adjustBody(parsed.Body())
	return string(hclwrite.Format(parsed.Bytes())), nil
}

// FormatTree will format all Terramate configuration files
// in the given tree starting at the given dir. It will recursively
// navigate on sub directories. Directories starting with "." are ignored.
//
// Only Terramate configuration files will be formatted.
//
// Files that are already formatted are ignored. If all files are formatted
// this function returns an empty result.
//
// All files will be left untouched. To save the formatted result on disk you
// can use FormatResult.Save for each FormatResult.
func FormatTree(dir string) ([]FormatResult, error) {
	logger := log.With().
		Str("action", "hcl.FormatTree()").
		Str("dir", dir).
		Logger()

	logger.Trace().Msg("listing terramate files")

	files, err := listTerramateFiles(dir)
	if err != nil {
		return nil, errors.E(errFormatTree, err)
	}

	results := []FormatResult{}
	errs := errors.L()

	for _, f := range files {
		logger := log.With().
			Str("file", f).
			Logger()

		logger.Trace().Msg("reading file")

		path := filepath.Join(dir, f)
		fileContents, err := os.ReadFile(path)
		if err != nil {
			errs.Append(err)
			continue
		}

		logger.Trace().Msg("formatting file")

		currentCode := string(fileContents)
		formatted, err := Format(currentCode, path)
		if err != nil {
			errs.Append(err)
			continue
		}

		if currentCode == formatted {
			logger.Trace().Msg("file already formatted")
			continue
		}

		logger.Trace().Msg("file needs formatting, adding to results")

		results = append(results, FormatResult{
			path:      path,
			formatted: formatted,
		})
	}

	dirs, err := listTerramateDirs(dir)
	if err != nil {
		errs.Append(err)
		return nil, errors.E(errFormatTree, errs)
	}

	for _, d := range dirs {
		logger := log.With().
			Str("subdir", d).
			Logger()

		logger.Trace().Msg("recursively formatting")
		subres, err := FormatTree(filepath.Join(dir, d))
		if err != nil {
			errs.Append(err)
			continue
		}
		results = append(results, subres...)
	}

	if err := errs.AsError(); err != nil {
		return nil, err
	}
	return results, nil
}

// Save will save the formatted result on the original file, replacing
// its original contents.
func (f FormatResult) Save() error {
	return os.WriteFile(f.path, []byte(f.formatted), 0644)
}

// Path is the absolute path of the original file.
func (f FormatResult) Path() string {
	return f.path
}

// Formatted is the contents of the original file after formatting.
func (f FormatResult) Formatted() string {
	return f.formatted
}

const (
	errFormatTree errors.Kind = "formatting tree"
)

func adjustBody(body *hclwrite.Body) {
	// We don't actually format the body, we just adjust it, adding
	// newlines in a way that hclwrite.Format will format things the way we want.
	// This is a quick/nasty hack.
	logger := log.With().
		Str("action", "hcl.adjustBody()").
		Logger()

	attrs := body.Attributes()
	for name, attr := range attrs {
		logger.Trace().
			Str("name", name).
			Msg("adjusting attribute")
		body.SetAttributeRaw(name, adjustAttrExpr(attr.Expr().BuildTokens(nil)))
	}

	blocks := body.Blocks()
	for _, block := range blocks {
		adjustBody(block.Body())
	}
}

func adjustAttrExpr(tokens hclwrite.Tokens) hclwrite.Tokens {
	logger := log.With().
		Str("action", "hcl.addNewlines()").
		Str("tokens", tokensStr(tokens)).
		Logger()

	logger.Trace().Msg("trimming newlines")

	trimmed := trimNewlines(tokens)

	logger = logger.With().
		Str("trimmedTokens", tokensStr(trimmed)).
		Logger()

	// We are interested on lists, ignore the rest
	if trimmed[0].Type == hclsyntax.TokenOBrack {
		// We don't need the position of the next token here
		// since there shouldn't be any next token on this case.
		adjustedList, pos := adjustListExpr(trimmed)
		if pos != len(trimmed) {
			panic(fmt.Errorf(
				"last pos %d != tokens len %d for tokens: %s",
				pos,
				len(trimmed),
				tokensStr(trimmed),
			))
		}
		return adjustedList
	}

	logger.Trace().Msg("not a list, returning tokens as is")
	return trimmed
}

// adjustListExpr will adjust the given list tokens so they can be formatted
// properly. It returns the adjusted tokens and the position of the first
// token after the list ended.
//
// If there is no more tokens after the end of
// the list the returned position will be equal to len(tokens).
func adjustListExpr(tokens hclwrite.Tokens) (hclwrite.Tokens, int) {
	logger := log.With().
		Str("action", "hcl.adjustListExpr()").
		Str("tokens", tokensStr(tokens)).
		Logger()

	if isListComprehension(tokens) {
		logger.Trace().Msg("list comprehension, ignoring")
		return tokens, len(tokens)
	}

	logger.Trace().Msg("it is a list, adjusting")

	newTokens := hclwrite.Tokens{tokens[0], newlineToken()}
	elemNextPos := 1 // Skip '['

	for {
		_, skipped := skipNewlines(tokens[elemNextPos:])
		elemNextPos += skipped

		logger.Trace().
			Int("skipped", skipped).
			Msg("skipped newlines")

		if tokens[elemNextPos].Type == hclsyntax.TokenCBrack {
			logger.Trace().Msg("reached end of list")
			break
		}

		if tokens[elemNextPos].Type == hclsyntax.TokenComma {
			logger.Trace().Msg("comma found, starting new iteration")

			elemNextPos++
			// restart processing so we eliminate newlines after comma
			// we don't need to worry about handling multiple commas
			// as valid since we already validate the code before this.
			continue
		}

		logger.Trace().Msg("getting next element of the list")

		element, nextPos := getNextListElement(tokens[elemNextPos:])
		elemNextPos += nextPos

		logger.Trace().
			Str("element", tokensStr(element)).
			Str("tokens", tokensStr(tokens)).
			Int("elemNextPos", elemNextPos).
			Msg("new element got")

		newTokens = append(newTokens, element...)
		newTokens = append(newTokens, commaToken(), newlineToken())
	}

	newTokens = append(newTokens, closeBracketToken())
	elemNextPos++

	logger.Trace().Msg("returning adjusted list")
	return newTokens, elemNextPos
}

func adjustObjExpr(tokens hclwrite.Tokens) (hclwrite.Tokens, int) {
	logger := log.With().
		Str("action", "hcl.adjustObjExpr()").
		Str("tokens", tokensStr(tokens)).
		Logger()

	// TODO(katcipis): we also want to improve list formatting inside objects
	// Not doing it for now, but here would be the place to add it.

	logger.Trace().Msg("searching for end of object definition")
	openBraces := 0

	for i, token := range tokens {
		switch token.Type {
		case hclsyntax.TokenOBrace:
			openBraces++
		case hclsyntax.TokenCBrace:
			openBraces--
		}
		if openBraces == 0 {
			i++
			return trimNewlines(tokens[0:i]), i
		}
	}

	panic(fmt.Errorf("object tokens %q expected to end with }", tokensStr(tokens)))
}

func getNextListElement(tokens hclwrite.Tokens) (hclwrite.Tokens, int) {
	if tokens[0].Type == hclsyntax.TokenOBrack {
		return adjustListExpr(tokens)
	}
	if tokens[0].Type == hclsyntax.TokenOBrace {
		return adjustObjExpr(tokens)
	}
	for i, token := range tokens {
		if token.Type == hclsyntax.TokenComma || token.Type == hclsyntax.TokenCBrack {
			return trimNewlines(tokens[0:i]), i
		}
	}
	panic(fmt.Errorf("tokens %q expected to end with , or ]", tokensStr(tokens)))
}

func closeBracketToken() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenCBrack,
		Bytes: []byte("]"),
	}
}

func commaToken() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenComma,
		Bytes: []byte(","),
	}
}

func newlineToken() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenNewline,
		Bytes: []byte("\n"),
	}
}

func trimNewlines(tokens hclwrite.Tokens) hclwrite.Tokens {
	if len(tokens) == 0 {
		return nil
	}

	var start int
	for start = 0; start < len(tokens); start++ {
		if tokens[start].Type != hclsyntax.TokenNewline {
			break
		}
	}

	var end int
	for end = len(tokens); end > 0; end-- {
		if tokens[end-1].Type != hclsyntax.TokenNewline {
			break
		}
	}

	if end < start {
		return nil
	}

	return tokens[start:end]
}

func tokensStr(tokens hclwrite.Tokens) string {
	return string(tokens.Bytes())
}

func skipNewlines(tokens hclwrite.Tokens) (hclwrite.Tokens, int) {
	for i, token := range tokens {
		if token.Type != hclsyntax.TokenNewline {
			return tokens[i:], i
		}
	}
	return nil, len(tokens)
}

func isListComprehension(tokens hclwrite.Tokens) bool {
	// Here we already assume the first token is [
	// So we are trying to determine if it is a list comprehension.
	tokens, _ = skipNewlines(tokens[1:])
	return tokens[0].Type == hclsyntax.TokenIdent &&
		string(tokens[0].Bytes) == "for"
}
