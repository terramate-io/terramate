package e2etest

import (
	"fmt"
	"strings"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestFormatRecursivelly(t *testing.T) {
	const unformattedHCL = `
stack {
name = "name"
		description = "desc"
	}
	`

	formattedHCL, err := hcl.Format(unformattedHCL, "")
	assert.NoError(t, err)

	s := sandbox.New(t)
	sprintf := fmt.Sprintf
	s.BuildTree([]string{
		sprintf("f:another-stacks/stack-1/stack.tm.hcl:%s", unformattedHCL),
		sprintf("f:another-stacks/stack-2/stack.tm.hcl:%s", unformattedHCL),
		sprintf("f:stacks/stack-1/stack.tm:%s", unformattedHCL),
		sprintf("f:stacks/stack-2/stack.tm:%s", unformattedHCL),
	})

	wantedFiles := []string{
		"another-stacks/stack-1/stack.tm.hcl",
		"another-stacks/stack-2/stack.tm.hcl",
		"stacks/stack-1/stack.tm",
		"stacks/stack-2/stack.tm",
	}

	assertWantedFilesContents := func(t *testing.T, files []string, want string) {
		t.Helper()

		for _, file := range files {
			got := s.RootEntry().ReadFile(file)
			assert.EqualStrings(t, want, string(got))
		}
	}

	cli := newCLI(t, s.RootDir())

	t.Run("Checking", func(t *testing.T) {
		assertRunResult(t, cli.run("fmt", "--check"), runExpected{
			Stdout: strings.Join(wantedFiles, "\n"),
		})
		assertWantedFilesContents(t, wantedFiles, unformattedHCL)
	})

	t.Run("ChangingInPlace", func(t *testing.T) {
		assertRunResult(t, cli.run("fmt"), runExpected{
			Stdout: strings.Join(wantedFiles, "\n"),
		})
		assertWantedFilesContents(t, wantedFiles, formattedHCL)
	})
}
