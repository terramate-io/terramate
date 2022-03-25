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

package generate_test

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/mineiros-io/terramate/generate"
)

func TestReportRepresentation(t *testing.T) {
	type testcase struct {
		name   string
		report generate.Report
		want   string
	}

	tcases := []testcase{
		{
			name:   "empty report",
			report: generate.Report{},
			want:   "Nothing to do, code generation is updated",
		},
		{
			name: "with bootstrap err",
			report: generate.Report{
				BootstrapErr: errors.New("such fail, much terrible"),
			},
			want: `Fatal failure preparing for code generation.
Error details: such fail, much terrible`,
		},
		{
			name: "with bootstrap err results are ignored (should have none)",
			report: generate.Report{
				BootstrapErr: errors.New("ignore"),
				Successes: []generate.Result{
					{
						StackPath: "/test",
						Created:   []string{"test"},
					},
				},
				Failures: []generate.FailureResult{
					{
						Error: errors.New("ignored"),
					},
				},
			},
			want: `Fatal failure preparing for code generation.
Error details: ignore`,
		},
		{
			name: "success results",
			report: generate.Report{
				Successes: []generate.Result{
					{
						StackPath: "/test",
						Created:   []string{"test"},
					},
					{
						StackPath: "/test2",
						Changed:   []string{"test"},
					},
					{
						StackPath: "/test3",
						Deleted:   []string{"test"},
					},
					{
						StackPath: "/test4",
						Created:   []string{"created1.tf", "created2.tf"},
						Changed:   []string{"changed.tf", "changed2.tf"},
						Deleted:   []string{"removed1.tf", "removed2.tf"},
					},
				},
			},
			want: `Code generation report

Successes:

- stack /test
	[+] test

- stack /test2
	[~] test

- stack /test3
	[-] test

- stack /test4
	[+] created1.tf
	[+] created2.tf
	[~] changed.tf
	[~] changed2.tf
	[-] removed1.tf
	[-] removed2.tf

Hint: '+', '~' and '-' means the file was created, changed and deleted, respectively.`,
		},
		{
			name: "failure results",
			report: generate.Report{
				Failures: []generate.FailureResult{
					{
						Result: generate.Result{
							StackPath: "/test",
						},
						Error: errors.New("full error"),
					},
					{
						Result: generate.Result{
							StackPath: "/test2",
							Created:   []string{"created1.tf", "created2.tf"},
							Changed:   []string{"changed.tf", "changed2.tf"},
							Deleted:   []string{"removed1.tf", "removed2.tf"},
						},
						Error: errors.New("partial error"),
					},
				},
			},
			want: `Code generation report

Failures:

- stack /test
	error: full error

- stack /test2
	error: partial error
	[+] created1.tf
	[+] created2.tf
	[~] changed.tf
	[~] changed2.tf
	[-] removed1.tf
	[-] removed2.tf

Hint: '+', '~' and '-' means the file was created, changed and deleted, respectively.`,
		},
		{
			name: "partial result",
			report: generate.Report{
				Successes: []generate.Result{
					{
						StackPath: "/success",
						Created:   []string{"created.tf"},
						Changed:   []string{"changed.tf"},
						Deleted:   []string{"removed.tf"},
					},
					{
						StackPath: "/success2",
						Created:   []string{"created.tf"},
						Changed:   []string{"changed.tf"},
						Deleted:   []string{"removed.tf"},
					},
				},
				Failures: []generate.FailureResult{
					{
						Result: generate.Result{
							StackPath: "/failed",
						},
						Error: errors.New("error"),
					},
					{
						Result: generate.Result{
							StackPath: "/failed2",
						},
						Error: errors.New("error"),
					},
				},
			},
			want: `Code generation report

Successes:

- stack /success
	[+] created.tf
	[~] changed.tf
	[-] removed.tf

- stack /success2
	[+] created.tf
	[~] changed.tf
	[-] removed.tf

Failures:

- stack /failed
	error: error

- stack /failed2
	error: error

Hint: '+', '~' and '-' means the file was created, changed and deleted, respectively.`,
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			got := tcase.report.String()
			if diff := cmp.Diff(got, tcase.want); diff != "" {
				t.Errorf("got:\n%s\n", got)
				t.Errorf("want:\n%s\n", tcase.want)
				t.Error("diff: got(-), want(+)")
				t.Fatal(diff)
			}
		})
	}
}

func assertReportHasError(t *testing.T, report generate.Report, err error) {
	t.Helper()
	// Most of this assertion behavior is due to making it easier to
	// refactor the tests to the new report design on code generation.
	// It is non ideal but it made the change radius smaller.
	// Can be improved further in the future.

	if err == nil {
		if len(report.Failures) > 0 {
			t.Fatalf("wanted no error but got failures: %v", report.Failures)
		}
		return
	}

	// Just checking if at least one of the errors match is exactly
	// what we were doing since before we had a chain of errors
	// and only checked for a match inside. This is non-ideal so in
	// the future lets match expectations with precision.
	if errors.Is(report.BootstrapErr, err) {
		return
	}
	for _, failure := range report.Failures {
		if errors.Is(failure.Error, err) {
			return
		}
	}
	t.Fatalf("unable to find match for %v on report:\n%s", err, report)
}

func assertEqualReports(t *testing.T, got, want generate.Report) {
	t.Helper()
	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("got(-) want(+)")
		t.Fatal(diff)
	}
}
