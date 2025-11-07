// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package list provides the script list command.
package list

import (
	"context"
	"fmt"

	"sort"

	"golang.org/x/exp/maps"

	hhcl "github.com/terramate-io/hcl/v2"
	"github.com/terramate-io/terramate/commands"
	"github.com/terramate-io/terramate/commands/script"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/project"
)

// Spec represents the script list specification.
type Spec struct {
	engine     *engine.Engine
	printers   printer.Printers
	workingDir string
}

type scriptListEntry struct {
	ScriptCfg *hcl.Script
	Dir       string
	DefCount  int
}

type scriptListMap map[string]*scriptListEntry

// Name returns the name of the script list command.
func (s *Spec) Name() string { return "script list" }

// Requirements returns the requirements of the command.
func (s *Spec) Requirements(context.Context, commands.CLI) any {
	return commands.RequirementsList{
		commands.RequireEngine(),
		commands.RequireExperiments(script.Experiment),
	}
}

// Exec executes the script list command.
func (s *Spec) Exec(_ context.Context, cli commands.CLI) error {
	s.workingDir = cli.WorkingDir()
	s.engine = cli.Engine()
	s.printers = cli.Printers()

	root := s.engine.Config()
	srcpath := project.PrjAbsPath(root.HostDir(), s.workingDir)

	cfg, found := root.Lookup(srcpath)
	if !found {
		return nil
	}

	entries := scriptListMap{}

	addParentScriptListEntries(cfg, entries)
	addChildScriptListEntries(cfg, entries)

	for _, name := range sortedKeys(entries) {
		entry := entries[name]

		s.printers.Stdout.Println(name)
		if entry.ScriptCfg.Name != nil {
			s.printers.Stdout.Println(fmt.Sprintf("  Name: %v", nameTruncation(exprString(entry.ScriptCfg.Name.Expr), "script.name")))
		}
		if entry.ScriptCfg.Description != nil {
			s.printers.Stdout.Println(fmt.Sprintf("  Description: %v", exprString(entry.ScriptCfg.Description.Expr)))
		}
		s.printers.Stdout.Println(fmt.Sprintf("  Defined at %v", entry.Dir))

		if entry.DefCount > 1 {
			s.printers.Stdout.Println(fmt.Sprintf("    (+%v more)", entry.DefCount-1))
		}
		s.printers.Stdout.Println("")
	}
	return nil
}

func addParentScriptListEntries(cfg *config.Tree, entries scriptListMap) {
	for _, sc := range cfg.Node.Scripts {
		scriptname := sc.AccessorName()
		defcount := 1
		olddef := entries[scriptname]
		if olddef != nil {
			defcount = olddef.DefCount + 1
		}

		// Towards parents we replace child definitions
		entries[scriptname] = &scriptListEntry{
			ScriptCfg: sc,
			Dir:       cfg.Dir().String(),
			DefCount:  defcount,
		}
	}

	if cfg.Parent != nil {
		addParentScriptListEntries(cfg.Parent, entries)
	}
}

func addChildScriptListEntries(cfg *config.Tree, entries scriptListMap) {
	for _, k := range sortedKeys(cfg.Children) {
		childCfg := cfg.Children[k]
		for _, sc := range childCfg.Node.Scripts {
			scriptname := sc.AccessorName()

			olddef := entries[scriptname]
			if olddef != nil {
				// Towards children we keep parent definitions
				olddef.DefCount++
				continue
			}

			entries[scriptname] = &scriptListEntry{
				ScriptCfg: sc,
				Dir:       childCfg.Dir().String(),
				DefCount:  1,
			}
		}

		addChildScriptListEntries(childCfg, entries)
	}
}

func sortedKeys[V any](m map[string]V) []string {
	ks := maps.Keys(m)
	sort.Strings(ks)
	return ks
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

func exprString(expr hhcl.Expression) string {
	return string(ast.TokensForExpression(expr).Bytes())
}
