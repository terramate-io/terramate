package generate_test

import (
	"fmt"
	"testing"

	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"

	"github.com/mineiros-io/terramate/generate"
)

func TestGenerateRootContext(t *testing.T) {
	testCodeGeneration(t, []testcase{
		{
			name: "empty generate_file.context=root generates empty file",
			configs: []hclconfig{
				{
					path: "/dir/file.tm",
					add: GenerateFile(
						Labels("/other/empty.txt"),
						Str("content", ""),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/other",
					files: map[string]fmt.Stringer{
						"empty.txt": stringer(""),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     "/other",
						Created: []string{"empty.txt"},
					},
				},
			},
		},
	})
}
