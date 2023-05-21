// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package download

import (
	"fmt"
	"strings"

	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/modvendor"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/tf"
)

// Vendored describes a vendored dependency.
type Vendored struct {
	// Source is the remote source.
	Source tf.Source
	// Dir is the directory where the dependency have been vendored into.
	Dir project.Path
}

// IgnoredVendor describes an ignored dependency.
type IgnoredVendor struct {
	RawSource string
	Reason    error
}

// Report with the result of the vendor related functions.
type Report struct {
	Vendored map[project.Path]Vendored
	Ignored  []IgnoredVendor
	Error    error

	vendorDir project.Path
}

// NewReport returns a new empty report.
func NewReport(vendordir project.Path) Report {
	return Report{
		Vendored:  make(map[project.Path]Vendored),
		vendorDir: vendordir,
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

	sources := project.Paths{}
	for source := range r.Vendored {
		sources = append(sources, source)
	}
	sources.Sort()

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
				if e, ok := err.(*errors.Error); ok {
					report = append(report, fmt.Sprintf("- %v", e.Description))
				} else {
					report = append(report, fmt.Sprintf("- %v", err))
				}
			}
		} else {
			report = append(report, fmt.Sprintf("- %v", r.Error))
		}
	}
	return strings.Join(report, "\n")
}

// RemoveIgnoredByKind removes all ignored from this report that have errors
// with the given kind.
func (r *Report) RemoveIgnoredByKind(kind errors.Kind) {
	r.Ignored = r.filterByKind(kind)
}

// IsEmpty returns true if the report is empty (nothing to report).
func (r Report) IsEmpty() bool {
	return len(r.Vendored) == 0 &&
		len(r.Ignored) == 0 &&
		r.Error == nil
}

// HasFailures returns true if any vendor attempt failed.
// It will exclude all [ErrAlreadyVendored] since those indicate
// that the module exists on the vendor dir.
func (r Report) HasFailures() bool {
	return len(r.filterByKind(ErrAlreadyVendored)) > 0
}

func (r *Report) filterByKind(kind errors.Kind) []IgnoredVendor {
	ignored := []IgnoredVendor{}
	for _, v := range r.Ignored {
		if !errors.IsKind(v.Reason, kind) {
			ignored = append(ignored, v)
		}
	}
	return ignored
}

func (r *Report) merge(other Report) {
	if other.Error != nil {
		r.Error = errors.L(r.Error, other.Error)
	}

	for k, v := range other.Vendored {
		r.Vendored[k] = v
	}

	r.Ignored = append(r.Ignored, other.Ignored...)
}

func (r *Report) addVendored(source tf.Source) {
	dir := modvendor.TargetDir(r.vendorDir, source)
	r.Vendored[dir] = Vendored{
		Source: source,
		Dir:    dir,
	}
}

func (r *Report) addIgnored(rawSource string, err error) {
	r.Ignored = append(r.Ignored, IgnoredVendor{
		RawSource: rawSource,
		Reason:    err,
	})
}
