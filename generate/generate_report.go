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

	"github.com/mineiros-io/terramate/stack"
)

// Result represents code generation result
type Result struct {
	// Dir is the absolute path of the dir relative to the project root.
	Dir string
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

func (r Report) String() string {
	if r.empty() {
		return "Nothing to do, code generation is updated"
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
	addStack := func(stack string) {
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
			addLine("\terror: %s", failure.Error)
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

func (r Report) empty() bool {
	return r.BootstrapErr == nil &&
		len(r.Failures) == 0 &&
		len(r.Successes) == 0
}

func (r *Report) sortFilenames() {
	for _, success := range r.Successes {
		success.sortFilenames()
	}
	for _, failure := range r.Failures {
		failure.Result.sortFilenames()
	}
}

func (r *Report) addFailure(s *stack.S, err error) {
	r.Failures = append(r.Failures, FailureResult{
		Result: Result{
			Dir: s.Path(),
		},
		Error: err,
	})
}

func (r *Report) addDirReport(path string, sr dirReport) {
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
