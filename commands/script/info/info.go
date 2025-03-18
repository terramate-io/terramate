// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package info

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	"golang.org/x/exp/maps"

	hhcl "github.com/terramate-io/hcl/v2"

	"github.com/fatih/color"
	"github.com/terramate-io/hcl/v2/hclsyntax"
	"github.com/terramate-io/terramate/cloud"
	cloudstack "github.com/terramate-io/terramate/cloud/stack"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/printer"
)

// Spec is the "script info" command specification.
type Spec struct {
	WorkingDir string
	Engine     *engine.Engine
	Printers   printer.Printers
	Labels     []string
}

// Name returns the name of the command.
func (s *Spec) Name() string { return "script info" }

func (s *Spec) Exec(ctx context.Context) error {
	labels := s.Labels
	entries, err := s.Engine.ListStacks(engine.NoGitFilter(), cloudstack.AnyTarget, cloud.NoStatusFilters(), false)
	if err != nil {
		return err
	}

	stacks := make(config.List[*config.SortableStack], len(entries.Stacks))
	for i, e := range entries.Stacks {
		stacks[i] = e.Stack.Sortable()
	}

	root := s.Engine.Config()
	m := newScriptsMatcher(labels)
	m.Search(root, stacks)

	if len(m.Results) == 0 {
		s.Printers.Stderr.Println(color.RedString("script not found: ") +
			strings.Join(s.Labels, " "))
		return errors.E("failed to execute command")
	}

	for _, x := range m.Results {
		s.Printers.Stdout.Println(fmt.Sprintf("Definition: %v", x.ScriptCfg.Range))
		if x.ScriptCfg.Name != nil {
			s.Printers.Stdout.Println(fmt.Sprintf("Name: %s", nameTruncation(exprString(x.ScriptCfg.Name.Expr), "script.name")))
		}
		if x.ScriptCfg.Description != nil {
			s.Printers.Stdout.Println(fmt.Sprintf("Description: %s", descTruncation(exprString(x.ScriptCfg.Description.Expr), "script.description")))
		}
		if len(x.Stacks) > 0 {
			s.Printers.Stdout.Println("Stacks:")
			for _, st := range x.Stacks {
				s.Printers.Stdout.Println(fmt.Sprintf("  %v", st.Dir()))
			}
		} else {
			s.Printers.Stdout.Println("Stacks: (none)")
		}

		s.Printers.Stdout.Println("Jobs:")
		for _, job := range x.ScriptCfg.Jobs {
			for cmdIdx, cmd := range formatScriptJob(job) {
				if cmdIdx == 0 {
					if job.Name != nil {
						s.Printers.Stdout.Println(fmt.Sprintf("  Name: %s", nameTruncation(exprString(job.Name.Expr), "script.job.name")))
					}
					if job.Description != nil {
						s.Printers.Stdout.Println(fmt.Sprintf("  Description: %s", descTruncation(exprString(job.Description.Expr), "script.job.description")))
					}
					s.Printers.Stdout.Println(fmt.Sprintf("  * %v", cmd))
				} else {
					s.Printers.Stdout.Println(fmt.Sprintf("    %v", cmd))
				}
			}
		}
		s.Printers.Stdout.Println("")
	}
	return nil
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

func exprString(expr hhcl.Expression) string {
	return string(ast.TokensForExpression(expr).Bytes())
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

func nameTruncation(name string, attrName string) string {
	if len(name) > config.MaxScriptNameRunes {
		printer.Stderr.Warn(
			fmt.Sprintf("`%s` exceeds the maximum allowed characters (%d): field truncated", attrName, config.MaxScriptNameRunes),
		)
		return name[:config.MaxScriptNameRunes] + "..."
	}
	return name
}

func descTruncation(name string, attrName string) string {
	if len(name) > config.MaxScriptDescRunes {
		printer.Stderr.Warn(
			fmt.Sprintf("`%s` exceeds the maximum allowed characters (%d): field truncated", attrName, config.MaxScriptNameRunes),
		)
		return name[:config.MaxScriptDescRunes] + "..."
	}
	return name
}
