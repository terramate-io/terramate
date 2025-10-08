// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tg

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/gruntwork-io/terragrunt/pkg/log"
	"github.com/gruntwork-io/terragrunt/pkg/log/format"
	"github.com/gruntwork-io/terragrunt/pkg/log/format/placeholders"
	"github.com/rs/zerolog"
)

// zeroLogAdapter adapts zerolog.Logger to Terragrunt's pkg/log.Logger interface
type zeroLogAdapter struct {
	logger     zerolog.Logger
	pipeReader *io.PipeReader
	pipeWriter *io.PipeWriter
	formatter  log.Formatter
}

// NewTerragruntLogger creates a Terragrunt-compatible logger from a zerolog logger.
// Returns the logger and a cleanup function that must be called to prevent resource leaks.
func NewTerragruntLogger(logger zerolog.Logger) (log.Logger, func() error) {
	// Create a pipe writer for compatibility with Terragrunt's API
	// We don't actually use it as zerolog handles output internally
	pr, pw := io.Pipe()

	// Discard the reader to prevent blocking writes and resource leaks
	go func() {
		_, _ = io.Copy(io.Discard, pr)
	}()

	// Create a formatter using Terragrunt's default formatter
	formatter := format.NewFormatter(placeholders.NewPlaceholderRegister())
	formatter.SetDisabledColors(true)
	formatter.DisableRelativePaths()
	formatter.SetDisabledOutput(true) // We use io.Discard anyway

	adapter := &zeroLogAdapter{
		logger:     logger,
		pipeReader: pr,
		pipeWriter: pw,
		formatter:  formatter,
	}

	cleanup := func() error {
		return adapter.Close()
	}

	return adapter, cleanup
}

// Close terminates the goroutine and releases resources
func (z *zeroLogAdapter) Close() error {
	var err error
	if z.pipeWriter != nil {
		if closeErr := z.pipeWriter.Close(); closeErr != nil {
			err = closeErr
		}
	}
	if z.pipeReader != nil {
		if closeErr := z.pipeReader.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}
	return err
}

func (z *zeroLogAdapter) Logf(level log.Level, format string, args ...interface{}) {
	switch level {
	case log.DebugLevel:
		z.logger.Debug().Msgf(format, args...)
	case log.InfoLevel:
		z.logger.Info().Msgf(format, args...)
	case log.WarnLevel:
		z.logger.Warn().Msgf(format, args...)
	case log.ErrorLevel:
		z.logger.Error().Msgf(format, args...)
	default:
		z.logger.Info().Msgf(format, args...)
	}
}

func (z *zeroLogAdapter) Log(level log.Level, args ...interface{}) {
	msg := formatArgs(args...)
	switch level {
	case log.DebugLevel:
		z.logger.Debug().Msg(msg)
	case log.InfoLevel:
		z.logger.Info().Msg(msg)
	case log.WarnLevel:
		z.logger.Warn().Msg(msg)
	case log.ErrorLevel:
		z.logger.Error().Msg(msg)
	default:
		z.logger.Info().Msg(msg)
	}
}

func (z *zeroLogAdapter) Logln(level log.Level, args ...interface{}) {
	z.Log(level, args...)
}

func (z *zeroLogAdapter) Debug(args ...interface{}) {
	z.Log(log.DebugLevel, args...)
}

func (z *zeroLogAdapter) Debugf(format string, args ...interface{}) {
	z.Logf(log.DebugLevel, format, args...)
}

func (z *zeroLogAdapter) Debugln(args ...interface{}) {
	z.Log(log.DebugLevel, args...)
}

func (z *zeroLogAdapter) Info(args ...interface{}) {
	z.Log(log.InfoLevel, args...)
}

func (z *zeroLogAdapter) Infof(format string, args ...interface{}) {
	z.Logf(log.InfoLevel, format, args...)
}

func (z *zeroLogAdapter) Infoln(args ...interface{}) {
	z.Log(log.InfoLevel, args...)
}

func (z *zeroLogAdapter) Warn(args ...interface{}) {
	z.Log(log.WarnLevel, args...)
}

func (z *zeroLogAdapter) Warnf(format string, args ...interface{}) {
	z.Logf(log.WarnLevel, format, args...)
}

func (z *zeroLogAdapter) Warnln(args ...interface{}) {
	z.Log(log.WarnLevel, args...)
}

func (z *zeroLogAdapter) Error(args ...interface{}) {
	z.Log(log.ErrorLevel, args...)
}

func (z *zeroLogAdapter) Errorf(format string, args ...interface{}) {
	z.Logf(log.ErrorLevel, format, args...)
}

func (z *zeroLogAdapter) Errorln(args ...interface{}) {
	z.Log(log.ErrorLevel, args...)
}

func (z *zeroLogAdapter) Trace(args ...interface{}) {
	z.Debug(args...)
}

func (z *zeroLogAdapter) Tracef(format string, args ...interface{}) {
	z.Debugf(format, args...)
}

func (z *zeroLogAdapter) Traceln(args ...interface{}) {
	z.Debugln(args...)
}

func (z *zeroLogAdapter) Print(args ...interface{}) {
	z.Info(args...)
}

func (z *zeroLogAdapter) Printf(format string, args ...interface{}) {
	z.Infof(format, args...)
}

func (z *zeroLogAdapter) Println(args ...interface{}) {
	z.Infoln(args...)
}

func (z *zeroLogAdapter) SetLevel(_ string) error {
	// zerolog level is set globally, not per-logger
	return nil
}

func (z *zeroLogAdapter) GetLevel() log.Level {
	switch z.logger.GetLevel() {
	case zerolog.DebugLevel, zerolog.TraceLevel:
		return log.DebugLevel
	case zerolog.InfoLevel:
		return log.InfoLevel
	case zerolog.WarnLevel:
		return log.WarnLevel
	case zerolog.ErrorLevel, zerolog.FatalLevel, zerolog.PanicLevel:
		return log.ErrorLevel
	default:
		return log.InfoLevel
	}
}

func (z *zeroLogAdapter) Level() log.Level {
	return z.GetLevel()
}

func (z *zeroLogAdapter) Writer() *io.PipeWriter {
	return z.pipeWriter
}

func (z *zeroLogAdapter) WriterLevel(_ log.Level) *io.PipeWriter {
	return z.pipeWriter
}

func (z *zeroLogAdapter) SetFormatter(formatter log.Formatter) {
	z.formatter = formatter
}

func (z *zeroLogAdapter) Formatter() log.Formatter {
	return z.formatter
}

func (z *zeroLogAdapter) WithField(key string, value interface{}) log.Logger {
	return &zeroLogAdapter{
		logger:     z.logger.With().Interface(key, value).Logger(),
		pipeReader: z.pipeReader,
		pipeWriter: z.pipeWriter,
		formatter:  z.formatter,
	}
}

func (z *zeroLogAdapter) WithFields(fields log.Fields) log.Logger {
	logger := z.logger.With()
	for k, v := range fields {
		logger = logger.Interface(k, v)
	}
	return &zeroLogAdapter{
		logger:     logger.Logger(),
		pipeReader: z.pipeReader,
		pipeWriter: z.pipeWriter,
		formatter:  z.formatter,
	}
}

func (z *zeroLogAdapter) WithError(err error) log.Logger {
	return &zeroLogAdapter{
		logger:     z.logger.With().Err(err).Logger(),
		pipeReader: z.pipeReader,
		pipeWriter: z.pipeWriter,
		formatter:  z.formatter,
	}
}

func (z *zeroLogAdapter) WithTime(t time.Time) log.Logger {
	return &zeroLogAdapter{
		logger:     z.logger.With().Time("time", t).Logger(),
		pipeReader: z.pipeReader,
		pipeWriter: z.pipeWriter,
		formatter:  z.formatter,
	}
}

func (z *zeroLogAdapter) WithContext(_ context.Context) log.Logger {
	return &zeroLogAdapter{
		logger:     z.logger,
		pipeReader: z.pipeReader,
		pipeWriter: z.pipeWriter,
		formatter:  z.formatter,
	}
}

func (z *zeroLogAdapter) WithOptions(_ ...log.Option) log.Logger {
	return &zeroLogAdapter{
		logger:     z.logger,
		pipeReader: z.pipeReader,
		pipeWriter: z.pipeWriter,
		formatter:  z.formatter,
	}
}

func (z *zeroLogAdapter) SetOptions(_ ...log.Option) {
	// No-op
}

func (z *zeroLogAdapter) Clone() log.Logger {
	return &zeroLogAdapter{
		logger:     z.logger.With().Logger(),
		pipeReader: z.pipeReader,
		pipeWriter: z.pipeWriter,
		formatter:  z.formatter,
	}
}

func formatArgs(args ...interface{}) string {
	return fmt.Sprint(args...)
}
