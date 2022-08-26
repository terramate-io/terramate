package modvendor

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/tf"
)

// Vendored describes a vendored dependency.
type Vendored struct {
	// Source is the remote source.
	Source tf.Source
	// Dir is the directory where the dependency have been vendored into.
	Dir string
}

// IgnoredVendor describes an ignored dependency.
type IgnoredVendor struct {
	RawSource string
	Reason    string
}

// Report with the result of the vendor related functions.
type Report struct {
	Vendored map[string]Vendored
	Ignored  []IgnoredVendor
	Error    error
}

// NewEmptyReport returns a new empty report.
func NewEmptyReport() Report {
	return Report{
		Vendored: make(map[string]Vendored),
	}
}

func (r Report) String() string {
	report := []string{
		"Vendor report:",
		"",
	}

	addLine := func(msg string, args ...interface{}) {
		report = append(report, fmt.Sprintf(msg, args...))
	}

	sources := []string{}
	for source := range r.Vendored {
		sources = append(sources, source)
	}

	sort.Strings(sources)
	for _, source := range sources {
		vendored := r.Vendored[source]
		addLine("[+] %s", vendored.Source.URL)
		addLine("    ref: %s", vendored.Source.Ref)
		addLine("    dir: %s", vendored.Dir)
	}
	for _, ignored := range r.Ignored {
		addLine("[!] %s", ignored.RawSource)
		addLine("    reason: %s", ignored.Reason)
	}

	return strings.Join(report, "\n")
}

// Verbose is like String but outputs additional fields (like Errors).
func (r Report) Verbose() string {
	report := []string{
		r.String(),
		"",
	}

	if r.Error != nil {
		report = append(report, "Errors:", "")
		if errs, ok := r.Error.(*errors.List); ok {
			for _, err := range errs.Errors() {
				report = append(report, fmt.Sprintf("- %v", err))
			}
		} else {
			report = append(report, fmt.Sprintf("- %v", r.Error))
		}
	}
	return strings.Join(report, "\n")
}

func (r *Report) addVendored(rawSource string, source tf.Source) {
	r.Vendored[rawSource] = Vendored{
		Source: source,
		Dir:    Dir(source),
	}
}

func (r *Report) addIgnored(rawSource string, reason string) {
	r.Ignored = append(r.Ignored, IgnoredVendor{
		RawSource: rawSource,
		Reason:    reason,
	})
}

func (r *Report) mergeReport(other Report) (out Report) {
	for k, v := range other.Vendored {
		r.Vendored[k] = v
	}
	out.Ignored = append(out.Ignored, other.Ignored...)
	errs := errors.L()
	errs.Append(r.Error, other.Error)
	r.Error = errs.AsError()
	return out
}
