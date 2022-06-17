package e2etest

import (
	"testing"

	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestE2EListNonGit(t *testing.T) {
	for _, tc := range listTestcases() {
		t.Run(tc.name, func(t *testing.T) {
			s := sandbox.NoGit(t)
			s.BuildTree(tc.layout)

			test.WriteRootConfig(t, s.RootDir())

			cli := newCLI(t, s.RootDir())
			assertRunResult(t, cli.listStacks(), tc.want)
		})
	}
}
