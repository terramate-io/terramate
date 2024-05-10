// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/hcl"
	prj "github.com/terramate-io/terramate/project"
)

type scriptListEntry struct {
	ScriptCfg *hcl.Script
	Dir       string
	DefCount  int
}

type scriptListMap map[string]*scriptListEntry

func (c *cli) printScriptList() {
	srcpath := prj.PrjAbsPath(c.rootdir(), c.wd())

	cfg, found := c.cfg().Lookup(srcpath)
	if !found {
		return
	}

	entries := scriptListMap{}

	addParentScriptListEntries(cfg, entries)
	addChildScriptListEntries(cfg, entries)

	for _, name := range sortedKeys(entries) {
		entry := entries[name]

		c.output.MsgStdOut("%v", name)
		if entry.ScriptCfg.Name != nil {
			c.output.MsgStdOut("  Name: %v", nameTruncation(exprString(entry.ScriptCfg.Name.Expr), "script.name"))
		}
		if entry.ScriptCfg.Description != nil {
			c.output.MsgStdOut("  Description: %v", exprString(entry.ScriptCfg.Description.Expr))
		}
		c.output.MsgStdOut("  Defined at %v", entry.Dir)

		if entry.DefCount > 1 {
			c.output.MsgStdOut("    (+%v more)", entry.DefCount-1)
		}

		c.output.MsgStdOut("")
	}
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
