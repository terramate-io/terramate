// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tmls

import (
	"context"
	"flag"
	"fmt"
	"io"

	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate"
	"go.lsp.dev/jsonrpc2"
)

const (
	defaultLogLevel = "info"
	defaultLogFmt   = "text"
)

var (
	modeFlag     = flag.String("mode", "stdio", "communication mode (stdio)")
	versionFlag  = flag.Bool("version", false, "print version and exit")
	logLevelFlag = flag.String(
		"log-level", defaultLogLevel,
		"Log level to use: 'trace', 'debug', 'info', 'warn', 'error', or 'fatal'",
	)
	logFmtFlag = flag.String(
		"log-fmt", defaultLogFmt,
		"Log format to use: 'console', 'text', or 'json'.",
	)

	defaultLogWriter = os.Stderr
)

// RunServer runs the server as a standalone binary. It should be invoked from main directly,
// as it will parse arguments and set up global logging.
func RunServer(opts ...Option) {
	flag.Parse()

	if *versionFlag {
		fmt.Println(terramate.Version())
		os.Exit(0)
	}

	// TODO(i4k): implement other modes.
	if *modeFlag != "stdio" {
		fmt.Println("terramate-ls only supports stdio mode")
		os.Exit(1)
	}

	configureLogging(*logLevelFlag, *logFmtFlag, defaultLogWriter)
	runServer(&readWriter{os.Stdin, os.Stdout}, opts...)
}

func runServer(conn io.ReadWriteCloser, opts ...Option) {
	logger := log.With().
		Str("action", "main.runServer()").
		Logger()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill, syscall.SIGTERM)
	defer stop()

	logger.Info().
		Str("mode", *modeFlag).
		Msg("Starting Terramate Language Server")

	rpcConn := jsonrpc2.NewConn(jsonrpc2.NewStream(conn))
	server := NewServer(rpcConn, opts...)

	rpcConn.Go(ctx, server.Handler)
	<-rpcConn.Done()
}

type readWriter struct {
	io.Reader
	io.Writer
}

func (s *readWriter) Close() error { return nil }

func configureLogging(logLevel string, logFmt string, output io.Writer) {
	switch logLevel {
	case "trace", "debug", "info", "warn", "error", "fatal":
		zloglevel, err := zerolog.ParseLevel(logLevel)

		if err != nil {
			_, _ = fmt.Fprintf(defaultLogWriter, "error: failed to parse -log-level=%s\n", logLevel)
			os.Exit(1)
		}

		zerolog.SetGlobalLevel(zloglevel)
	default:
		_, _ = fmt.Fprintf(defaultLogWriter, "error: log level %q not supported\n", logLevel)
		os.Exit(1)
	}

	switch logFmt {
	case "json":
		zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
		log.Logger = log.Output(output)
	case "text": // no color
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: output, NoColor: true, TimeFormat: time.RFC3339})
	default: // default: console mode using color
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: output, NoColor: false, TimeFormat: time.RFC3339})
	}
}
