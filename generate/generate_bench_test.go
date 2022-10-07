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
	"fmt"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/generate"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func BenchmarkGenerate(b *testing.B) {
	benchs := []benchmark{
		{
			stacks:   10,
			asserts:  10,
			genhcl:   10,
			genfiles: 5,
			globals:  100,
		},
		{
			stacks:   100,
			asserts:  10,
			genhcl:   10,
			genfiles: 5,
			globals:  100,
		},
		{
			stacks:   1000,
			asserts:  10,
			genhcl:   10,
			genfiles: 5,
			globals:  100,
		},
	}

	for _, bench := range benchs {
		b.Run(bench.String(), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				bench.run(b)
			}
		})
	}
}

type benchmark struct {
	stacks   int
	asserts  int
	genhcl   int
	genfiles int
	globals  int
}

func (bm benchmark) String() string {
	return fmt.Sprintf("stacks=%d asserts=%d genhcl=%d genfiles=%d globals=%d",
		bm.stacks, bm.asserts, bm.genhcl, bm.genfiles, bm.globals)
}

func (bm benchmark) run(b *testing.B) {
	s := sandbox.New(b)
	createStacks(s, bm.stacks)
	globals := createGlobals(s, bm.globals)
	createGenHCLs(s, globals, bm.genhcl)
	createGenFiles(s, globals, bm.genfiles)
	createAsserts(s, globals, bm.asserts)

	b.ResetTimer()

	report := generate.Do(s.RootDir(), s.RootDir())

	b.StopTimer()

	assert.EqualInts(b, bm.stacks, len(report.Successes))
	assert.EqualInts(b, 0, len(report.Failures))

	for _, success := range report.Successes {
		assert.EqualInts(b, bm.genhcl+bm.genfiles, len(success.Created))
		assert.EqualInts(b, 0, len(success.Changed))
		assert.EqualInts(b, 0, len(success.Deleted))
	}
}

func createStacks(s sandbox.S, stacks int) {
}

func createGlobals(s sandbox.S, globals int) []string {
	return nil
}

func createGenHCLs(s sandbox.S, globals []string, genhcls int) {
}

func createGenFiles(s sandbox.S, globals []string, genfiles int) {
}

func createAsserts(s sandbox.S, globals []string, asserts int) {
}
