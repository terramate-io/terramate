// Copyright 2023 Mineiros GmbH
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

package globals_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/globals"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/project"
	"github.com/mineiros-io/terramate/test"
	errtest "github.com/mineiros-io/terramate/test/errors"
	"github.com/mineiros-io/terramate/test/hclwrite"
	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestGlobalsResolver(t *testing.T) {
	t.Skip()
	type (
		hclconfig struct {
			path     string
			filename string
			add      *hclwrite.Block
		}
		testcase struct {
			name      string
			layout    []string
			configs   []hclconfig
			stack     string
			lookupRef string
			want      eval.Stmts
			wantErr   error
		}
	)

	for _, tc := range []testcase{
		{
			name:      "indexing references are postponed until all other globals with base prefix are evaluated - case 3",
			layout:    []string{"s:stack"},
			stack:     "/stack",
			lookupRef: `global`,
			configs: []hclconfig{
				{
					path: "/stack",
					add: Doc(
						Globals(
							Labels("a", "b", "c"),
							Expr("providers", `{}`),
							Expr("_available_providers", `{
								aws = {
								  source  = "hashicorp/aws"
								  version = "~> 4.14"
								}
								vault = {
								  source  = "hashicorp/vault"
								  version = "~> 3.10"
								}
								postgresql = {
								  source  = "cyrilgdn/postgresql"
								  version = "~> 1.18.0"
								}
								mysql = {
								  source  = "petoju/mysql"
								  version = "~> 3.0.29"
								}
							  }`),
						),
						Globals(
							Labels("a", "b", "c"),
							Expr("required_providers", `{for k, v in global.a.b.c._available_providers : k => v if tm_try(global.a.b.c.providers[k], false)}`),
						),
						Globals(
							Labels("a", "b", "c", "providers"),
							Bool("aws", true),
						),
						Globals(
							Labels("a", "b", "c", "providers"),
							Bool("mysql", true),
						),
					),
				},
			},
			want: eval.Stmts{
				eval.NewStmt(t, `global.a.b.c._available_providers.aws.source`, `"hashicorp/aws"`),
				eval.NewStmt(t, `global.a.b.c._available_providers.aws.version`, `"~> 4.14"`),
				eval.NewStmt(t, `global.a.b.c._available_providers.vault.source`, `"hashicorp/vault"`),
				eval.NewStmt(t, `global.a.b.c._available_providers.vault.source`, `"3.10"`),
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s := sandbox.New(t)
			s.BuildTree(tc.layout)
			for _, globalBlock := range tc.configs {
				path := filepath.Join(s.RootDir(), globalBlock.path)
				filename := config.DefaultFilename
				if globalBlock.filename != "" {
					filename = globalBlock.filename
				}
				test.AppendFile(t, path, filename, globalBlock.add.String())
			}

			cfg, err := config.LoadRoot(s.RootDir())
			// TODO(i4k): this better not be tested here.
			if errors.IsKind(tc.wantErr, hcl.ErrHCLSyntax) {
				errtest.Assert(t, err, tc.wantErr)
			}

			if err != nil {
				return
			}

			tree, ok := cfg.Lookup(project.NewPath(tc.stack))
			if !ok {
				t.Fatalf("malformed test: tc.stack not found")
			}

			resolver := globals.NewResolver(tree)

			ref := eval.NewRef(t, tc.lookupRef)
			got, err := resolver.LookupRef(ref)
			errtest.Assert(t, err, tc.wantErr)
			if tc.wantErr != nil {
				return
			}

			fmt.Printf("want: %+v\n\n", tc.want)
			fmt.Printf("got : %+v\n\n", got)

			if diff := cmp.Diff(got, tc.want,
				cmp.AllowUnexported(eval.Stmt{}, project.Path{}),
				cmpopts.IgnoreInterfaces(struct {
					hhcl.Expression
				}{})); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}
