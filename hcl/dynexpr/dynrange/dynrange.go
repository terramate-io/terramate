// Copyright 2023 Mineiros GmbH
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

// Package dynrange provides helper functions for dealing with dynamic generated
// range for expressions.
// Note: This is package must not import other Terramate packages because it's
// imported by the "errors" package, which makes this package now the leaf of the
// import tree of Terramate.
// This package must be deleted when we remove the need for injection expression
// bytes in hcl.Expression.
package dynrange

import (
	"bytes"
	"fmt"

	"github.com/hashicorp/hcl/v2"
)

// WrapExprBytes wraps the exprBytes provided in a string suited for a dynamically
// generated expression.
func WrapExprBytes(exprBytes []byte) string {
	fnameBytes := append(injectedTokensPrefix(), exprBytes...)
	fnameBytes = append(fnameBytes, injectedTokenSuffix()...)
	return string(fnameBytes)
}

// UnwrapExprBytes unwraps the expression bytes from the input.
func UnwrapExprBytes(input string) ([]byte, bool) {
	if !HasExprBytesWrapped(input) {
		return nil, false
	}

	content := []byte(input)
	exprStartPos := len(injectedTokensPrefix())
	endEndPos := bytes.Index(content[exprStartPos:], injectedTokenSuffix())
	return content[exprStartPos : exprStartPos+endEndPos], true
}

// HasExprBytesWrapped tells if the content is a dynamic expression wrapper buffer.
func HasExprBytesWrapped(content string) bool {
	return bytes.Contains([]byte(content), injectedTokensPrefix())
}

// ReplaceInjectedExpr replaces all occurrences of injected bytes with replace.
func ReplaceInjectedExpr(content, replace string) string {
	target := []byte(content)
	startPos := bytes.Index([]byte(target), injectedTokensPrefix())
	if startPos == -1 {
		return content
	}

	before := target[0:startPos]
	injection := target[startPos:]
	endPos := len(injectedTokensPrefix()) - 1
	exprBegin := injection[endPos+1:]
	endPos = bytes.Index(exprBegin, injectedTokenSuffix())
	res := []byte{}
	res = append(res, before...)
	res = append(res, []byte(replace)...)
	res = append(res, exprBegin[endPos+1:]...)

	content = string(res)
	if bytes.Contains(res, injectedTokensPrefix()) {
		return ReplaceInjectedExpr(content, replace)
	}
	return content
}

// HideInjectedExpr returns a placeholder for the injected bytes. This is useful when
// presenting strings which could have generated expressions.
func HideInjectedExpr(content string) string {
	const dynfilename = "<generated expression>"
	return ReplaceInjectedExpr(content, dynfilename)
}

// Fixup cleans the provided range by hiding any injected bytes.
func Fixup(r hcl.Range) hcl.Range {
	cleaned := hcl.Range{
		Start: r.Start,
		End:   r.End,
	}

	cleaned.Filename = HideInjectedExpr(r.Filename)
	return cleaned
}

func injectedTokensPrefix() []byte {
	const bugMessage = `
If you are seeing this, it means Terramate has a bug.
It's harmless but please report it at https://github.com/mineiros-io/terramate/issues
`
	prefix := []byte{0}
	prefix = append(prefix, []byte(fmt.Sprintf("<%s>", bugMessage))...)
	prefix = append(prefix, 0)
	return prefix
}

func injectedTokenSuffix() []byte {
	return []byte{0}
}
