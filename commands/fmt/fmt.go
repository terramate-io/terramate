package fmt

import (
	"context"
	stdfmt "fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/exit"
	"github.com/terramate-io/terramate/hcl/fmt"
	"github.com/terramate-io/terramate/printer"
)

type Spec struct {
	WorkingDir       string
	Check            bool
	DetailedExitCode bool
	Files            []string
	Printers         printer.Printers
}

func (s *Spec) Name() string { return "fmt" }

func (s *Spec) Exec(ctx context.Context) error {
	logger := log.With().
		Str("action", "commands/fmt").
		Str("workingDir", s.WorkingDir).
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
		results, err = fmt.FormatTree(s.WorkingDir)
		if err != nil {
			return errors.E(err, "formatting directory %s", s.WorkingDir)
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
					return errors.E(exit.Failed, "code is not formatted")
				}
				return nil
			}

			stdfmt.Print(formatted)
			return nil
		}

		fallthrough
	default:
		var err error
		results, err = fmt.FormatFiles(s.WorkingDir, s.Files)
		if err != nil {
			return errors.E(err, "formatting files")
		}
	}

	for _, res := range results {
		path := strings.TrimPrefix(res.Path(), s.WorkingDir+string(filepath.Separator))
		s.Printers.Stdout.Println(path)
	}

	if len(results) > 0 {
		if s.Check {
			return errors.E(exit.Failed, "code is not formatted")
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
		return errors.E(exit.Changed, "code is not formatted")
	}
	return nil
}
