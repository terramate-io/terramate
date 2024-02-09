// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package errlog provides functions to log Terramate errors nicely and
// in a consistent manner.
package errlog

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog"
	"github.com/terramate-io/terramate/errors"
)

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
		ctx = ctx.Stringer("file", tmerr.FileRange)
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
		panic(errors.E(errors.ErrInternal, "invalid call to errlog.Fatal or errlog.Warn, message is not a string"))
	}
	return fmt.Sprintf(msgfmt, args[1:]...)
}
