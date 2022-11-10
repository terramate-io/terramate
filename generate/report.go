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

package generate

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/project"
)

// Result represents code generation result
type Result struct {
	// Dir is the absolute path of the dir relative to the project root.
	Dir project.Path
	// Created contains filenames of all created files inside the stack
	Created []string
	// Changed contains filenames of all changed files inside the stack
	Changed []string
	// Deleted contains filenames of all deleted files inside the stack
	Deleted []string
}

// FailureResult represents a failure on code generation.
type FailureResult struct {
	Result
	Error error
}

// Report has the results of the code generation process.
type Report struct {
	// BootstrapErr is an error that happened before code generation
	// could be started, indicating that no changes were made to any stack.
	BootstrapErr error

	// Successes are the success results
	Successes []Result

	// Failures are stacks that failed without generating any code
	Failures []FailureResult

	// CleanupErr is an error that happened after code generation
	// was done while trying to cleanup files outside stacks.
	CleanupErr error
}

// HasFailures returns true if this report includes any failures.
func (r Report) HasFailures() bool {
	return r.BootstrapErr != nil || len(r.Failures) > 0
}

// Full provides a full report of the generated code, including information per stack.
func (r Report) Full() string {
	if r.empty() {
		return "Nothing to do, generated code is up to date"
	}
	if r.BootstrapErr != nil {
		return fmt.Sprintf(
			"Fatal failure preparing for code generation.\nError details: %v",
			r.BootstrapErr,
		)
	}

	// Probably could look better as a single template
	// Since the report for now is simple enough just went with plain Go
	report := []string{"Code generation report", ""}
	addLine := func(msg string, args ...interface{}) {
		report = append(report, fmt.Sprintf(msg, args...))
	}
	newline := func() {
		addLine("")
	}
	addStack := func(stack project.Path) {
		addLine("- %s", stack)
	}
	addResultChangeset := func(res Result) {
		for _, created := range res.Created {
			addLine("\t[+] %s", created)
		}
		for _, changed := range res.Changed {
			addLine("\t[~] %s", changed)
		}
		for _, deleted := range res.Deleted {
			addLine("\t[-] %s", deleted)
		}
	}
	needsHint := false

	if len(r.Successes) > 0 {
		addLine("Successes:")
		newline()
		for _, success := range r.Successes {
			addStack(success.Dir)
			addResultChangeset(success)
			newline()
		}
		needsHint = true
	}

	if len(r.Failures) > 0 {
		addLine("Failures:")
		newline()
		for _, failure := range r.Failures {
			addStack(failure.Dir)
			if list, ok := failure.Error.(*errors.List); ok {
				for _, err := range list.Errors() {
					addLine("\terror: %s", err)
				}
			} else {
				addLine("\terror: %s", failure.Error)
			}
			addResultChangeset(failure.Result)
			newline()
		}
		needsHint = true
	}

	if r.CleanupErr != nil {
		addLine("Fatal failure while cleaning up generated code outside stacks:")
		addLine("\terror: %s\n", r.CleanupErr)
	}

	if needsHint {
		addLine("Hint: '+', '~' and '-' means the file was created, changed and deleted, respectively.")
	}

	return strings.Join(report, "\n")
}

// Minimal provides a minimal report of the generated code.
// It only lists created/deleted/changed files in a per file manner.
func (r Report) Minimal() string {
	if r.empty() {
		return ""
	}
	if r.BootstrapErr != nil {
		return fmt.Sprintf(
			"Fatal failure preparing for code generation.\nError details: %v",
			r.BootstrapErr,
		)
	}
	report := []string{}
	addLine := func(msg string, args ...interface{}) {
		report = append(report, fmt.Sprintf(msg, args...))
	}
	addResult := func(res Result) {
		for _, c := range res.Created {
			addLine("Created file %s/%s", res.Dir, c)
		}
		for _, c := range res.Changed {
			addLine("Changed file %s/%s", res.Dir, c)
		}
		for _, c := range res.Deleted {
			addLine("Deleted file %s/%s", res.Dir, c)
		}
	}

	for _, success := range r.Successes {
		addResult(success)
	}

	for _, failure := range r.Failures {
		if list, ok := failure.Error.(*errors.List); ok {
			for _, err := range list.Errors() {
				addLine("Error on %s: %v", failure.Dir, err)
			}
		} else {
			addLine("Error on %s: %v", failure.Dir, failure.Error)
		}
		addResult(failure.Result)
	}

	if r.CleanupErr != nil {
		addLine("Fatal failure while cleaning up generated code outside stacks:")
		addLine("\terror: %s", r.CleanupErr)
	}

	return strings.Join(report, "\n")
}

func (r Report) empty() bool {
	return r.BootstrapErr == nil &&
		len(r.Failures) == 0 &&
		len(r.Successes) == 0
}

func (r *Report) sort() {
	r.sortDirs()
	r.sortFilenames()
}

func (r *Report) sortDirs() {
	sort.Slice(r.Successes, func(i, j int) bool {
		return r.Successes[i].Dir < r.Successes[j].Dir
	})
	sort.Slice(r.Failures, func(i, j int) bool {
		return r.Failures[i].Dir < r.Failures[j].Dir
	})
}

func (r *Report) sortFilenames() {
	for _, success := range r.Successes {
		success.sortFilenames()
	}
	for _, failure := range r.Failures {
		failure.Result.sortFilenames()
	}
}

func (r *Report) addFailure(dir project.Path, err error) {
	r.Failures = append(r.Failures, FailureResult{
		Result: Result{
			Dir: dir,
		},
		Error: err,
	})
}

func (r *Report) addDirReport(path project.Path, sr dirReport) {
	if sr.empty() {
		return
	}

	res := Result{
		Dir:     path,
		Created: sr.created,
		Changed: sr.changed,
		Deleted: sr.deleted,
	}

	if sr.isSuccess() {
		r.Successes = append(r.Successes, res)
		return
	}
	r.Failures = append(r.Failures, FailureResult{
		Result: res,
		Error:  sr.err,
	})
}

func (r *Result) sortFilenames() {
	sort.Strings(r.Created)
	sort.Strings(r.Changed)
	sort.Strings(r.Deleted)
}

type dirReport struct {
	created []string
	changed []string
	deleted []string
	err     error
}

func (s *dirReport) addCreatedFile(filename string) {
	s.created = append(s.created, filename)
}

func (s *dirReport) addDeletedFile(filename string) {
	s.deleted = append(s.deleted, filename)
}

func (s *dirReport) addChangedFile(filename string) {
	s.changed = append(s.changed, filename)
}

func (s dirReport) isSuccess() bool {
	return s.err == nil
}

func (s dirReport) empty() bool {
	return len(s.created) == 0 &&
		len(s.changed) == 0 &&
		len(s.deleted) == 0 &&
		s.err == nil
}

type result interface {
	Result | FailureResult
}

func joinResults[T result](results ...[]T) []T {
	var all []T
	for _, r := range results {
		all = append(all, r...)
	}
	return all
}

func mergeReports(r1, r2 Report) Report {
	merged := Report{}
	merged.BootstrapErr = errors.L(r1.BootstrapErr, r2.BootstrapErr).AsError()
	merged.CleanupErr = errors.L(r1.CleanupErr, r2.CleanupErr).AsError()

	merged.Successes = joinResults(r1.Successes, r2.Successes)
	merged.Failures = joinResults(r1.Failures, r2.Failures)
	return merged
}
