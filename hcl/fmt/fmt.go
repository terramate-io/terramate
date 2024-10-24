// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package fmt contains functions for formatting hcl config.
package fmt

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/terramate-io/hcl/v2"
	"github.com/terramate-io/hcl/v2/hclsyntax"
	"github.com/terramate-io/hcl/v2/hclwrite"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/fs"
)

// ErrHCLSyntax is the error kind for syntax errors.
const ErrHCLSyntax errors.Kind = "HCL syntax error"

// ErrReadFile is the error kind for any error related to reading the file content.
const ErrReadFile errors.Kind = "failed to read file"

// FormatResult represents the result of a formatting operation.
type FormatResult struct {
	path      string
	formatted string
}

// FormatMultiline will format the given source code.
// It enforces lists to be formatted as multiline, where each
// element on the list resides on its own line followed by a comma.
//
// It returns an error if the given source is invalid HCL.
func FormatMultiline(src, filename string) (string, error) {
	parsed, diags := hclwrite.ParseConfig([]byte(src), filename, hcl.InitialPos)
	if diags.HasErrors() {
		return "", errors.E(ErrHCLSyntax, diags)
	}
	fmtBody(parsed.Body())
	return string(hclwrite.Format(parsed.Bytes())), nil
}

// Format will format the given source code using hcl.Format.
// It returns an error if the given source is invalid HCL.
func Format(src, filename string) (string, error) {
	parsed, diags := hclwrite.ParseConfig([]byte(src), filename, hcl.InitialPos)
	if diags.HasErrors() {
		return "", errors.E(ErrHCLSyntax, diags)
	}
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
	// TODO(i4k): use files from the config tree.
	files, _, dirs, err := fs.ListTerramateFiles(dir)
	if err != nil {
		return nil, errors.E(errFormatTree, err)
	}
	sort.Strings(files)

	errs := errors.L()
	results, err := FormatFiles(dir, files)

	errs.Append(err)

	sort.Strings(dirs)
	for _, d := range dirs {
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

// FormatFiles will format all the provided Terramate paths.
// Only Terramate configuration files can be reliably formatted with this function.
// If HCL files for a different tool is provided, the result is unpredictable.
//
// Note: The provided file paths can be absolute or relative. If relative, ensure
// working directory is corrected adjusted. The special `-` filename is treated as a
// normal filename, then if it needs to be interpreted as `stdin` this needs to be
// handled separately by the caller.
//
// Files that are already formatted are ignored. If all files are formatted
// this function returns an empty result.
//
// All files will be left untouched. To save the formatted result on disk you
// can use FormatResult.Save for each FormatResult.
func FormatFiles(basedir string, files []string) ([]FormatResult, error) {
	results := []FormatResult{}
	errs := errors.L()

	for _, file := range files {
		fname := file
		if !filepath.IsAbs(file) {
			fname = filepath.Join(basedir, file)
		}
		fileContents, err := os.ReadFile(fname)
		if err != nil {
			errs.Append(errors.E(ErrReadFile, err))
			continue
		}
		currentCode := string(fileContents)
		formatted, err := Format(currentCode, fname)
		if err != nil {
			errs.Append(err)
			continue
		}
		if currentCode == formatted {
			continue
		}
		results = append(results, FormatResult{
			path:      fname,
			formatted: formatted,
		})
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

func fmtBody(body *hclwrite.Body) {
	attrs := body.Attributes()
	for name, attr := range attrs {
		body.SetAttributeRaw(name, fmtAttrExpr(attr.Expr().BuildTokens(nil)))
	}

	blocks := body.Blocks()
	for _, block := range blocks {
		fmtBody(block.Body())
	}
}

func fmtAttrExpr(tokens hclwrite.Tokens) hclwrite.Tokens {
	formatted, pos := fmtExpr(tokens)
	if pos != len(tokens) {
		panic(fmt.Errorf(
			"last pos %d != tokens len %d for tokens: %q",
			pos,
			len(tokens),
			tokensStr(tokens),
		))
	}
	return formatted
}

// fmtListExpr will adjust the given list tokens so they can be formatted
// properly. It returns the adjusted tokens and the position of the first
// token after the list ended.
//
// isInsideObject is necessary to disambiguate between a list indexing operation
// and lists being used as keys inside an object.
//
// If there is no more tokens after the end of
// the list the returned position will be equal to len(tokens).
func fmtListExpr(tokens hclwrite.Tokens, isInsideObject bool) (hclwrite.Tokens, int) {
	elemIndex := 0
	newTokens := hclwrite.Tokens{tokens[elemIndex], newlineToken()}
	elemIndex++

	skipNls := func() {
		_, skipped := skipNewlines(tokens[elemIndex:])
		elemIndex += skipped
	}

	for {
		skipNls()

		tokenType := tokens[elemIndex].Type

		if tokenType == hclsyntax.TokenComma {
			elemIndex++
			continue
		}

		if tokenType == hclsyntax.TokenComment {
			newTokens = append(newTokens, tokens[elemIndex])
			elemIndex++
			continue
		}

		if tokenType == hclsyntax.TokenCBrack {
			break
		}

		element, nextPos := fmtExpr(tokens[elemIndex:])
		elemIndex += nextPos

		element = trimNewlines(element)

		newTokens = append(newTokens, element...)
		// Heredocs need to be handled differently, the comma must
		// be on the next line in this case
		if isHeredoc(element) {
			newTokens = append(newTokens, newlineToken())
		}

		// On some scenarios like { [] = etc, ... } we need to avoid double commas
		if newTokens[len(newTokens)-1].Type != hclsyntax.TokenComma {
			newTokens = append(newTokens, commaToken())
		}
		newTokens = append(newTokens, newlineToken())
	}

	newTokens = append(newTokens, closeBracketToken())
	elemIndex++

	// Handling ["one"][0] and things like [[0]%[0]]
	// We can also have newlines when dealing with operations

	if elemIndex == len(tokens) {
		return newTokens, elemIndex
	}

	// We are handling things like this:
	// var = [[] # c1\n\n #c\n [*]]
	// We need to keep any comments after the immediate end of the list
	// We don't keep the extra newlines, only newlines belonging to comments themselves.
	searchCommentOrNl := true
	hasNewlineBeforeToken := false

	for searchCommentOrNl {
		switch tokens[elemIndex].Type {
		case hclsyntax.TokenComment:
			newTokens = append(newTokens, tokens[elemIndex])
			elemIndex++
		case hclsyntax.TokenNewline:
			hasNewlineBeforeToken = true
			elemIndex++
		default:
			searchCommentOrNl = false
		}
	}

	nextTokenType := tokens[elemIndex].Type

	switch nextTokenType {
	case hclsyntax.TokenIdent, hclsyntax.TokenCBrace,
		hclsyntax.TokenNumberLit, hclsyntax.TokenOQuote:
		{
			// this handle scenarios like:
			// { a = []\nb = [] }

			newTokens = append(newTokens, newlineToken())
			return newTokens, elemIndex
		}
	case hclsyntax.TokenComma, hclsyntax.TokenCBrack, hclsyntax.TokenCParen:
		{

			return newTokens, elemIndex
		}
	case hclsyntax.TokenOBrack:
		{
			// We could have a index access []\n[0] or we could be inside
			// an object, where the newline indicates a new key = value
			// Also we could be inside an object but still have an indexing operation
			// so we use newlines to disambiguate.
			if isInsideObject && hasNewlineBeforeToken {

				newTokens = append(newTokens, newlineToken())
				return newTokens, elemIndex
			}

			indexAccess, nextPos := fmtIndexAccess(tokens[elemIndex:])
			elemIndex += nextPos

			newTokens = append(newTokens, indexAccess...)

			return newTokens, elemIndex
		}
	default:
		{
			// HCL allows all sort of crazy things, instead of mapping all of them
			// we just assume the next token is an operator and the rest can be any expression
			newTokens = append(newTokens, tokens[elemIndex])
			elemIndex++

			skipNls()
			operand, nextPos := fmtExpr(tokens[elemIndex:])
			elemIndex += nextPos

			newTokens = append(newTokens, operand...)
			return newTokens, elemIndex
		}
	}
}

func fmtIndexAccess(tokens hclwrite.Tokens) (hclwrite.Tokens, int) {
	// This function expects that `tokens` represent a index access.
	// Any possible lists inside an index will not be reformatted.
	// It will navigate the tokens until if finds the end of the index access chain.
	// eg: var = [ "item" ][0].name.hi[1],
	openBrackets := 0
	openBraces := 0
	openParens := 0

	for i, token := range tokens {
		switch token.Type {
		case hclsyntax.TokenOParen:
			openParens++
		case hclsyntax.TokenCParen:
			openParens--
		case hclsyntax.TokenOBrace:
			openBraces++
		case hclsyntax.TokenCBrace:
			openBraces--
		case hclsyntax.TokenOBrack:
			openBrackets++
		case hclsyntax.TokenCBrack:
			openBrackets--
			// openBrackets is -1 means we reached the end of an outer list
			// Don't need to check other open delimiters in this case
			// unless the code was originally malformed, but that should not
			// be possible here.
			if openBrackets == -1 {
				return tokens[0:i], i
			}
		case hclsyntax.TokenComma:
			if openBrackets == 0 && openParens == 0 && openBraces == 0 {
				return tokens[0:i], i
			}
		}
	}
	// We could be at the end of the whole attribute expression
	// For example:
	// a = ["list"][0]
	// The index access is formatted here, and it will go all the way
	// until the end of the attribute expression.
	return tokens, len(tokens)
}

// fmtExpr will try to format overall expressions, handling if
// there are lists inside.
//
// When finding a comma it may stop formatting the expression and return
// the tokens + position of the comma, since it is use to format expressions
// inside lists (we need to format each expression before/after commas individually).
func fmtExpr(tokens hclwrite.Tokens) (hclwrite.Tokens, int) {
	newTokens := make(hclwrite.Tokens, 0, len(tokens))
	elemIndex := 0
	openBrackets := 0
	openBraces := 0
	openParens := 0
	openStrTemplate := 0

	curTokenStartsList := func() bool {
		// Current token already == [. Lets do further checking.
		if openStrTemplate > 0 {
			// Inside string template, ignore anything that looks like a list
			return false
		}
		if isListComprehension(tokens[elemIndex:]) {
			return false
		}
		if elemIndex == 0 {
			// Cant backtrack, so assuming a list
			return true
		}

		var previousToken *hclwrite.Token

		// Skipping newlines/comments in reverse, HCL is fun !
		// Handling things like this: "[0\n[[]]]" (and also including comments)
		for i := elemIndex - 1; i >= 0; i-- {
			previousToken = tokens[i]
			if previousToken.Type != hclsyntax.TokenNewline &&
				previousToken.Type != hclsyntax.TokenComment {
				break
			}
		}

		switch previousToken.Type {
		case hclsyntax.TokenCBrace, hclsyntax.TokenCBrack,
			hclsyntax.TokenIdent, hclsyntax.TokenQuotedLit,
			hclsyntax.TokenStringLit, hclsyntax.TokenNumberLit,
			hclsyntax.TokenCParen, hclsyntax.TokenCQuote,
			hclsyntax.TokenCHeredoc, hclsyntax.TokenStar:
			// Indexing, not an actual list
			return false
		default:
			return true
		}
	}

	addToken := func(token *hclwrite.Token) {
		newTokens = append(newTokens, token)
		elemIndex++
	}

	for elemIndex < len(tokens) {
		token := tokens[elemIndex]

		switch token.Type {
		case hclsyntax.TokenOParen:
			addToken(token)
			openParens++
		case hclsyntax.TokenTemplateInterp:
			addToken(token)
			openStrTemplate++
		case hclsyntax.TokenTemplateSeqEnd:
			addToken(token)
			openStrTemplate--
		case hclsyntax.TokenCParen:
			addToken(token)
			openParens--
		case hclsyntax.TokenOBrace:
			addToken(token)
			openBraces++
		case hclsyntax.TokenCBrace:
			addToken(token)
			openBraces--
		case hclsyntax.TokenOBrack:
			if curTokenStartsList() {
				listTokens, pos := fmtListExpr(tokens[elemIndex:], openBraces > 0)
				newTokens = append(newTokens, listTokens...)
				elemIndex += pos
			} else {
				addToken(token)
				openBrackets++
			}
		case hclsyntax.TokenCBrack:
			openBrackets--
			// openBrackets is -1 when we reach the end of the outer list
			// Don't need to check other open delimiters in this case
			// unless the code was originally malformed, but that should not
			// be possible here.
			if openBrackets == -1 {
				return newTokens, elemIndex
			}
			addToken(token)
		case hclsyntax.TokenComma:
			if openBrackets == 0 && openParens == 0 && openBraces == 0 && openStrTemplate == 0 {
				return newTokens, elemIndex
			}
			addToken(token)
		default:
			addToken(token)
		}
	}
	// We could be at the end of the whole attribute expression
	// For example:
	// a = ["list"][0]
	// The index access is formatted here, and it will go all the way
	// until the end of the attribute expression.
	return newTokens, len(tokens)
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

	return tokens[start:end]
}

func tokensStr(tokens hclwrite.Tokens) string {
	return string(tokens.Bytes())
}

func skipNewlines(tokens hclwrite.Tokens) (hclwrite.Tokens, int) {
	return skipTokens(tokens, hclsyntax.TokenNewline)
}

func skipTokens(tokens hclwrite.Tokens, skipTypes ...hclsyntax.TokenType) (hclwrite.Tokens, int) {
	for i, token := range tokens {
		skip := false
		for _, skipType := range skipTypes {
			if token.Type == skipType {
				skip = true
				break
			}
		}
		if !skip {
			return tokens[i:], i
		}
	}
	return nil, len(tokens)
}

func isListComprehension(tokens hclwrite.Tokens) bool {
	// Here we already assume the first token is [
	// So we are trying to determine if it is a list comprehension.
	tokens, _ = skipTokens(tokens[1:], hclsyntax.TokenNewline, hclsyntax.TokenComment)
	return tokens[0].Type == hclsyntax.TokenIdent &&
		string(tokens[0].Bytes) == "for"
}

func isHeredoc(tokens hclwrite.Tokens) bool {
	lastToken := tokens[len(tokens)-1]
	return lastToken.Type == hclsyntax.TokenCHeredoc
}
