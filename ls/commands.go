// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tmls

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/stack"
	"go.lsp.dev/jsonrpc2"
	lsp "go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// Errors created by the commands handling.
const (
	ErrUnrecognizedCommand        errors.Kind = "terramate-ls: unknown command"
	ErrCreateStackUnrecognizedArg errors.Kind = "terramate-ls: terramate.createStack: unrecognized argument"
	ErrCreateStackNoArguments     errors.Kind = "terramate-ls: terramate.createStack requires at least 1 argument"
	ErrCreateStackInvalidArgument errors.Kind = "terramate-ls: terramate.createStack: invalid argument"
	ErrCreateStackMissingRequired errors.Kind = "terramate-ls: terramate.createStack: missing required argument"
	ErrCreateStackFailed          errors.Kind = "terramate-ls: terramate.createStack failed"
)

func (s *Server) handleExecuteCommand(
	ctx context.Context,
	reply jsonrpc2.Replier,
	r jsonrpc2.Request,
	log zerolog.Logger,
) error {
	var params lsp.ExecuteCommandParams
	if err := json.Unmarshal(r.Params(), &params); err != nil {
		log.Error().Err(err).Msg("failed to unmarshal params")
		return jsonrpc2.ErrParse
	}

	switch params.Command {
	case "terramate.createStack":
		err := s.createStack(params)
		return reply(ctx, nil, err)
	default:
		err := errors.E(ErrUnrecognizedCommand, params.Command)
		log.Error().Err(err).Send()
		return reply(ctx, nil, err)
	}
}

func (s *Server) createStack(params lsp.ExecuteCommandParams) error {
	args := params.Arguments
	if len(args) < 1 {
		return errors.E(ErrCreateStackNoArguments)
	}

	// TODO(i4k): load stack once when handling the initialize method.
	root, err := config.LoadRoot(s.workspace)
	if err != nil {
		return errors.E(err, "loading project root from %s", s.workspace)
	}

	stackConfig := config.Stack{}
	for _, arg := range args {
		strArg, ok := arg.(string)
		if !ok {
			return errors.E(ErrCreateStackInvalidArgument, "%+v", args[0])
		}
		pos := strings.IndexRune(strArg, '=')
		argName := strArg[0:pos]
		argVal := strArg[pos+1:]
		switch argName {
		case "uri":
			dir, err := uri.Parse(argVal)
			if err != nil {
				return errors.E(ErrCreateStackInvalidArgument, err, "failed to parse URI: %s", argVal)
			}

			stackConfig.Dir = project.PrjAbsPath(s.workspace, dir.Filename())
		case "genid":
			id, err := uuid.NewRandom()
			if err != nil {
				return errors.E(ErrCreateStackFailed, err, "generating ID")
			}
			stackConfig.ID = id.String()
		case "name":
			stackConfig.Name = argVal
		case "description":
			stackConfig.Description = argVal
		default:
			return errors.E(ErrCreateStackUnrecognizedArg, strArg)
		}
	}

	if stackConfig.Dir.String() == "" {
		log.Error().Msgf("missing required `uri` argument")
		return errors.E(ErrCreateStackMissingRequired, "`uri` is not set")
	}

	err = stack.Create(root, stackConfig)
	if err != nil {
		log.Error().Err(err).Msg("creating stack")
		return errors.E(ErrCreateStackFailed, err)
	}
	return nil
}
