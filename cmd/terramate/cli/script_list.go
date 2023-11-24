// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"sort"

	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/hcl"
	project2 "github.com/terramate-io/terramate/project"
)

type scriptListEntry struct {
	ScriptCfg *hcl.Script
	Dir       string
	DefCount  int
}

type scriptListMap map[string]*scriptListEntry

func (c *cli) printScriptList() {
	srcpath := project2.PrjAbsPath(c.rootdir(), c.wd())

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
		c.output.MsgStdOut("  %v", formatScriptDescription(entry.ScriptCfg))

		c.output.MsgStdOut("  Defined at %v", entry.Dir)

		if entry.DefCount > 1 {
			c.output.MsgStdOut("    (+%v more)", entry.DefCount-1)
		}

		c.output.MsgStdOut("")
	}
}

func addParentScriptListEntries(cfg *config.Tree, entries scriptListMap) {
	for _, sc := range cfg.Node.Scripts {
		scriptname := sc.Name()
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
	// Sort keys to reliably pick first sibling
	keys := make([]string, 0, len(cfg.Children))
	for k := range cfg.Children {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		childCfg := cfg.Children[k]
		for _, sc := range childCfg.Node.Scripts {
			scriptname := sc.Name()

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
