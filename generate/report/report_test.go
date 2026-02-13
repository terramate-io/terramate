// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package report_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/terramate-io/terramate/errors"
	genreport "github.com/terramate-io/terramate/generate/report"
	"github.com/terramate-io/terramate/project"
)

func TestReportFull(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name        string
		report      genreport.Report
		wantFull    string
		wantMinimal string
	}

	tcases := []testcase{
		{
			name:        "empty report",
			report:      genreport.Report{},
			wantFull:    "Nothing to do, generated code is up to date",
			wantMinimal: "",
		},
		{
			name: "with bootstrap err",
			report: genreport.Report{
				BootstrapErr: errors.E("such fail, much terrible"),
			},
			wantFull: `Fatal failure preparing for code generation.
Error details: such fail, much terrible`,
			wantMinimal: `Fatal failure preparing for code generation.
Error details: such fail, much terrible`,
		},
		{
			name: "with bootstrap err results are ignored (should have none)",
			report: genreport.Report{
				BootstrapErr: errors.E("ignore"),
				Successes: []genreport.Result{
					{
						Dir:     project.NewPath("/test"),
						Created: []string{"test"},
					},
				},
				Failures: []genreport.FailureResult{
					{
						Error: errors.E("ignored"),
					},
				},
			},
			wantFull: `Fatal failure preparing for code generation.
Error details: ignore`,
			wantMinimal: `Fatal failure preparing for code generation.
Error details: ignore`,
		},
		{
			name: "success results",
			report: genreport.Report{
				Successes: []genreport.Result{
					{
						Dir:     project.NewPath("/test"),
						Created: []string{"test"},
					},
					{
						Dir:     project.NewPath("/test2"),
						Changed: []string{"test"},
					},
					{
						Dir:     project.NewPath("/test3"),
						Deleted: []string{"test"},
					},
					{
						Dir:     project.NewPath("/test4"),
						Created: []string{"created1.tf", "created2.tf"},
						Changed: []string{"changed.tf", "changed2.tf"},
						Deleted: []string{"removed1.tf", "removed2.tf"},
					},
				},
			},
			wantFull: `Code generation report

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

Hint: '+', '~' and '-' mean the file was created, changed and deleted, respectively.`,
			wantMinimal: `Created file /test/test
Changed file /test2/test
Deleted file /test3/test
Created file /test4/created1.tf
Created file /test4/created2.tf
Changed file /test4/changed.tf
Changed file /test4/changed2.tf
Deleted file /test4/removed1.tf
Deleted file /test4/removed2.tf`,
		},
		{
			name: "failure results",
			report: genreport.Report{
				Failures: []genreport.FailureResult{
					{
						Result: genreport.Result{
							Dir: project.NewPath("/test"),
						},
						Error: errors.E("full error"),
					},
					{
						Result: genreport.Result{
							Dir:     project.NewPath("/test2"),
							Created: []string{"created1.tf", "created2.tf"},
							Changed: []string{"changed.tf", "changed2.tf"},
							Deleted: []string{"removed1.tf", "removed2.tf"},
						},
						Error: errors.E("partial error"),
					},
				},
			},
			wantFull: `Code generation report

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

Hint: '+', '~' and '-' mean the file was created, changed and deleted, respectively.`,
			wantMinimal: `Error on /test: full error
Error on /test2: partial error
Created file /test2/created1.tf
Created file /test2/created2.tf
Changed file /test2/changed.tf
Changed file /test2/changed2.tf
Deleted file /test2/removed1.tf
Deleted file /test2/removed2.tf`,
		},
		{
			name: "partial result",
			report: genreport.Report{
				Successes: []genreport.Result{
					{
						Dir:     project.NewPath("/success"),
						Created: []string{"created.tf"},
						Changed: []string{"changed.tf"},
						Deleted: []string{"removed.tf"},
					},
					{
						Dir:     project.NewPath("/success2"),
						Created: []string{"created.tf"},
						Changed: []string{"changed.tf"},
						Deleted: []string{"removed.tf"},
					},
				},
				Failures: []genreport.FailureResult{
					{
						Result: genreport.Result{
							Dir: project.NewPath("/failed"),
						},
						Error: errors.E("error"),
					},
					{
						Result: genreport.Result{
							Dir: project.NewPath("/failed2"),
						},
						Error: errors.E("error"),
					},
				},
			},
			wantFull: `Code generation report

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

Hint: '+', '~' and '-' mean the file was created, changed and deleted, respectively.`,
			wantMinimal: `Created file /success/created.tf
Changed file /success/changed.tf
Deleted file /success/removed.tf
Created file /success2/created.tf
Changed file /success2/changed.tf
Deleted file /success2/removed.tf
Error on /failed: error
Error on /failed2: error`,
		},
		{
			name: "error result is a list",
			report: genreport.Report{
				Failures: []genreport.FailureResult{
					{
						Result: genreport.Result{
							Dir: project.NewPath("/empty"),
						},
						Error: errors.L(),
					},
					{
						Result: genreport.Result{
							Dir: project.NewPath("/failed"),
						},
						Error: errors.L(errors.E("error")),
					},
					{
						Result: genreport.Result{
							Dir: project.NewPath("/failed2"),
						},
						Error: errors.L(
							errors.E("error1"),
							errors.E("error2"),
						),
					},
				},
			},
			wantFull: `Code generation report

Failures:

- /empty

- /failed
	error: error

- /failed2
	error: error1
	error: error2

Hint: '+', '~' and '-' mean the file was created, changed and deleted, respectively.`,
			wantMinimal: `Error on /failed: error
Error on /failed2: error1
Error on /failed2: error2`,
		},
		{
			name: "cleanup error result",
			report: genreport.Report{
				Successes: []genreport.Result{
					{
						Dir:     project.NewPath("/success"),
						Created: []string{"created.tf"},
						Changed: []string{"changed.tf"},
						Deleted: []string{"removed.tf"},
					},
				},
				CleanupErr: errors.E("cleanup error"),
			},
			wantFull: `Code generation report

Successes:

- /success
	[+] created.tf
	[~] changed.tf
	[-] removed.tf

Fatal failure while cleaning up generated code outside stacks:
	error: cleanup error

Hint: '+', '~' and '-' mean the file was created, changed and deleted, respectively.`,
			wantMinimal: `Created file /success/created.tf
Changed file /success/changed.tf
Deleted file /success/removed.tf
Fatal failure while cleaning up generated code outside stacks:
	error: cleanup error`,
		},
	}

	for _, tc := range tcases {
		tcase := tc
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			got := tcase.report.Full()
			if diff := cmp.Diff(got, tcase.wantFull); diff != "" {
				t.Error("full report failed")
				t.Errorf("got:\n%s\n", got)
				t.Errorf("want:\n%s\n", tcase.wantFull)
				t.Error("diff: got(-), want(+)")
				t.Fatal(diff)
			}

			got = tcase.report.Minimal()
			if diff := cmp.Diff(got, tcase.wantMinimal); diff != "" {
				t.Error("minimal report failed")
				t.Errorf("got:\n%s\n", got)
				t.Errorf("want:\n%s\n", tcase.wantMinimal)
				t.Error("diff: got(-), want(+)")
				t.Fatal(diff)
			}
		})
	}
}
