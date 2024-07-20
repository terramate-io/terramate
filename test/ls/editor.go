// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package ls

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/madlambda/spells/assert"
	"github.com/rs/zerolog"
	tmls "github.com/terramate-io/terramate/ls"
	"github.com/terramate-io/terramate/test/sandbox"
	"go.lsp.dev/jsonrpc2"
	lsp "go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// Editor is the editor server.
type Editor struct {
	t       *testing.T
	sandbox sandbox.S
	conn    jsonrpc2.Conn

	// Requests that arrived at the editor.
	Requests chan jsonrpc2.Request
}

// NewEditor creates a new editor server.
func NewEditor(t *testing.T, s sandbox.S, conn jsonrpc2.Conn) *Editor {
	return &Editor{
		t:        t,
		sandbox:  s,
		conn:     conn,
		Requests: make(chan jsonrpc2.Request),
	}
}

// Handler is the default editor request handler.
func (e *Editor) Handler(ctx context.Context, reply jsonrpc2.Replier, r jsonrpc2.Request) error {
	go func() {
		e.Requests <- r
	}()
	return reply(ctx, nil, nil)
}

func (e *Editor) call(method string, params, result interface{}) (jsonrpc2.ID, error) {
	return e.conn.Call(context.Background(), method, params, result)
}

// Initialize sends a initialize request to the language server and return its
// result.
func (e *Editor) Initialize(workspace string) lsp.InitializeResult {
	e.t.Helper()
	var got lsp.InitializeResult
	_, err := e.call(
		lsp.MethodInitialize,
		lsp.InitializeParams{
			RootURI: uri.File(workspace),
		},
		&got)

	assert.NoError(e.t, err, "calling %q", lsp.MethodInitialize)
	return got
}

// CheckInitialize sends an initialize request to the language server and checks
// if the response is the expected default response (See DefaultInitializeResult).
func (e *Editor) CheckInitialize(workspace string) {
	e.t.Helper()
	got := e.Initialize(workspace)
	if diff := cmp.Diff(got, DefaultInitializeResult()); diff != "" {
		e.t.Fatalf("init result differs, got(-) want(+):\n%s", diff)
	}

	gotReq := <-e.Requests
	assert.EqualStrings(e.t, lsp.MethodWindowShowMessage, gotReq.Method())
	gotParams := lsp.ShowMessageParams{}
	assert.NoError(e.t, json.Unmarshal(gotReq.Params(), &gotParams))
	if lsp.MessageTypeInfo != gotParams.Type {
		e.t.Fatalf("message type got %v != want %v", gotParams.Type, lsp.MessageTypeInfo)
	}
}

// Open sends a didOpen request to the language server.
func (e *Editor) Open(path string) {
	t := e.t
	t.Helper()
	abspath := filepath.Join(e.sandbox.RootDir(), path)
	fileContents, err := os.ReadFile(abspath)
	assert.NoError(t, err, "reading stack file %q", path)
	var openResult interface{}
	_, err = e.call(lsp.MethodTextDocumentDidOpen, lsp.DidOpenTextDocumentParams{
		TextDocument: lsp.TextDocumentItem{
			URI:        uri.File(abspath),
			LanguageID: "terramate",
			Text:       string(fileContents),
		},
	}, &openResult)
	assert.NoError(t, err, "calling %s", lsp.MethodTextDocumentDidOpen)
	if openResult != nil {
		t.Fatalf("expected nil result but got [%v]", openResult)
	}
}

// Change sends a didChange request to the language server.
func (e *Editor) Change(path, content string) {
	t := e.t
	t.Helper()
	abspath := filepath.Join(e.sandbox.RootDir(), path)
	var changeResult interface{}
	_, err := e.call(lsp.MethodTextDocumentDidChange, lsp.DidChangeTextDocumentParams{
		TextDocument: lsp.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: lsp.TextDocumentIdentifier{
				URI: uri.File(abspath),
			},
		},
		ContentChanges: []lsp.TextDocumentContentChangeEvent{
			{
				Text: content,
			},
		},
	}, &changeResult)
	assert.NoError(t, err, "call %q", lsp.MethodTextDocumentDidChange)
}

// Format send a MethodTextDocumentFormatting event to the language server.
func (e *Editor) Format(path string) ([]lsp.TextEdit, error) {
	t := e.t
	t.Helper()
	abspath := filepath.Join(e.sandbox.RootDir(), path)
	var edits []lsp.TextEdit
	_, err := e.call(lsp.MethodTextDocumentFormatting, lsp.DocumentFormattingParams{
		TextDocument: lsp.TextDocumentIdentifier{
			URI: uri.File(abspath),
		},
	}, &edits)

	return edits, err
}

// Command invokes the provided command in the LSP server.
func (e *Editor) Command(cmd lsp.ExecuteCommandParams) (interface{}, error) {
	t := e.t
	t.Helper()
	var cmdResult interface{}
	_, err := e.call(tmls.MethodExecuteCommand, cmd, &cmdResult)
	return cmdResult, err
}

// DefaultInitializeResult is the default server response for the initialization
// request.
func DefaultInitializeResult() lsp.InitializeResult {
	return lsp.InitializeResult{
		Capabilities: lsp.ServerCapabilities{
			CompletionProvider: &lsp.CompletionOptions{},
			DefinitionProvider: false,
			HoverProvider:      false,
			TextDocumentSync: map[string]interface{}{
				"change":    float64(1),
				"openClose": true,
				"save":      map[string]interface{}{},
			},
			DocumentFormattingProvider: true,
		},
	}
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
