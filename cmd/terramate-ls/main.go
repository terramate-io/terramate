// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Terramate-ls is a language server.
// For details on how to use it just run:
//
//	terramate-ls --help
package main

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
	tmls "github.com/terramate-io/terramate/ls"
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

func main() {
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
	runServer(&readWriter{os.Stdin, os.Stdout})
}

func runServer(conn io.ReadWriteCloser) {
	logger := log.With().
		Str("action", "main.runServer()").
		Logger()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill, syscall.SIGTERM)
	defer stop()

	logger.Info().
		Str("mode", *modeFlag).
		Msg("Starting Terramate Language Server")

	rpcConn := jsonrpc2.NewConn(jsonrpc2.NewStream(conn))
	server := tmls.NewServer(rpcConn)

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
			fmt.Fprintf(defaultLogWriter, "error: failed to parse -log-level=%s\n", logLevel)
			os.Exit(1)
		}

		zerolog.SetGlobalLevel(zloglevel)
	default:
		fmt.Fprintf(defaultLogWriter, "error: log level %q not supported\n", logLevel)
		os.Exit(1)
	}

	if logFmt == "json" {
		zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
		log.Logger = log.Output(output)
	} else if logFmt == "text" { // no color
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: output, NoColor: true, TimeFormat: time.RFC3339})
	} else { // default: console mode using color
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: output, NoColor: false, TimeFormat: time.RFC3339})
	}
}
