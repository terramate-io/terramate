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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/generate"
	errtest "github.com/mineiros-io/terramate/test/errors"
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
			want:   "Nothing to do, generated code is up to date",
		},
		{
			name: "with bootstrap err",
			report: generate.Report{
				BootstrapErr: errors.E("such fail, much terrible"),
			},
			want: `Fatal failure preparing for code generation.
Error details: such fail, much terrible`,
		},
		{
			name: "with bootstrap err results are ignored (should have none)",
			report: generate.Report{
				BootstrapErr: errors.E("ignore"),
				Successes: []generate.Result{
					{
						Dir:     "/test",
						Created: []string{"test"},
					},
				},
				Failures: []generate.FailureResult{
					{
						Error: errors.E("ignored"),
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
						Dir:     "/test",
						Created: []string{"test"},
					},
					{
						Dir:     "/test2",
						Changed: []string{"test"},
					},
					{
						Dir:     "/test3",
						Deleted: []string{"test"},
					},
					{
						Dir:     "/test4",
						Created: []string{"created1.tf", "created2.tf"},
						Changed: []string{"changed.tf", "changed2.tf"},
						Deleted: []string{"removed1.tf", "removed2.tf"},
					},
				},
			},
			want: `Code generation report

Successes:

- /test
	[+] test

- /test2
	[~] test

- /test3
	[-] test

- /test4
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
							Dir: "/test",
						},
						Error: errors.E("full error"),
					},
					{
						Result: generate.Result{
							Dir:     "/test2",
							Created: []string{"created1.tf", "created2.tf"},
							Changed: []string{"changed.tf", "changed2.tf"},
							Deleted: []string{"removed1.tf", "removed2.tf"},
						},
						Error: errors.E("partial error"),
					},
				},
			},
			want: `Code generation report

Failures:

- /test
	error: full error

- /test2
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
						Dir:     "/success",
						Created: []string{"created.tf"},
						Changed: []string{"changed.tf"},
						Deleted: []string{"removed.tf"},
					},
					{
						Dir:     "/success2",
						Created: []string{"created.tf"},
						Changed: []string{"changed.tf"},
						Deleted: []string{"removed.tf"},
					},
				},
				Failures: []generate.FailureResult{
					{
						Result: generate.Result{
							Dir: "/failed",
						},
						Error: errors.E("error"),
					},
					{
						Result: generate.Result{
							Dir: "/failed2",
						},
						Error: errors.E("error"),
					},
				},
			},
			want: `Code generation report

Successes:

- /success
	[+] created.tf
	[~] changed.tf
	[-] removed.tf

- /success2
	[+] created.tf
	[~] changed.tf
	[-] removed.tf

Failures:

- /failed
	error: error

- /failed2
	error: error

Hint: '+', '~' and '-' means the file was created, changed and deleted, respectively.`,
		},
		{
			name: "error result is a list",
			report: generate.Report{
				Failures: []generate.FailureResult{
					{
						Result: generate.Result{
							Dir: "/empty",
						},
						Error: errors.L(),
					},
					{
						Result: generate.Result{
							Dir: "/failed",
						},
						Error: errors.L(errors.E("error")),
					},
					{
						Result: generate.Result{
							Dir: "/failed2",
						},
						Error: errors.L(
							errors.E("error1"),
							errors.E("error2"),
						),
					},
				},
			},
			want: `Code generation report

Failures:

- /empty

- /failed
	error: error

- /failed2
	error: error1
	error: error2

Hint: '+', '~' and '-' means the file was created, changed and deleted, respectively.`,
		},
		{
			name: "cleanup error result",
			report: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     "/success",
						Created: []string{"created.tf"},
						Changed: []string{"changed.tf"},
						Deleted: []string{"removed.tf"},
					},
				},
				CleanupErr: errors.E("cleanup error"),
			},
			want: `Code generation report

Successes:

- /success
	[+] created.tf
	[~] changed.tf
	[-] removed.tf

Fatal failure while cleaning up generated code outside stacks:
	error: cleanup error

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

	// WHY: we can't just use cmp.Diff since the errors included on the Report
	// are not comparable and may contain unexported fields (depending on how errors are built)

	errtest.Assert(t, got.BootstrapErr, want.BootstrapErr)

	if diff := cmp.Diff(got.Successes, want.Successes); diff != "" {
		t.Errorf("success results differs: got(-) want(+)")
		t.Error(diff)
	}

	assert.EqualInts(t,
		len(want.Failures),
		len(got.Failures),
		"unmatching failures: want:\n%s\ngot:\n%s\n", want, got)

	for i, gotFailure := range got.Failures {
		wantFailure := want.Failures[i]

		if diff := cmp.Diff(gotFailure.Result, wantFailure.Result); diff != "" {
			t.Errorf("failure result differs: got(-) want(+)")
			t.Fatal(diff)
		}

		errtest.Assert(t, gotFailure.Error, wantFailure.Error)
	}
}
