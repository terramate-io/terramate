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

func assertReportHasNoError(t *testing.T, report generate.Report) {
	t.Helper()

	if report.HasFailures() {
		t.Fatalf("wanted no error but got failures:\n%s", report)
	}
}

func assertEqualReports(t *testing.T, got, want generate.Report) {
	t.Helper()
	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("got(-) want(+)")
		t.Fatal(diff)
	}
}
