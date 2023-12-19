package render

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

// Text represents a text stream to be written using the provided io.Writer
type Text struct {
	w io.Writer
}

// NewText creates a new text stream
func NewText(w io.Writer) *Text {
	return &Text{w}
}

// Println prints a message to the io.Writer
func (t *Text) Println(msg string) {
	fmt.Fprintln(t.w, msg)
}

// Warnln prints a message with a "Warning:" prefix. The prefix is printed in
// the boldYellow style.
func (t *Text) Warnln(title string) {
	fmt.Fprintln(t.w, boldYellow("Warning:"), bold(title))
}

// ErrorWithDetailsln prints an error with a title and the underlying error
// e.g.:
/* Error: parsing failed
 * -> somefile.tm:8,3-7: terramate schema error: unrecognized attribute
 */
func (t *Text) ErrorWithDetailsln(title string, err error) {
	t.Errorln(title)
	t.ErrorDetailsln(err)
}

// Errorln prints a message with a "Error:" prefix. The prefix is prinited in
// the boldRed style.
func (t *Text) Errorln(title string) {
	fmt.Fprintln(t.w, boldRed("Error:"), bold(title))
}

// Detailln prints a message with a "->" prefix. This can be used to next
// information.
func (t *Text) Detailln(msg string) {
	fmt.Fprintln(t.w, "->", msg)
}

// Successln prints a message in the boldGreen style
func (t *Text) Successln(msg string) {
	fmt.Fprintln(t.w, boldGreen(msg))
}

// ErrorDetailsln gracefully prints the provided error.
// If the error is a stdlib error, it is printed as is.
// If the error has type errors.Error, all attributes of the error are pretty
// printed.
// If the error has type errors.List, all of the error items are printed.
func (t *Text) ErrorDetailsln(err error) {
	var list *errors.List
	if errors.As(err, &list) {
		errs := list.Errors()
		for _, err := range errs {
			t.logErr(err)
		}
		return
	}

	t.logErr(err)
}

func (t *Text) logErr(err error) {
	if err == nil {
		return
	}

	var tmerr *errors.Error
	if errors.As(err, &tmerr) {
		t.Detailln(tmerr.Error())
		return
	}

	t.Detailln(err.Error())
}
