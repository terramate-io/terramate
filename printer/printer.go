// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package printer

import (
	"fmt"
	"io"
	"os"

	"github.com/fatih/color"
	"github.com/terramate-io/terramate/errors"
)

var (
	bold       = color.New(color.Bold).Sprint
	boldYellow = color.New(color.Bold, color.FgYellow).Sprint
	boldRed    = color.New(color.Bold, color.FgRed).Sprint
	boldGreen  = color.New(color.Bold, color.FgGreen).Sprint
)

var (
	// Stderr is the default stderr printer
	Stderr = NewPrinter(os.Stderr)
	// Stdout is the default stdout printer
	Stdout = NewPrinter(os.Stdout)

	// DefaultPrinters groups the default stdout and stderr printers.
	DefaultPrinters = Printers{
		Stdout: Stdout,
		Stderr: Stderr,
	}
)

// Printer encapsulates an io.Writer
type Printer struct {
	io.Writer
}

// Printers groups stdout and stderr printers.
type Printers struct {
	Stdout *Printer
	Stderr *Printer
}

// NewPrinter creates a new Printer with the provider io.Writer e.g.: stdio,
// stderr, file etc.
func NewPrinter(w io.Writer) *Printer {
	return &Printer{Writer: w}
}

// Println prints a message to the io.Writer
func (p *Printer) Println(msg string) {
	fprintln(p, msg)
}

// Printf prints a formatted message to the io.Writer
func (p *Printer) Printf(format string, a ...any) {
	fprintf(p, format, a...)
}

// Warn prints a message with a "Warning:" prefix. The prefix is printed in
// the boldYellow style.
func (p *Printer) Warn(arg any) {
	switch arg := arg.(type) {
	case *errors.DetailedError:
		p.printDetailedWarning(arg)
	default:
		fprintln(p, boldYellow("Warning:"), bold(arg))
	}
}

// Warnf is short for Warn(fmt.Sprintf(...)).
func (p *Printer) Warnf(format string, a ...any) {
	p.Warn(fmt.Sprintf(format, a...))
}

// ErrorWithDetails prints an error with a title and the underlying error. If
// the error contains multiple error items, each error is printed with a `->`
// prefix.
// e.g.:
// Error: parsing failed
// -> somefile.tm:8,3-7: terramate schema error: unrecognized attribute
// -> somefile.tm:9,4-7: terramate schema error: unrecognized block
func (p *Printer) ErrorWithDetails(title string, err error) {
	derr := errors.D("%s", title).WithError(err)
	p.Error(derr)
}

// FatalWithDetails prints an error with a title and the underlying error and calls
// os.Exit(1).
func (p *Printer) FatalWithDetails(title string, err error) {
	p.ErrorWithDetails(title, err)
	os.Exit(int(errors.ExitStatus(err)))
}

// Fatal prints an error and calls
// os.Exit(1).
func (p *Printer) Fatal(err any) {
	p.Error(err)
	var exit = 1
	if e, ok := err.(error); ok {
		exit = int(errors.ExitStatus(e))
	}
	os.Exit(exit)
}

// Fatalf is short for Fatal(fmt.Sprintf(...)).
func (p *Printer) Fatalf(format string, a ...any) {
	p.Fatal(fmt.Sprintf(format, a...))
}

// WarnWithDetails is similar to ErrorWithDetailsln but prints a warning
// instead
func (p *Printer) WarnWithDetails(title string, err error) {
	derr := errors.D("%s", title).WithError(err)
	p.Warn(derr)
}

// Error prints a message with a "Error:" prefix. The prefix is printed in
// the boldRed style.
func (p *Printer) Error(arg any) {
	switch arg := arg.(type) {
	case *errors.DetailedError:
		p.printDetailedError(arg)
	case error:
		if errstr := arg.Error(); errstr != "" {
			fprintln(p, boldRed("Error:"), bold(arg.Error()))
		}
	default:
		fprintln(p, boldRed("Error:"), bold(arg))
	}
}

// Errorf is short for Error(fmt.Sprintf(...)).
func (p *Printer) Errorf(format string, a ...any) {
	p.Error(fmt.Sprintf(format, a...))
}

// Success prints a message in the boldGreen style
func (p *Printer) Success(msg string) {
	fprintln(p, boldGreen(msg))
}

// Successf is short for Success(fmt.Sprintf(...)).
func (p *Printer) Successf(format string, a ...any) {
	p.Success(fmt.Sprintf(format, a...))
}

func (p *Printer) printDetailedError(err *errors.DetailedError) {
	title, items := inspectDetailedError(err)

	p.Error(title)
	for _, item := range items {
		fprintln(p, boldRed(">"), item)
	}
}

func (p *Printer) printDetailedWarning(err *errors.DetailedError) {
	title, items := inspectDetailedError(err)

	p.Warn(title)
	for _, item := range items {
		fprintln(p, boldYellow(">"), item)
	}
}

func inspectDetailedError(err *errors.DetailedError) (title string, items []string) {
	err.Inspect(func(i int, msg string, _ error, details []errors.ErrorDetails) {
		if i == 0 {
			title = msg
		}

		t := []string{}
		for _, d := range details {
			t = append(t, d.Msg)
		}
		items = append(t, items...)
	})
	return
}

func fprintln(w io.Writer, a ...any) {
	_, _ = fmt.Fprintln(w, a...)
}

func fprintf(w io.Writer, format string, a ...any) {
	_, _ = fmt.Fprintf(w, format, a...)
}
