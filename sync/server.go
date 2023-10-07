// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package sync

import (
	"context"
	"encoding/json"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.lsp.dev/jsonrpc2"
)

const MethodStackSetValue = "stack/set"
const MethodStackSetValueOK = "stack/set/ok"

// Server is the Language Server.
type Server struct {
	conn     jsonrpc2.Conn
	handlers handlers

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
		MethodStackSetValue: s.handleStackSetValue,
	}
}

// Handler handles the client requests.
func (s *Server) Handler(ctx context.Context, reply jsonrpc2.Replier, r jsonrpc2.Request) error {
	logger := s.log.With().
		Str("action", "server.Handler()").
		Str("method", r.Method()).
		Logger()

	logger.Info().
		RawJSON("params", r.Params()).
		Msg("handling request.")

	if handler, ok := s.handlers[r.Method()]; ok {
		return handler(ctx, reply, r, logger)
	}

	logger.Trace().Msg("not implemented")
	return reply(ctx, nil, jsonrpc2.ErrMethodNotFound)
}

type RequestSetStackValue struct {
	StackPath string      `json:"stack_path"`
	Name      string      `json:"name"`
	Value     interface{} `json:"value"`
}

func (s *Server) handleStackSetValue(
	ctx context.Context,
	reply jsonrpc2.Replier,
	r jsonrpc2.Request,
	log zerolog.Logger,
) error {

	var params RequestSetStackValue
	if err := json.Unmarshal(r.Params(), &params); err != nil {
		log.Error().Err(err).Msg("failed to unmarshal params")
		// TODO(i4k): we should check if it's a json.UnmarshallTypeErr or
		// json.UnmarshalFieldError to return jsonrpc2.ErrInvalidParams and
		// json.ErrParse otherwise.
		return jsonrpc2.ErrInvalidParams
	}

	err := reply(ctx, true, nil)
	if err != nil {
		// WHY(i4k): in stdio mode it's impossible to have network issues.
		// TODO(i4k): improve this for the networked server.
		log.Error().Err(err).Msg("failed to reply")
		return err
	}

	log.Info().Msgf("client request handled for stack %s", params.StackPath)

	s.conn.Close()

	return nil
}
