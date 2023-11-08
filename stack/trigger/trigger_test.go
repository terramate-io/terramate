// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package trigger_test

import (
	"fmt"
	"math"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/stack/trigger"
	"github.com/terramate-io/terramate/test"
	errtest "github.com/terramate-io/terramate/test/errors"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"

	"github.com/rs/zerolog"
	"github.com/terramate-io/terramate/test/sandbox"
)

type testcase struct {
	name   string
	layout []string
	path   string
	reason string
	want   error
}

func TestTriggerStacks(t *testing.T) {
	t.Parallel()
	for _, tc := range []testcase{
		{
			name: "stack on root",
			layout: []string{
				"s:stack",
			},
			path:   "/stack",
			reason: "stack inside root",
		},
		{
			name: "stack on subdir",
			layout: []string{
				"s:dir/stack",
			},
			path:   "/dir/stack",
			reason: "subdir stack",
		},
		{
			name: "root is stack",
			layout: []string{
				"s:.",
			},
			path:   "/",
			reason: "root is stack",
		},
		{
			name:   "stack doesnt exist",
			path:   "/non-existent-stack",
			reason: "should not trigger",
			want:   errors.E(trigger.ErrTrigger),
		},
		{
			name: "subdir of a stack is not valid",
			path: "/stack/dir",
			layout: []string{
				"s:stack",
				"d:stack/dir",
			},
			want: errors.E(trigger.ErrTrigger),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testTrigger(t, tc)
		})
	}
}

func testTrigger(t *testing.T, tc testcase) {
	s := sandbox.NoGit(t, true)
	s.BuildTree(tc.layout)
	root, err := config.LoadRoot(s.RootDir())
	assert.NoError(t, err)
	err = trigger.Create(root, project.NewPath(tc.path), tc.reason)
	errtest.Assert(t, err, tc.want)

	if err != nil {
		return
	}

	// check created trigger on fs
	triggerDir := filepath.Join(trigger.Dir(root.HostDir()), tc.path)
	entries := test.ReadDir(t, triggerDir)
	if len(entries) != 1 {
		t.Fatalf("want 1 trigger file, got %d: %+v", len(entries), entries)
	}

	triggerFile := filepath.Join(triggerDir, entries[0].Name())
	triggerInfo, err := trigger.ParseFile(triggerFile)

	assert.NoError(t, err)
	assert.EqualStrings(t, tc.reason, triggerInfo.Reason)

	assert.IsTrue(t, triggerInfo.Ctime > 0)
	assert.IsTrue(t, triggerInfo.Ctime < math.MaxInt64)

	assert.EqualStrings(t, trigger.DefaultContext, triggerInfo.Context)
	assert.EqualStrings(t, trigger.DefaultType, triggerInfo.Type)

	gotPath, ok := trigger.StackPath(project.PrjAbsPath(root.HostDir(), triggerFile))

	assert.IsTrue(t, ok)
	assert.EqualStrings(t, tc.path, gotPath.String())
}

func TestTriggerParser(t *testing.T) {
	t.Parallel()
	type testcase struct {
		name string
		body fmt.Stringer
		err  error
	}
	for _, tc := range []testcase{
		{
			name: "no config",
			body: Doc(),
			err:  errors.E(trigger.ErrParsing),
		},
		{
			name: "missing required attributes",
			body: Trigger(),
			err:  errors.E(trigger.ErrParsing),
		},
		{
			name: "valid file",
			body: Trigger(
				Number("ctime", 1000000),
				Str("reason", "something"),
				Expr("type", "changed"),
				Expr("context", "stack"),
			),
		},
		{
			name: "valid file (backward compatibility)",
			body: Trigger(
				Number("ctime", 1000000),
				Str("reason", "something"),
			),
		},
		{
			name: "multiple trigger blocks - fails",
			body: Doc(
				Trigger(
					Number("ctime", 1000000),
					Str("reason", "1"),
					Expr("type", "changed"),
					Expr("context", "stack"),
				),
				Trigger(
					Number("ctime", 2000000),
					Str("reason", "2"),
					Expr("type", "changed"),
					Expr("context", "stack"),
				),
			),
			err: errors.E(trigger.ErrParsing),
		},
		{
			name: "unexpected block",
			body: Block("strange",
				Number("ctime", 1000000),
				Str("reason", "something"),
				Expr("type", "changed"),
				Expr("context", "stack"),
			),
			err: errors.E(trigger.ErrParsing),
		},
		{
			name: "invalid attribute",
			body: Trigger(
				Number("ctime", 1000000),
				Str("reason", "something"),
				Expr("type", "changed"),
				Expr("context", "stack"),
				Str("invalid", "value"),
			),
			err: errors.E(trigger.ErrParsing),
		},
		{
			name: "ctime not number",
			body: Trigger(
				Str("ctime", "1000000"),
				Str("reason", "something"),
				Expr("type", "changed"),
				Expr("context", "stack"),
			),
			err: errors.E(trigger.ErrParsing),
		},
		{
			name: "funcall not supported",
			body: Trigger(
				Str("ctime", "1000000"),
				Expr("reason", `tm_title("something")`),
				Expr("type", "changed"),
				Expr("context", "stack"),
			),
			err: errors.E(trigger.ErrParsing),
		},
		{
			name: "reason not string",
			body: Trigger(
				Str("ctime", "1000000"),
				Expr("reason", `["wrong"]`),
				Expr("type", "changed"),
				Expr("context", "stack"),
			),
			err: errors.E(trigger.ErrParsing),
		},
		{
			name: "type not a keyword",
			body: Trigger(
				Str("ctime", "1000000"),
				Expr("reason", `["wrong"]`),
				Str("type", "changed"),
				Expr("context", "stack"),
			),
			err: errors.E(trigger.ErrParsing),
		},
		{
			name: "context not a keyword",
			body: Trigger(
				Str("ctime", "1000000"),
				Expr("reason", `["wrong"]`),
				Expr("type", "changed"),
				Str("context", "stack"),
			),
			err: errors.E(trigger.ErrParsing),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file := test.WriteFile(t, test.TempDir(t), "test-trigger.hcl", tc.body.String())
			_, err := trigger.ParseFile(file)
			errtest.Assert(t, err, tc.err, "when parsing: %s", tc.body)
		})
	}
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
