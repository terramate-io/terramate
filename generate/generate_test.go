// Copyright 2021 Mineiros GmbH
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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/mineiros-io/terramate/generate"
	"github.com/rs/zerolog"
)

func assertHCLEquals(t *testing.T, got string, want string) {
	t.Helper()

	// Not 100% sure it is a good idea to compare HCL as strings, formatting
	// issues can be annoying and can make tests brittle
	// (but we test the formatting too... so maybe that is good ? =P)
	const trimmedChars = "\n "

	want = string(generate.PrependHeader(want))
	got = strings.Trim(got, trimmedChars)
	want = strings.Trim(want, trimmedChars)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Error("generated code doesn't match expectation")
		t.Errorf("want:\n%q", want)
		t.Errorf("got:\n%q", got)
		t.Fatalf("diff:\n%s", diff)
	}
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
