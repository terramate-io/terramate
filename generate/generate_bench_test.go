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
	"github.com/mineiros-io/terramate"
	"github.com/mineiros-io/terramate/generate"
	"github.com/mineiros-io/terramate/project"
	"github.com/mineiros-io/terramate/test/sandbox"

	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
)

func BenchmarkGenerateComplex(b *testing.B) {
	b.StopTimer()
	s := sandbox.New(b)

	tree := []string{
		"s:root-stack",
	}

	emptydirs := "a/lot/of/empty/dirs/here/and/there/should/not/impact/perf"
	stackNames := []rune{'a', 'b', 'c'}
	for _, r1 := range stackNames {
		tree = append(tree, fmt.Sprintf("s:root-stack/%s/%c", emptydirs, r1))
		for _, r2 := range stackNames {
			tree = append(tree, fmt.Sprintf("s:root-stack/%s/%c/%c", emptydirs, r1, r2))
			for _, r3 := range stackNames {
				tree = append(tree, fmt.Sprintf("s:root-stack/%s/%c/%c/%c", emptydirs, r1, r2, r3))
			}
		}
	}

	s.BuildTree(tree)

	const ngenhcls = 5
	const nglobals = 10
	const rootExpr = `"terramate is fun"`
	const stackExpr = `"${tm_upper(global.rootVal)}! isn't it?"`
	rootGlobals := createGlobals(s.RootEntry(), "rootVal", nglobals, rootExpr)
	createGenHCLs(s.RootEntry(), "root", rootGlobals, ngenhcls)

	for _, r1 := range stackNames {
		dir := s.DirEntry(fmt.Sprintf("root-stack/%s/%c", emptydirs, r1))
		name := string([]rune{r1})
		globals := createGlobals(dir, name, nglobals, stackExpr)
		createGenHCLs(dir, name, globals, ngenhcls)

		for _, r2 := range stackNames {
			dir := s.DirEntry(fmt.Sprintf("root-stack/%s/%c/%c", emptydirs, r1, r2))
			name := string([]rune{r1, r2})
			globals := createGlobals(dir, name, nglobals, stackExpr)
			createGenHCLs(dir, name, globals, ngenhcls)
			for _, r3 := range stackNames {
				dir := s.DirEntry(fmt.Sprintf("root-stack/%s/%c/%c/%c", emptydirs, r1, r2, r3))
				name := string([]rune{r1, r2, r3})
				globals := createGlobals(dir, name, nglobals, stackExpr)
				createGenHCLs(dir, name, globals, ngenhcls)
			}
		}
	}

	cfg := s.Config() // configuration reused
	_, err := terramate.ListStacks(cfg.Tree())
	assert.NoError(b, err)

	b.StartTimer()
	for n := 0; n < b.N; n++ {
		_ = generate.Do(cfg, project.NewPath("/modules"), nil)
	}
}

func createGlobals(dir sandbox.DirEntry, namebase string, nglobals int, expr string) []string {
	builder := Globals()
	globalsNames := make([]string, nglobals)

	globalsNames[0] = namebase
	builder.AddExpr(namebase, expr)

	for i := 1; i < nglobals; i++ {
		name := fmt.Sprintf("%s%d", namebase, i)
		globalsNames[i] = name
		builder.AddExpr(name, fmt.Sprintf("global.%s", namebase))
	}

	dir.CreateFile("globals.tm", builder.String())
	return globalsNames
}

func createGenHCLs(dir sandbox.DirEntry, name string, globals []string, genhcls int) {
	for i := 0; i < genhcls; i++ {
		genhclDoc := GenerateHCL()
		genhclDoc.AddLabel(fmt.Sprintf("gen/%s-%d.hcl", name, i))

		content := Content()
		for j, global := range globals {
			content.AddExpr(
				fmt.Sprintf("val%d%d", i, j),
				"global."+global)
		}

		genhclDoc.AddBlock(content)
		dir.CreateFile(
			fmt.Sprintf("genhcl%d.tm", i),
			genhclDoc.String(),
		)
	}
}
