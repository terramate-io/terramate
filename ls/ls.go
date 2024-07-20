// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package tmls implements a Terramate Language Server (LSP).
package tmls

import (
	"context"
	"encoding/json"
	stdfmt "fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/hcl/fmt"
	"go.lsp.dev/jsonrpc2"
	lsp "go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// MethodExecuteCommand is the LSP method name for invoking server commands.
const MethodExecuteCommand = "workspace/executeCommand"

// Server is the Language Server.
type Server struct {
	conn      jsonrpc2.Conn
	workspace string
	handlers  handlers

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

// NewServer creates a new language server.
func NewServer(conn jsonrpc2.Conn) *Server {
	return ServerWithLogger(conn, log.Logger)
}

// ServerWithLogger creates a new language server with a custom logger.
func ServerWithLogger(conn jsonrpc2.Conn, l zerolog.Logger) *Server {
	s := &Server{
		conn: conn,
		log:  l,
	}
	s.buildHandlers()
	return s
}

func (s *Server) buildHandlers() {
	s.handlers = map[string]handler{
		lsp.MethodInitialize:             s.handleInitialize,
		lsp.MethodInitialized:            s.handleInitialized,
		lsp.MethodTextDocumentDidOpen:    s.handleDocumentOpen,
		lsp.MethodTextDocumentDidChange:  s.handleDocumentChange,
		lsp.MethodTextDocumentDidSave:    s.handleDocumentSaved,
		lsp.MethodTextDocumentCompletion: s.handleCompletion,
		lsp.MethodTextDocumentFormatting: s.handleFormatting,

		// commands
		MethodExecuteCommand: s.handleExecuteCommand,
	}
}

// Handler handles the client requests.
func (s *Server) Handler(ctx context.Context, reply jsonrpc2.Replier, r jsonrpc2.Request) error {
	logger := s.log.With().
		Str("action", "server.Handler()").
		Str("workspace", s.workspace).
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
	type initParams struct {
		ProcessID int    `json:"processId,omitempty"`
		RootURI   string `json:"rootUri,omitempty"`
	}

	var params initParams
	if err := json.Unmarshal(r.Params(), &params); err != nil {
		log.Error().Err(err).Msg("failed to unmarshal params")
		// TODO(i4k): we should check if it's a json.UnmarshallTypeErr or
		// json.UnmarshalFieldError to return jsonrpc2.ErrInvalidParams and
		// json.ErrParse otherwise.
		return jsonrpc2.ErrInvalidParams
	}

	s.workspace = string(uri.New(params.RootURI).Filename())
	err := reply(ctx, lsp.InitializeResult{
		Capabilities: lsp.ServerCapabilities{
			DocumentFormattingProvider: true,
			CompletionProvider:         &lsp.CompletionOptions{},

			// if we support `goto` definition.
			DefinitionProvider: false,

			// If we support `hover` info.
			HoverProvider: false,

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

	log.Info().Msgf("client connected using workspace %q", s.workspace)

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
		err := stdfmt.Errorf("expected content changes = 1, got = %d", len(params.ContentChanges))
		log.Error().Err(err).Send()
		return err
	}

	content := params.ContentChanges[0].Text
	fname := params.TextDocument.URI.Filename()

	return s.checkAndReply(ctx, reply, fname, content)
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

func (s *Server) handleFormatting(
	ctx context.Context,
	reply jsonrpc2.Replier,
	r jsonrpc2.Request,
	log zerolog.Logger,
) error {
	var params lsp.DocumentFormattingParams
	if err := json.Unmarshal(r.Params(), &params); err != nil {
		log.Error().Err(err).Msg("failed to unmarshal params")
		return jsonrpc2.ErrParse
	}
	log.Debug().Str("params", string(r.Params()))
	fname := params.TextDocument.URI.Filename()
	content, err := os.ReadFile(fname)
	if err != nil {
		log.Error().Err(err).Msg("failed to read file for formatting")
		return reply(ctx, nil, err)
	}
	formatted, err := fmt.Format(string(content), fname)
	if err != nil {
		log.Error().Err(err).Msg("failed to format file")
		return reply(ctx, nil, err)
	}
	log.Info().Msgf("formatted:'%s'", formatted)
	oldlines := strings.Split(string(content), "\n")

	var textedits []lsp.TextEdit
	textedits = append(textedits,
		// remove old content
		lsp.TextEdit{
			NewText: "",
			Range: lsp.Range{
				Start: lsp.Position{
					Line:      0,
					Character: 0,
				},
				End: lsp.Position{
					Line:      uint32(len(oldlines) - 1),
					Character: uint32(len(oldlines[len(oldlines)-1])),
				},
			},
		})

	newlines := strings.Split(string(formatted), "\n")
	for i, line := range newlines {
		textedits = append(textedits, lsp.TextEdit{
			NewText: line,
			Range: lsp.Range{
				Start: lsp.Position{
					Line:      uint32(i),
					Character: 0,
				},
				End: lsp.Position{
					Line:      uint32(i),
					Character: uint32(len(line) - 1),
				},
			},
		})
	}

	return reply(ctx, textedits, nil)
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
		if strings.HasSuffix(filename, ".tm") || strings.HasSuffix(filename, ".tm.hcl") {
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

// checkFiles checks if the given provided files have errors but the currentFile
// is handled separately because it can be unsaved.
func (s *Server) checkFiles(files []string, currentFile string, currentContent string) error {
	dir := filepath.Dir(currentFile)
	var experiments []string
	root, rootdir, found, err := config.TryLoadConfig(dir)
	if !found {
		rootdir = s.workspace
	} else if err == nil {
		experiments = root.Tree().Node.Experiments()
	}

	parser, err := hcl.NewTerramateParser(rootdir, dir, experiments...)
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
