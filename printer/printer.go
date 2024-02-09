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
)

// Printer encapuslates an io.Writer
type Printer struct {
	w io.Writer
}

// NewPrinter creates a new Printer with the provider io.Writer e.g.: stdio,
// stderr, file etc.
func NewPrinter(w io.Writer) *Printer {
	return &Printer{w}
}

// Println prints a message to the io.Writer
func (p *Printer) Println(msg string) {
	fmt.Fprintln(p.w, msg)
}

// Warn prints a message with a "Warning:" prefix. The prefix is printed in
// the boldYellow style.
func (p *Printer) Warn(title string) {
	fmt.Fprintln(p.w, boldYellow("Warning:"), bold(title))
}

// ErrorWithDetails prints an error with a title and the underlying error. If
// the error contains multiple error items, each error is printed with a `->`
// prefix.
// e.g.:
// Error: parsing failed
// -> somefile.tm:8,3-7: terramate schema error: unrecognized attribute
// -> somefile.tm:9,4-7: terramate schema error: unrecognized block
func (p *Printer) ErrorWithDetails(title string, err error) {
	p.Error(title)

	for _, item := range toStrings(err) {
		fmt.Fprintln(p.w, boldRed(">"), item)
	}
}

// Fatal prints an error with a title and the underlying error and calls
// os.Exit(1).
func (p *Printer) Fatal(title string, err error) {
	p.ErrorWithDetails(title, err)
	os.Exit(1)
}

// WarnWithDetails is similar to ErrorWithDetailsln but prints a warning
// instead
func (p *Printer) WarnWithDetails(title string, err error) {
	p.Warn(title)

	for _, item := range toStrings(err) {
		fmt.Fprintln(p.w, boldYellow(">"), item)
	}
}

// Error prints a message with a "Error:" prefix. The prefix is printed in
// the boldRed style.
func (p *Printer) Error(title string) {
	fmt.Fprintln(p.w, boldRed("Error:"), bold(title))
}

// Success prints a message in the boldGreen style
func (p *Printer) Success(msg string) {
	fmt.Fprintln(p.w, boldGreen(msg))
}

// toStrings converts an error into a list of strings where each string
// represents an individual error.
func toStrings(err error) []string {
	errs := errors.L(err).Errors()
	list := make([]string, 0, len(errs))
	for _, errItem := range errs {
		list = append(list, errItem.Error())
	}

	return list
}
