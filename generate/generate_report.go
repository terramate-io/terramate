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
	// StackPath is the absolute path of the stack relative to the project root.
	StackPath string
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
}

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
		addLine("- stack %s", stack)
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
			addStack(success.StackPath)
			addResultChangeset(success)
			newline()
		}
		needsHint = true
	}

	if len(r.Failures) > 0 {
		addLine("Failures:")
		newline()
		for _, failure := range r.Failures {
			addStack(failure.StackPath)
			addLine("\terror: %s", failure.Error)
			addResultChangeset(failure.Result)
			newline()
		}
		needsHint = true
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

func (r *Report) addFailure(s stack.S, err error) {
	r.Failures = append(r.Failures, FailureResult{
		Result: Result{
			StackPath: s.PrjAbsPath(),
		},
		Error: err,
	})
}

func (r *Report) addStackReport(s stack.S, sr stackReport) {
	if sr.empty() {
		return
	}

	res := Result{
		StackPath: s.PrjAbsPath(),
		Created:   sr.created,
		Changed:   sr.changed,
		Deleted:   sr.deleted,
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

type stackReport struct {
	created []string
	changed []string
	deleted []string
	err     error
}

func (s *stackReport) addCreatedFile(filename string) {
	s.created = append(s.created, filename)
}

func (s *stackReport) addDeletedFile(filename string) {
	s.deleted = append(s.deleted, filename)
}

func (s *stackReport) addChangedFile(filename string) {
	s.changed = append(s.changed, filename)
}

func (s stackReport) isSuccess() bool {
	return s.err == nil
}

func (s stackReport) empty() bool {
	return len(s.created) == 0 &&
		len(s.changed) == 0 &&
		len(s.deleted) == 0 &&
		s.err == nil
}
