package genfile_test

import (
	"testing"

	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
)

func TestLoadGenerateFilesForRootContext(t *testing.T) {
	t.Parallel()
	tcases := []testcase{
		{
			name:   "empty content attribute generates empty body",
			layout: []string{"d:dir"},
			dir:    "/dir",
			configs: []hclconfig{
				{
					path: "/empty.tm",
					add: GenerateFile(
						Labels("empty"),
						Str("context", "root"),
						Str("content", ""),
					),
				},
			},
			want: []result{
				{
					name: "empty",
					file: genFile{
						body:      "",
						condition: true,
					},
				},
			},
		},
	}
	for _, tcase := range tcases {
		testGenfile(t, tcase)
	}
}
