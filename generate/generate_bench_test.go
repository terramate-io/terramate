// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package generate_test

import (
	"fmt"
	"path"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/generate"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/test/sandbox"
)

func BenchmarkGenerate(b *testing.B) {
	// benchmarks the case when there are a lot of globals defined in a hierarchy
	// of directories/stacks but only a few are referenced in generate blocks.
	// Terramate must be smart and only evaluate the used globals.

	b.StopTimer()
	s := sandbox.NoGit(b, true)

	stackPaths := []string{
		"s1",
		"s1/s2",
		"s1/s2/s3",
		"s1/s2/s3/s4",
		"s1/s2/s3/s4/s5",
	}

	layout := []string{}
	for _, s := range stackPaths {
		layout = append(layout, "s:"+s)
	}

	s.BuildTree(layout)

	const numGlobalsPerStack = 100
	for _, sp := range stackPaths {
		content := "globals {\nlist = tm_range(1000)\n"
		for i := 0; i < numGlobalsPerStack; i++ {
			content += fmt.Sprintf("\t%s_%d = [for i in global.list : i*i]\n", path.Base(sp), i)
		}
		content += "}\n"
		s.DirEntry(sp).CreateFile("globals.tm", content)
	}

	s.DirEntry(stackPaths[len(stackPaths)-1]).CreateFile("gen.tm", fmt.Sprintf(`
	generate_hcl "leaf.hcl" {
		content {
			a = global.s5_%d
		}
	}
	`, numGlobalsPerStack/2))

	root, err := config.LoadRoot(s.RootDir())
	assert.NoError(b, err)

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		report := generate.Do(root, project.NewPath("/vendor"), nil)
		if report.HasFailures() {
			b.Fatal(report.Full())
		}
	}
}

func BenchmarkGenerateRegex(b *testing.B) {
	// benchmarks the case when there are a lot of tm_regex() with same
	// pattern. The tm_regex() function must cache its compiled patterns.

	b.StopTimer()
	s := sandbox.NoGit(b, true)

	stackPaths := []string{
		"s1",
		"s1/s2",
		"s1/s2/s3",
		"s1/s2/s3/s4",
		"s1/s2/s3/s4/s5",
	}

	layout := []string{}
	for _, s := range stackPaths {
		layout = append(layout, "s:"+s)
	}

	s.BuildTree(layout)

	const numGlobalsPerStack = 10
	for _, sp := range stackPaths {
		content := `globals {
			list = tm_range(1000)
			pattern = "([\\d]+)-(\\d\\d)-(\\d\\d)"
		`
		for i := 0; i < numGlobalsPerStack; i++ {
			content += fmt.Sprintf("\t%s_%d = [for i in global.list : tm_regex(global.pattern, \"${i*i}-01-01\")[0] == \"${i*i}\" ? i*i : -1]\n", path.Base(sp), i)
		}
		content += "}\n"
		s.DirEntry(sp).CreateFile("globals.tm", content)
	}

	s.DirEntry(stackPaths[len(stackPaths)-1]).CreateFile("gen.tm", fmt.Sprintf(`
	generate_hcl "leaf.hcl" {
		content {
			a = global.s5_%d
		}
	}
	`, numGlobalsPerStack/2))

	root, err := config.LoadRoot(s.RootDir())
	assert.NoError(b, err)

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		report := generate.Do(root, project.NewPath("/vendor"), nil)
		if report.HasFailures() {
			b.Fatal(report.Full())
		}
	}
}
