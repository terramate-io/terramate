// Copyright 2022 Mineiros GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package download

import (
	"fmt"
	"strings"

	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/modvendor"
	"github.com/mineiros-io/terramate/project"
	"github.com/mineiros-io/terramate/tf"
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

func (r *Report) merge(other Report) {
	if other.Error != nil {
		r.Error = other.Error
	}

	for k, v := range other.Vendored {
		r.Vendored[k] = v
	}

	r.Ignored = append(other.Ignored)
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
