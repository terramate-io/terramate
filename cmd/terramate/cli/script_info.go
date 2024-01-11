// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"os"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/hcl/ast"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

func (c *cli) printScriptInfo() {
	labels := c.parsedArgs.Script.Info.Cmds

	stacks, err := c.computeSelectedStacks(false)
	if err != nil {
		fatal(err, "computing selected stacks")
	}

	m := newScriptsMatcher(labels)
	m.Search(c.cfg(), stacks)

	if len(m.Results) == 0 {
		c.output.MsgStdErr(color.RedString("script not found: ") +
			strings.Join(c.parsedArgs.Script.Info.Cmds, " "))
		os.Exit(1)
	}

	for _, x := range m.Results {
		c.output.MsgStdOut("Definition: %v", x.ScriptCfg.Range)
		c.output.MsgStdOut("Description: %v", formatScriptDescription(x.ScriptCfg))

		if len(x.Stacks) > 0 {
			c.output.MsgStdOut("Stacks:")
			for _, st := range x.Stacks {
				c.output.MsgStdOut("  %v", st.Dir())
			}
		} else {
			c.output.MsgStdOut("Stacks: (none)")
		}

		c.output.MsgStdOut("Jobs:")
		for _, job := range x.ScriptCfg.Jobs {
			for cmdIdx, cmd := range formatScriptJob(job) {
				if cmdIdx == 0 {
					c.output.MsgStdOut("  * %v", cmd)
				} else {
					c.output.MsgStdOut("    %v", cmd)
				}
			}
		}

		c.output.MsgStdOut("")
	}
}

type scriptInfoEntry struct {
	ScriptCfg *hcl.Script
	Stacks    config.List[*config.SortableStack]
}

type scriptsMatcher struct {
	labels []string

	Results []*scriptInfoEntry
}

func newScriptsMatcher(labels []string) *scriptsMatcher {
	return &scriptsMatcher{
		labels:  labels,
		Results: nil,
	}
}

func (m *scriptsMatcher) Search(cfg *config.Root, stacks config.List[*config.SortableStack]) {
	rootScope := make(config.List[*config.SortableStack], len(stacks))
	copy(rootScope, stacks)

	m.searchRecursive(cfg.Tree(), &rootScope)
}

func tryGetScriptByName(cfg *config.Tree, labels []string) *hcl.Script {
	for _, s := range cfg.Node.Scripts {
		if slices.Equal(s.Labels, labels) {
			return s
		}
	}
	return nil
}

func sortedKeys[V any](m map[string]V) []string {
	ks := maps.Keys(m)
	sort.Strings(ks)
	return ks
}

func (m *scriptsMatcher) searchRecursive(cfg *config.Tree, scope *config.List[*config.SortableStack]) {
	scriptCfg := tryGetScriptByName(cfg, m.labels)
	if scriptCfg != nil {
		m.addResult(cfg, scriptCfg, scope)
	} else {
		for _, k := range sortedKeys(cfg.Children) {
			m.searchRecursive(cfg.Children[k], scope)
		}
	}
}

func (m *scriptsMatcher) addResult(cfg *config.Tree, scriptCfg *hcl.Script, scope *config.List[*config.SortableStack]) {
	outerScope := config.List[*config.SortableStack]{}
	innerScope := config.List[*config.SortableStack]{}

	cfgdir := cfg.Dir()
	for _, st := range *scope {
		stackdir := st.Dir()

		if stackdir.HasDirPrefix(cfgdir.String()) {
			innerScope = append(innerScope, st)
		} else {
			outerScope = append(outerScope, st)
		}
	}

	// Modify the current scope
	*scope = outerScope

	newEntry := &scriptInfoEntry{
		ScriptCfg: scriptCfg,
		// This may be updated at a deeper level as we move stacks to children
		Stacks: innerScope,
	}
	m.Results = append(m.Results, newEntry)

	for _, k := range sortedKeys(cfg.Children) {
		m.searchRecursive(cfg.Children[k], &newEntry.Stacks)
	}
}

func formatScriptDescription(cfg *hcl.Script) string {
	return string(ast.TokensForExpression(cfg.Description.Expr).Bytes())
}

func formatScriptJob(job *hcl.ScriptJob) []string {
	if job.Commands != nil {
		switch e := job.Commands.Expr.(type) {
		case *hclsyntax.TupleConsExpr:
			commands := []string{}
			for _, cmd := range e.Exprs {
				commands = append(commands, string(ast.TokensForExpression(cmd).Bytes()))
			}
			return commands

		default:
			return []string{string(ast.TokensForExpression(e).Bytes())}
		}

	} else if job.Command != nil {
		return []string{string(ast.TokensForExpression(job.Command.Expr).Bytes())}
	}

	return []string{}
}
