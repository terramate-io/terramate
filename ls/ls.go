// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package tmls implements a Terramate Language Server (LSP) providing:
// - Go to Definition: Navigate to symbol definitions including globals, lets, and stack attributes
// - Find References: Locate all references to a symbol across the workspace
// - Rename Symbol: Rename symbols with workspace-wide refactoring support
// - Import Resolution: Follows import chains for global variable resolution
package tmls

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl"
	"go.lsp.dev/jsonrpc2"
	lsp "go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// MethodExecuteCommand is the LSP method name for invoking server commands.
const MethodExecuteCommand = "workspace/executeCommand"

// Server is the Language Server.
type Server struct {
	conn       jsonrpc2.Conn
	workspaces []string
	handlers   handlers

	hclOptions []hcl.Option

	// documents stores open document content by file path
	documents   map[string][]byte
	documentsMu sync.RWMutex

	log zerolog.Logger
}

// handler is a jsonrpc2.Handler with a custom logger.
type handler = func(
	ctx context.Context,
	reply jsonrpc2.Replier,
	req jsonrpc2.Request,
	log zerolog.Logger,
) error

type handlers map[string]handler

// Option type for the language server.
type Option func(*Server)

// NewServer creates a new language server.
func NewServer(conn jsonrpc2.Conn, opts ...Option) *Server {
	s := &Server{
		conn:      conn,
		documents: make(map[string][]byte),
		log:       log.Logger,
	}

	for _, opt := range opts {
		opt(s)
	}

	s.buildHandlers()
	return s
}

// WithLogger sets a custom logger.
func WithLogger(l zerolog.Logger) Option {
	return func(s *Server) {
		s.log = l
	}
}

// WithHCLOptions sets the HCL parser options.
func WithHCLOptions(hclOpts ...hcl.Option) Option {
	return func(s *Server) {
		s.hclOptions = hclOpts
	}
}

// getDocumentContent returns the content of an open document, or reads from disk if not cached
func (s *Server) getDocumentContent(fname string) ([]byte, error) {
	s.documentsMu.RLock()
	content, ok := s.documents[fname]
	s.documentsMu.RUnlock()

	if ok {
		s.log.Debug().Str("file", fname).Msg("using cached document content")
		return content, nil
	}

	s.log.Debug().Str("file", fname).Msg("reading document from disk (not cached)")
	return os.ReadFile(fname)
}

// setDocumentContent stores the content of an open document
func (s *Server) setDocumentContent(fname string, content []byte) {
	s.documentsMu.Lock()
	s.documents[fname] = content
	s.documentsMu.Unlock()
	s.log.Debug().Str("file", fname).Int("size", len(content)).Msg("cached document content")
}

// deleteDocumentContent removes a document from the cache
func (s *Server) deleteDocumentContent(fname string) {
	s.documentsMu.Lock()
	delete(s.documents, fname)
	s.documentsMu.Unlock()
	s.log.Debug().Str("file", fname).Msg("removed document from cache")
}

func (s *Server) buildHandlers() {
	s.handlers = map[string]handler{
		lsp.MethodInitialize:                s.handleInitialize,
		lsp.MethodInitialized:               s.handleInitialized,
		lsp.MethodTextDocumentDidOpen:       s.handleDocumentOpen,
		lsp.MethodTextDocumentDidChange:     s.handleDocumentChange,
		lsp.MethodTextDocumentDidClose:      s.handleDocumentClose,
		lsp.MethodTextDocumentDidSave:       s.handleDocumentSaved,
		lsp.MethodTextDocumentCompletion:    s.handleCompletion,
		lsp.MethodTextDocumentDefinition:    s.handleDefinition,
		lsp.MethodTextDocumentReferences:    s.handleReferences,
		lsp.MethodTextDocumentRename:        s.handleRename,
		lsp.MethodTextDocumentPrepareRename: s.handlePrepareRename,

		// commands
		MethodExecuteCommand: s.handleExecuteCommand,
	}
}

// Handler handles the client requests.
func (s *Server) Handler(ctx context.Context, reply jsonrpc2.Replier, r jsonrpc2.Request) error {
	logger := s.log.With().
		Str("action", "server.Handler()").
		Strs("workspaces", s.workspaces).
		Str("method", r.Method()).
		Logger()

	logger.Debug().
		RawJSON("params", r.Params()).
		Msg("handling request.")

	if handler, ok := s.handlers[r.Method()]; ok {
		return handler(ctx, reply, r, logger)
	}

	return reply(ctx, nil, jsonrpc2.ErrMethodNotFound)
}

func (s *Server) handleInitialize(
	ctx context.Context,
	reply jsonrpc2.Replier,
	r jsonrpc2.Request,
	log zerolog.Logger,
) error {
	type workspaceFolder struct {
		URI  string `json:"uri,omitempty"`
		Name string `json:"name,omitempty"`
	}
	type initParams struct {
		ProcessID        int               `json:"processId,omitempty"`
		RootURI          string            `json:"rootUri,omitempty"`
		WorkspaceFolders []workspaceFolder `json:"workspaceFolders,omitempty"`
	}

	var params initParams
	if err := json.Unmarshal(r.Params(), &params); err != nil {
		log.Error().Err(err).Msg("failed to unmarshal params")
		// TODO(i4k): we should check if it's a json.UnmarshallTypeErr or
		// json.UnmarshalFieldError to return jsonrpc2.ErrInvalidParams and
		// json.ErrParse otherwise.
		return jsonrpc2.ErrInvalidParams
	}

	if len(params.WorkspaceFolders) > 0 {
		for _, wsfolder := range params.WorkspaceFolders {
			s.workspaces = append(s.workspaces, uri.New(wsfolder.URI).Filename())
		}
	} else {
		s.workspaces = []string{uri.New(params.RootURI).Filename()}
	}

	err := reply(ctx, lsp.InitializeResult{
		Capabilities: lsp.ServerCapabilities{
			CompletionProvider: &lsp.CompletionOptions{},

			// if we support `goto` definition.
			DefinitionProvider: true,

			// If we support `hover` info.
			HoverProvider: false,

			// if we support finding references
			ReferencesProvider: true,

			// if we support rename
			RenameProvider: &lsp.RenameOptions{
				PrepareProvider: true,
			},

			TextDocumentSync: lsp.TextDocumentSyncOptions{
				// Send all file content on every change (can be optimized later).
				Change: lsp.TextDocumentSyncKindFull,

				// if we want to be notified about open/close of Terramate files.
				OpenClose: true,
				Save: &lsp.SaveOptions{
					// If we want the file content on save,
					IncludeText: false,
				},
			},
		},
	}, nil)

	if err != nil {
		// WHY(i4k): in stdio mode it's impossible to have network issues.
		// TODO(i4k): improve this for the networked server.
		log.Fatal().Err(err).Msg("failed to reply")
	}

	log.Info().Msgf("client connected using workspaces %q", s.workspaces)

	err = s.conn.Notify(ctx, lsp.MethodWindowShowMessage, lsp.ShowMessageParams{
		Message: "connected to terramate-ls",
		Type:    lsp.MessageTypeInfo,
	})

	if err != nil {
		log.Fatal().Err(err).Msg("failed to notify client")
	}
	return nil
}

func (s *Server) handleInitialized(
	ctx context.Context,
	reply jsonrpc2.Replier,
	_ jsonrpc2.Request,
	_ zerolog.Logger,
) error {
	return reply(ctx, nil, nil)
}

func (s *Server) handleDocumentOpen(
	ctx context.Context,
	reply jsonrpc2.Replier,
	r jsonrpc2.Request,
	log zerolog.Logger,
) error {
	var params lsp.DidOpenTextDocumentParams
	if err := json.Unmarshal(r.Params(), &params); err != nil {
		log.Error().Err(err).Msg("failed to unmarshal params")
		return jsonrpc2.ErrParse
	}

	fname := params.TextDocument.URI.Filename()
	content := params.TextDocument.Text

	// Cache the document content
	s.setDocumentContent(fname, []byte(content))

	return s.checkAndReply(ctx, reply, fname, content)
}

func (s *Server) handleDocumentChange(
	ctx context.Context,
	reply jsonrpc2.Replier,
	r jsonrpc2.Request,
	log zerolog.Logger,
) error {
	var params lsp.DidChangeTextDocumentParams
	if err := json.Unmarshal(r.Params(), &params); err != nil {
		log.Error().Err(err).Msg("failed to unmarshal params")
		return err
	}

	if len(params.ContentChanges) != 1 {
		err := fmt.Errorf("expected content changes = 1, got = %d", len(params.ContentChanges))
		log.Error().Err(err).Send()
		return err
	}

	content := params.ContentChanges[0].Text
	fname := params.TextDocument.URI.Filename()

	// Update cached document content
	s.setDocumentContent(fname, []byte(content))

	return s.checkAndReply(ctx, reply, fname, content)
}

func (s *Server) handleDocumentClose(
	ctx context.Context,
	reply jsonrpc2.Replier,
	r jsonrpc2.Request,
	log zerolog.Logger,
) error {
	var params lsp.DidCloseTextDocumentParams
	if err := json.Unmarshal(r.Params(), &params); err != nil {
		log.Error().Err(err).Msg("failed to unmarshal params")
		return jsonrpc2.ErrParse
	}

	fname := params.TextDocument.URI.Filename()

	// Remove document from cache to prevent memory leak
	s.deleteDocumentContent(fname)

	return reply(ctx, nil, nil)
}

func (s *Server) handleDocumentSaved(
	ctx context.Context,
	reply jsonrpc2.Replier,
	r jsonrpc2.Request,
	log zerolog.Logger,
) error {
	var params lsp.DidSaveTextDocumentParams
	if err := json.Unmarshal(r.Params(), &params); err != nil {
		log.Error().Err(err).Msg("failed to unmarshal params")
		return jsonrpc2.ErrParse
	}

	fname := params.TextDocument.URI.Filename()
	content, err := os.ReadFile(fname)
	if err != nil {
		log.Error().Err(err).Msg("reading saved file.")
		return nil
	}

	return s.checkAndReply(ctx, reply, fname, string(content))
}

// sendErrorDiagnostics sends diagnostics for each provided file, the ones with
// no reported error gets an empty list of diagnostics, so the editor can clean
// up its problems panel for it.
func (s *Server) sendErrorDiagnostics(ctx context.Context, files []string, err error) error {
	errs := errors.L()
	switch e := err.(type) {
	case *errors.Error:
		// an error can wrap a list of errors.
		errs = e.AsList()
	case *errors.List:
		errs = e
	default:
		if err != nil {
			log.Debug().Err(err).Msg("unknown error ignored because it doesn't provide file range")
		}
	}

	diagsMap := map[string][]lsp.Diagnostic{}
	for _, filename := range files {
		diagsMap[filename] = []lsp.Diagnostic{}
	}

	for _, err := range errs.Errors() {
		e, ok := err.(*errors.Error)
		if !ok || e.FileRange.Empty() {
			log.Debug().Err(err).Msg("ignoring error without metadata")
			continue
		}

		log.Debug().Str("error", e.Detailed()).Msg("sending diagnostics")

		filename := e.FileRange.Filename
		fileRange := lsp.Range{}
		fileRange.Start.Line = uint32(e.FileRange.Start.Line) - 1
		fileRange.Start.Character = uint32(e.FileRange.Start.Column) - 1
		fileRange.End.Line = uint32(e.FileRange.End.Line) - 1
		fileRange.End.Character = uint32(e.FileRange.End.Column) - 1

		diagsMap[filename] = append(diagsMap[filename], lsp.Diagnostic{
			Message:  e.Message(),
			Range:    fileRange,
			Severity: lsp.DiagnosticSeverityError,
			Source:   "terramate",
		})
	}

	for _, filename := range files {
		diags := diagsMap[filename]
		filePath := lsp.URI(uri.File(filepath.ToSlash(filename)))
		s.sendDiagnostics(ctx, filePath, diags)
	}

	return nil
}

func (s *Server) handleCompletion(
	ctx context.Context,
	reply jsonrpc2.Replier,
	r jsonrpc2.Request,
	log zerolog.Logger,
) error {
	var params lsp.CompletionParams
	if err := json.Unmarshal(r.Params(), &params); err != nil {
		log.Error().Err(err).Msg("failed to unmarshal params")
		return jsonrpc2.ErrParse
	}
	log.Debug().Str("params", string(r.Params()))
	return reply(ctx, nil, nil)
}

func (s *Server) sendDiagnostics(ctx context.Context, uri lsp.URI, diags []lsp.Diagnostic) {
	err := s.conn.Notify(ctx, lsp.MethodTextDocumentPublishDiagnostics, lsp.PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: diags,
	})

	if err != nil {
		log.Fatal().Err(err).Msg("failed to send diagnostics to the client.")
	}
}

func (s *Server) checkAndReply(
	ctx context.Context,
	reply jsonrpc2.Replier,
	fname string,
	content string,
) error {
	files, err := listFiles(fname)
	files = append(files, fname)
	sort.Strings(files)
	if err == nil {
		err = s.checkFiles(files, fname, content)
	}

	return reply(ctx, nil,
		s.sendErrorDiagnostics(ctx, files, err),
	)
}

func listFiles(fromFile string) ([]string, error) {
	dir := filepath.Dir(fromFile)
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	files := []string{}
	for _, dirEntry := range dirEntries {
		if dirEntry.IsDir() {
			continue
		}

		filename := dirEntry.Name()
		if isTerramateFile(filename) {
			path := filepath.Join(dir, filename)

			if path == fromFile {
				// ignore source file
				continue
			}

			files = append(files, path)
		}
	}

	return files, nil
}

func (s *Server) findWorkspaceForDir(dir string) (string, error) {
	for _, ws := range s.workspaces {
		if dir == ws || strings.HasPrefix(dir, ws+string(filepath.Separator)) {
			return ws, nil
		}
	}
	return "", errors.E("dir '%s' is not in any of workspaces '%v'", dir, s.workspaces)
}

// checkFiles checks if the given provided files have errors but the currentFile
// is handled separately because it can be unsaved.
func (s *Server) checkFiles(files []string, currentFile string, currentContent string) error {
	dir := filepath.Dir(currentFile)
	var experiments []string
	root, rootdir, found, err := config.TryLoadConfig(dir, false)
	if !found {
		var err error
		rootdir, err = s.findWorkspaceForDir(dir)
		if err != nil {
			return err
		}
	} else if err == nil {
		experiments = root.Tree().Node.Experiments()
	}
	opts := []hcl.Option{hcl.WithExperiments(experiments...)}
	opts = append(opts, s.hclOptions...)

	parser, err := hcl.NewTerramateParser(rootdir, dir, opts...)
	if err != nil {
		return errors.E(err, "failed to create terramate parser")
	}

	for _, fname := range files {
		var (
			contents []byte
			err      error
		)

		if currentFile == fname {
			contents = []byte(currentContent)
		} else {
			contents, err = os.ReadFile(fname)
		}

		if err != nil {
			return err
		}

		err = parser.AddFileContent(fname, contents)
		if err != nil {
			return err
		}
	}

	log.Debug().Msg("about to parse all the files")
	_, err = parser.ParseConfig()
	return err
}
