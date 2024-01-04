// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package printer

import (
	"fmt"
	"io"

	"github.com/fatih/color"
	"github.com/terramate-io/terramate/errors"
)

var (
	bold       = color.New(color.Bold).Sprint
	boldYellow = color.New(color.Bold, color.FgYellow).Sprint
	boldRed    = color.New(color.Bold, color.FgRed).Sprint
	boldGreen  = color.New(color.Bold, color.FgGreen).Sprint
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

// Warnln prints a message with a "Warning:" prefix. The prefix is printed in
// the boldYellow style.
func (p *Printer) Warnln(title string) {
	fmt.Fprintln(p.w, boldYellow("Warning:"), bold(title))
}

// ErrorWithDetailsln prints an error with a title and the underlying error. If
// the error contains multiple error items, each error is printed with a `->`
// prefix.
// e.g.:
// Error: parsing failed
// -> somefile.tm:8,3-7: terramate schema error: unrecognized attribute
// -> somefile.tm:9,4-7: terramate schema error: unrecognized block
func (p *Printer) ErrorWithDetailsln(title string, err error) {
	p.Errorln(title)

	for _, item := range toStrings(err) {
		fmt.Fprintln(p.w, boldRed(">"), item)
	}
}

// Errorln prints a message with a "Error:" prefix. The prefix is prinited in
// the boldRed style.
func (p *Printer) Errorln(title string) {
	fmt.Fprintln(p.w, boldRed("Error:"), bold(title))
}

// Successln prints a message in the boldGreen style
func (p *Printer) Successln(msg string) {
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
