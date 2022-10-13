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

// Package errlog provides functions to log Terramate errors nicely and
// in a consistent manner.
package errlog

import (
	"fmt"
	"os"
	"strings"

	"github.com/mineiros-io/terramate/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Fatal logs the error as a Fatal if the error is not nil.
func Fatal(logger zerolog.Logger, err error, args ...any) {
	logerrs(logger, zerolog.FatalLevel, zerolog.ErrorLevel, err, args)
	// zerolog does not call os.Exit if you pass the fatal level
	// with WithLevel, it only aborts when calling the Fatal() method.
	// So we need to ensure the exit 1 here.
	os.Exit(1)
}

// Warn logs the error as a warning if the error is not nil.
func Warn(logger zerolog.Logger, err error, args ...any) {
	logerrs(logger, zerolog.WarnLevel, zerolog.WarnLevel, err, args)
}

func logerrs(logger zerolog.Logger, level, childlevel zerolog.Level, err error, args []any) {
	var list *errors.List

	if errors.As(err, &list) {
		errs := list.Errors()
		for _, err := range errs {
			logerr(logger, childlevel, err, nil)
		}
		logger.WithLevel(level).Msg(msgfmt(args))
		return
	}

	logerr(logger, level, err, args)
}

func logerr(
	logger zerolog.Logger,
	level zerolog.Level,
	err error,
	args []any,
) {
	var tmerr *errors.Error
	msg := msgfmt(args)
	if !errors.As(err, &tmerr) {
		msgparts := []string{}
		if msg != "" {
			msgparts = append(msgparts, msg)
		}
		if err != nil {
			msgparts = append(msgparts, err.Error())
		}
		logger.WithLevel(level).Msg(strings.Join(msgparts, ": "))
		return
	}

	ctx := logger.With()
	if !tmerr.FileRange.Empty() {
		ctx.Stringer("file", tmerr.FileRange)
	}
	if tmerr.Stack != nil {
		ctx.Str("stack", tmerr.Stack.Path())
	}

	msgparts := []string{}

	if msg != "" {
		msgparts = append(msgparts, msg)
	}
	if tmerr.Kind != "" {
		msgparts = append(msgparts, string(tmerr.Kind))
	}
	if tmerr.Description != "" {
		msgparts = append(msgparts, tmerr.Description)
	}
	if tmerr.Err != nil {
		msgparts = append(msgparts, tmerr.Err.Error())
	}

	logger = ctx.Logger()
	logger.WithLevel(level).Msg(strings.Join(msgparts, ": "))
}

func msgfmt(args []any) string {
	if len(args) == 0 {
		return ""
	}
	msgfmt, ok := args[0].(string)
	if !ok {
		log.Error().Msg("invalid call to errlog.Fatal or errlog.Warn, message is not a string")
		return ""
	}
	return fmt.Sprintf(msgfmt, args[1:]...)
}
