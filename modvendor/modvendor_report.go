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

func (r *Report) addVendored(rawSource string, source tf.Source, dir string) {
	r.Vendored[rawSource] = Vendored{
		Source: source,
		Dir:    dir,
	}
}

func (r *Report) addIgnored(rawSource string, reason string) {
	r.Ignored = append(r.Ignored, IgnoredVendor{
		RawSource: rawSource,
		Reason:    reason,
	})
}
