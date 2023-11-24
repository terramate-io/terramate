// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"reflect"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/hcl/ast"
)

func (c *cli) printScriptInfo(labels []string) {
	stacks, err := c.computeSelectedStacks(false)
	if err != nil {
		fatal(err, "computing selected stacks")
	}

	m := newScriptsMatcher(labels)
	m.Search(c.cfg().Tree(), &stacks)

	for _, x := range m.Results {
		c.output.MsgStdOut("Definition: %v", x.ScriptCfg.Range)
		c.output.MsgStdOut("Description: %v", formatScriptDescription(x.ScriptCfg))
		c.output.MsgStdOut("Stacks:")
		for _, st := range x.Stacks {
			c.output.MsgStdOut("  %v", st.Dir())
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

func (m *scriptsMatcher) Search(cfg *config.Tree, stacks *config.List[*config.SortableStack]) {
	m.searchRecursive(cfg, stacks)
}

func tryGetScriptByName(cfg *config.Tree, labels []string) *hcl.Script {
	for _, s := range cfg.Node.Scripts {
		if reflect.DeepEqual(s.Labels, labels) {
			return s
		}
	}
	return nil
}

func sortedKeys[V any](m map[string]V) []string {
	r := make([]string, 0, len(m))
	for k := range m {
		r = append(r, k)
	}
	sort.Strings(r)
	return r
}

func (m *scriptsMatcher) searchRecursive(cfg *config.Tree, stacks *config.List[*config.SortableStack]) {
	scriptCfg := tryGetScriptByName(cfg, m.labels)
	if scriptCfg != nil {
		m.addResult(cfg, scriptCfg, stacks)
	} else {
		for _, k := range sortedKeys(cfg.Children) {
			m.searchRecursive(cfg.Children[k], stacks)
		}
	}
}

func (m *scriptsMatcher) addResult(cfg *config.Tree, scriptCfg *hcl.Script, stacks *config.List[*config.SortableStack]) {
	parentStacks := config.List[*config.SortableStack]{}
	myStacks := config.List[*config.SortableStack]{}

	cfgdir := cfg.Dir()
	for _, st := range *stacks {
		stackdir := st.Dir()

		if stackdir.HasPrefix(cfgdir.String()) {
			myStacks = append(myStacks, st)
		} else {
			parentStacks = append(parentStacks, st)
		}
	}

	// Update the parent stacks
	*stacks = parentStacks

	newEntry := &scriptInfoEntry{
		ScriptCfg: scriptCfg,
		// This may be updated in the next block as we move stacks to children
		Stacks: myStacks,
	}
	m.Results = append(m.Results, newEntry)

	for _, k := range sortedKeys(cfg.Children) {
		m.searchRecursive(cfg.Children[k], &newEntry.Stacks)
	}
}

func formatScriptDescription(cfg *hcl.Script) string {
	toks := ast.TokensForExpression(cfg.Description.Expr)
	s := string(toks.Bytes())
	s = strings.Trim(s, "\"")
	return s
}

func formatScriptJob(job *hcl.ScriptJob) []string {
	commands := []string{}

	if job.Commands != nil {
		switch e := job.Commands.Expr.(type) {
		case *hclsyntax.TupleConsExpr:
			for _, cmd := range e.Exprs {
				toks := ast.TokensForExpression(cmd)
				commands = append(commands, string(toks.Bytes()))
			}

		}
	} else if job.Command != nil {
		toks := ast.TokensForExpression(job.Command.Expr)
		commands = append(commands, string(toks.Bytes()))
	} else {
		return commands
	}

	return commands
}
