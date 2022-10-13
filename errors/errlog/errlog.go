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
	"os"
	"strings"

	"github.com/mineiros-io/terramate/errors"
	"github.com/rs/zerolog"
)

// Fatal logs the error as a Fatal if the error is not nil.
// If the error is nil this is a no-op.
func Fatal(logger zerolog.Logger, msg string, err error) {
	if err == nil {
		return
	}

	logerrs(logger, zerolog.FatalLevel, zerolog.ErrorLevel, msg, err)
	// zerolog does not call os.Exit if you pass the fatal level
	// with WithLevel, it only aborts when calling the Fatal() method.
	// So we need to ensure the exit 1 here.
	os.Exit(1)
}

// Warn logs the error as a warning if the error is not nil.
// If the error is nil this is a no-op.
func Warn(logger zerolog.Logger, msg string, err error) {
	if err == nil {
		return
	}

	logerrs(logger, zerolog.WarnLevel, zerolog.WarnLevel, msg, err)
}

func logerrs(logger zerolog.Logger, level, childlevel zerolog.Level,
	msg string, err error) {

	var list *errors.List

	if errors.As(err, &list) {
		errs := list.Errors()
		for _, err := range errs {
			logerr(logger, childlevel, "", err)
		}
		logger.WithLevel(level).Msg(msg)
		return
	}

	logerr(logger, level, msg, err)
}

func logerr(
	logger zerolog.Logger,
	level zerolog.Level,
	msg string,
	err error,
) {
	var tmerr *errors.Error
	if !errors.As(err, &tmerr) {
		logger.WithLevel(level).Msgf("%s: %s", msg, err)
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
