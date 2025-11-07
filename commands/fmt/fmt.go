// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package fmt provides the fmt command.
package fmt

import (
	"context"
	stdfmt "fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/commands"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/exit"
	"github.com/terramate-io/terramate/hcl/fmt"
	"github.com/terramate-io/terramate/printer"
)

// Spec is the command specification for the fmt command.
type Spec struct {
	Check            bool
	DetailedExitCode bool
	Files            []string

	workingDir string
	printers   printer.Printers
}

// Name returns the name of the command.
func (s *Spec) Name() string { return "fmt" }

// Requirements returns the requirements of the command.
func (s *Spec) Requirements(context.Context, commands.CLI) any { return commands.RequireEngine() }

// Exec executes the fmt command.
func (s *Spec) Exec(_ context.Context, cli commands.CLI) error {
	s.workingDir = cli.WorkingDir()
	s.printers = cli.Printers()

	logger := log.With().
		Str("action", "commands/fmt").
		Str("workingDir", s.workingDir).
		Bool("check", s.Check).
		Bool("detailed-exit-code", s.DetailedExitCode).
		Strs("files", s.Files).
		Logger()

	logger.Debug().Msgf("executing %s", s.Name())

	if s.Check && s.DetailedExitCode {
		return errors.E("fmt --check conflicts with --detailed-exit-code")
	}

	var results []fmt.FormatResult
	switch len(s.Files) {
	case 0:
		var err error
		results, err = fmt.FormatTree(s.workingDir)
		if err != nil {
			return errors.E(err, "formatting directory %s", s.workingDir)
		}
	case 1:
		if s.Files[0] == "-" {
			content, err := io.ReadAll(os.Stdin)
			if err != nil {
				return errors.E(err, "reading stdin")
			}
			original := string(content)
			formatted, err := fmt.Format(original, "<stdin>")
			if err != nil {
				return errors.E(err, "formatting stdin")
			}

			if s.Check {
				if formatted != original {
					return errors.E(exit.Failed)
				}
				return nil
			}

			stdfmt.Print(formatted)
			return nil
		}

		fallthrough
	default:
		var err error
		results, err = fmt.FormatFiles(s.workingDir, s.Files)
		if err != nil {
			return errors.E(err, "formatting files")
		}
	}

	for _, res := range results {
		path := strings.TrimPrefix(res.Path(), s.workingDir+string(filepath.Separator))
		s.printers.Stdout.Println(path)
	}

	if len(results) > 0 {
		if s.Check {
			return errors.E(exit.Failed)
		}
	}

	errs := errors.L()
	for _, res := range results {
		errs.Append(res.Save())
	}

	if err := errs.AsError(); err != nil {
		return errors.E(err, "saving formatted files")
	}

	if len(results) > 0 && s.DetailedExitCode {
		return errors.E(exit.Changed)
	}
	return nil
}
