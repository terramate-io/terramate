// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tmls_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	tmls "github.com/terramate-io/terramate/ls"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/test"
	lstest "github.com/terramate-io/terramate/test/ls"
	lsp "go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func TestWrongCommand(t *testing.T) {
	t.Parallel()
	f := lstest.Setup(t)

	f.Editor.CheckInitialize(f.Sandbox.RootDir())

	res, err := f.Editor.Command(lsp.ExecuteCommandParams{
		Command: "terramate.wrong.command",
	})
	assert.Error(t, err, "invalid command must return an error")
	assert.IsTrue(t, strings.Contains(err.Error(), string(tmls.ErrUnrecognizedCommand)))
	assert.IsTrue(t, res == nil)
}

func TestCreateInvalidArguments(t *testing.T) {
	t.Parallel()
	f := lstest.Setup(t)
	f.Editor.CheckInitialize(f.Sandbox.RootDir())
	res, err := f.Editor.Command(lsp.ExecuteCommandParams{
		Command: "terramate.createStack",
		// no arguments
	})
	assert.Error(t, err, "must fail: createStack requires 1 argument")
	assert.IsTrue(t, strings.Contains(err.Error(), string(tmls.ErrCreateStackNoArguments)))
	assert.IsTrue(t, res == nil)

	// invalid argument type.
	res, err = f.Editor.Command(lsp.ExecuteCommandParams{
		Command: "terramate.createStack",
		Arguments: []interface{}{
			1,
		},
	})
	assert.Error(t, err, "must fail: createStack requires string argument")
	assert.IsTrue(t, strings.Contains(err.Error(), string(tmls.ErrCreateStackInvalidArgument)))
	assert.IsTrue(t, res == nil)
}

func TestCreateCommand(t *testing.T) {
	t.Parallel()

	type want struct {
		err    error
		stacks map[string]config.Stack
	}

	type testcase struct {
		name   string
		layout []string
		args   []string
		want   want
	}

	for _, tc := range []testcase{
		{
			name:   "createStack missing required argument",
			args:   []string{"name=test"},
			layout: []string{},
			want: want{
				err: errors.E(tmls.ErrCreateStackMissingRequired),
			},
		},
		{
			name:   "createStack works if target dir does not exists",
			args:   []string{"uri=new-stack"},
			layout: []string{},
			want: want{
				stacks: map[string]config.Stack{
					"new-stack": {
						Dir:  project.NewPath("/new-stack"),
						Name: "new-stack",
					},
				},
			},
		},
		{
			name:   "createStack works if target dir exists but is not a stack",
			args:   []string{"uri=somedir"},
			layout: []string{`d:somedir`},
			want: want{
				stacks: map[string]config.Stack{
					"somedir": {
						Dir:  project.NewPath("/somedir"),
						Name: "somedir",
					},
				},
			},
		},
		{
			name:   "createStack fails if target dir is a stack",
			args:   []string{"uri=stack"},
			layout: []string{`stack:stack`},
			want: want{
				err: errors.E(tmls.ErrCreateStackFailed),
			},
		},
		{
			name: "createStack with a custom name",
			args: []string{"uri=new-stack", "name=some name here"},
			want: want{
				stacks: map[string]config.Stack{
					"new-stack": {
						Dir:  project.NewPath("/new-stack"),
						Name: "some name here",
					},
				},
			},
		},
		{
			name: "createStack with a custom description",
			args: []string{"uri=new-stack", "description=some name here"},
			want: want{
				stacks: map[string]config.Stack{
					"new-stack": {
						Dir:         project.NewPath("/new-stack"),
						Name:        "new-stack",
						Description: "some name here",
					},
				},
			},
		},
		{
			name: "createStack with a custom name and description",
			args: []string{
				"uri=new-stack",
				"description=some name here",
				"name=my stack",
			},
			want: want{
				stacks: map[string]config.Stack{
					"new-stack": {
						Dir:         project.NewPath("/new-stack"),
						Name:        "my stack",
						Description: "some name here",
					},
				},
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := lstest.Setup(t)
			f.Editor.CheckInitialize(f.Sandbox.RootDir())
			f.Sandbox.BuildTree(tc.layout)
			args := mkCreateArgs(f.Sandbox.RootDir(), tc.args)
			res, err := f.Editor.Command(lsp.ExecuteCommandParams{
				Command:   "terramate.createStack",
				Arguments: args,
			})
			if tc.want.err != nil {
				assert.Error(t, err)
				assert.IsTrue(t, strings.Contains(err.Error(), tc.want.err.Error()))
			} else {
				assert.NoError(t, err)
			}
			assert.IsTrue(t, res == nil)
			for wantStackPath, wantStack := range tc.want.stacks {
				gotStack := f.Sandbox.LoadStack(project.NewPath("/" + wantStackPath))
				test.AssertStacks(t, *gotStack, wantStack)
			}
		})
	}
}

func TestCreateGenerateID(t *testing.T) {
	t.Parallel()
	f := lstest.Setup(t)
	f.Editor.CheckInitialize(f.Sandbox.RootDir())
	res, err := f.Editor.Command(lsp.ExecuteCommandParams{
		Command: "terramate.createStack",
		Arguments: []interface{}{
			"uri=" + uri.File(filepath.Join(f.Sandbox.RootDir(), "my-stack")),
			"genid=true",
		},
	})
	assert.NoError(t, err)
	assert.IsTrue(t, res == nil)

	gotStack := f.Sandbox.LoadStack(project.NewPath("/my-stack"))
	assert.EqualStrings(t, "/my-stack", gotStack.Dir.String())
	assert.IsTrue(t, gotStack.ID != "", "id was not generated")
}

func mkCreateArgs(rootdir string, args []string) (retArgs []interface{}) {
	if len(args) == 0 {
		panic("no arguments")
	}
	for _, arg := range args {
		pos := strings.IndexRune(arg, '=')
		argName := arg[0:pos]
		argVal := arg[pos+1:]
		switch argName {
		case "uri":
			argVal = string(uri.File(filepath.Join(rootdir, argVal)))
		case "name":
		case "description":
		default:
			panic("unreachable")
		}

		retArgs = append(retArgs, fmt.Sprintf("%s=%s", argName, argVal))
	}
	return retArgs
}
