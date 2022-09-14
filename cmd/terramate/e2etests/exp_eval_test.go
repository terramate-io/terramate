package e2etest

import (
	"path/filepath"
	"testing"

	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/hclwrite"
	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestExpConfigGet(t *testing.T) {
	type (
		globalsBlock struct {
			path string
			add  *hclwrite.Block
		}
		testcase struct {
			name    string
			layout  []string
			wd      string
			globals []globalsBlock
			eval    string
			want    runExpected
		}
	)

	addnl := func(s string) string { return s + "\n" }

	testcases := []testcase{
		{
			name: "boolean expression",
			eval: `true`,
			want: runExpected{
				Stdout: addnl("true"),
			},
		},
		{
			name: "list expression",
			eval: `[1,2,3,4]`,
			want: runExpected{
				Stdout: addnl("[1, 2, 3, 4]"),
			},
		},
		{
			name: "tuple expression",
			eval: addnl(`[true,"test", [1, 2], {
				a = 1,
				b = 2,
			}]`),
			want: runExpected{
				Stdout: addnl(`[true, "test", [1, 2], {
  a = 1
  b = 2
}]`),
			},
		},
		{
			name: "simple funcalls",
			eval: `tm_upper("a")`,
			want: runExpected{
				Stdout: addnl(`"A"`),
			},
		},
		{
			name: "nested funcalls",
			eval: `tm_upper(tm_lower("A"))`,
			want: runExpected{
				Stdout: addnl(`"A"`),
			},
		},
		{
			name: "eval has access to hierarchical globals",
			globals: []globalsBlock{
				{
					path: "/",
					add: Globals(
						Str("val", "global string"),
					),
				},
			},
			eval: `global.val`,
			want: runExpected{
				Stdout: addnl(`"global string"`),
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			s := sandbox.New(t)

			s.BuildTree(tc.layout)

			for _, globalBlock := range tc.globals {
				path := filepath.Join(s.RootDir(), globalBlock.path)
				test.AppendFile(t, path, "globals.tm",
					globalBlock.add.String())
			}

			test.WriteRootConfig(t, s.RootDir())
			ts := newCLI(t, filepath.Join(s.RootDir(), tc.wd))
			assertRunResult(t, ts.run("experimental", "eval", tc.eval), tc.want)
		})
	}
}
