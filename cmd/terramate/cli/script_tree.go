// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"fmt"
	"io"
	"path"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/hcl"
	prj "github.com/terramate-io/terramate/project"
	"golang.org/x/exp/slices"
)

func (c *cli) printScriptTree() {
	srcpath := prj.PrjAbsPath(c.rootdir(), c.wd())

	cfg, found := c.cfg().Lookup(srcpath)
	if !found {
		return
	}

	rootNode, topNode := addParentScriptTreeNodes(cfg, nil, true)
	addChildScriptTreeNodes(cfg, topNode)

	var sb strings.Builder
	rootNode.format(&sb, "", nil)
	c.output.MsgStdOut(sb.String())
}

type scriptsTreeNode struct {
	DirName  string
	IsStack  bool
	Scripts  []*hcl.Script
	Children []*scriptsTreeNode
	Parent   *scriptsTreeNode
	Visible  bool
}

func (node *scriptsTreeNode) format(w io.Writer, prefix string, parentScripts []string) {
	stackColor := color.New(color.FgGreen).SprintFunc()
	scriptColor := color.New(color.FgYellow).SprintFunc()
	parentScriptColor := color.New(color.Faint).SprintFunc()

	var text string
	if node.DirName != "" {
		text += node.DirName
	} else {
		text += "/"
	}

	if node.IsStack {
		fmt.Fprintln(w, "#"+stackColor(text))
	} else {
		fmt.Fprintln(w, text)
	}

	visibleChildren := make([]*scriptsTreeNode, 0, len(node.Children))
	for _, child := range node.Children {
		if child.Visible {
			visibleChildren = append(visibleChildren, child)
		}
	}

	blockPrefix := prefix
	if len(visibleChildren) > 0 {
		blockPrefix += "│ "
	} else {
		blockPrefix += "  "
	}

	for _, sc := range node.Scripts {
		fmt.Fprintln(w, blockPrefix+"* "+scriptColor(sc.AccessorName()+": "))
		if sc.Name != nil {
			fmt.Fprintln(w, blockPrefix+"  "+scriptColor("  Name: "+nameTruncation(exprString(sc.Name.Expr), "script.name")))
		}
		if sc.Description != nil {
			desc := exprString(sc.Description.Expr)
			fmt.Fprintln(w, blockPrefix+"  "+scriptColor("  Description: "+desc))
		}
	}

	if node.IsStack {
		for _, p := range parentScripts {
			found := slices.ContainsFunc(node.Scripts,
				func(a *hcl.Script) bool {
					return a.AccessorName() == p
				})
			if !found {
				fmt.Fprintln(w, blockPrefix+parentScriptColor("~ "+p))
			}
		}
	}

	for _, e := range node.Scripts {
		if !slices.Contains(parentScripts, e.AccessorName()) {
			parentScripts = append(parentScripts, e.AccessorName())
		}
	}

	sort.Strings(parentScripts)

	for i, child := range visibleChildren {
		if i == len(visibleChildren)-1 {
			fmt.Fprint(w, prefix+"└── ")
			child.format(w, prefix+"    ", parentScripts)
		} else {
			fmt.Fprint(w, prefix+"├── ")
			child.format(w, prefix+"│   ", parentScripts)
		}
	}
}

func addParentScriptTreeNodes(cfg *config.Tree, cur *scriptsTreeNode, selected bool) (root *scriptsTreeNode, top *scriptsTreeNode) {
	_, dirname := path.Split(cfg.Dir().String())
	if dirname == "" {
		dirname = "/"
	}

	thisNode := &scriptsTreeNode{
		DirName: dirname,
		IsStack: selected && cfg.IsStack(),
		Visible: true,
	}

	thisNode.Scripts = append(thisNode.Scripts, cfg.Node.Scripts...)

	if cur != nil {
		thisNode.Children = []*scriptsTreeNode{cur}
		cur.Parent = thisNode
	}

	if cfg.Parent != nil {
		rootNode, _ := addParentScriptTreeNodes(cfg.Parent, thisNode, false)
		return rootNode, thisNode
	}

	return thisNode, thisNode
}

func addChildScriptTreeNodes(cfg *config.Tree, cur *scriptsTreeNode) {
	for _, k := range sortedKeys(cfg.Children) {
		childCfg := cfg.Children[k]
		_, dirname := path.Split(childCfg.Dir().String())
		if dirname == "" {
			dirname = "/"
		}

		isStack := childCfg.IsStack()

		if isStack {
			p := cur
			for p != nil && !p.Visible {
				p.Visible = true
				p = p.Parent
			}
		}

		childNode := &scriptsTreeNode{
			DirName: dirname,
			IsStack: isStack,
			Visible: isStack,
			Parent:  cur,
		}

		childNode.Scripts = append(childNode.Scripts, childCfg.Node.Scripts...)
		cur.Children = append(cur.Children, childNode)

		addChildScriptTreeNodes(childCfg, childNode)
	}
}
