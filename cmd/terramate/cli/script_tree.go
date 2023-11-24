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
	project2 "github.com/terramate-io/terramate/project"
	"golang.org/x/exp/slices"
)

func (c *cli) printScriptTree() {
	srcpath := project2.PrjAbsPath(c.rootdir(), c.wd())

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
	DirName   string
	StackName string
	Scripts   []*hcl.Script
	Children  []*scriptsTreeNode
	Parent    *scriptsTreeNode
	Visible   bool
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
	fmt.Fprintln(w, text)

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

	if node.StackName != "" {
		fmt.Fprintln(w, blockPrefix+"# "+stackColor(node.StackName))
	}

	for _, sc := range node.Scripts {
		desc := formatScriptDescription(sc)
		fmt.Fprintln(w, blockPrefix+"* "+scriptColor(sc.Name()+": "+desc))
	}

	if node.StackName != "" {
		for _, p := range parentScripts {
			_, found := slices.BinarySearchFunc(node.Scripts, p,
				func(a *hcl.Script, b string) int {
					return strings.Compare(a.Name(), b)
				})
			if !found {
				fmt.Fprintln(w, blockPrefix+parentScriptColor("~ "+p))
			}
		}
	}

	for _, e := range node.Scripts {
		_, found := slices.BinarySearch(parentScripts, e.Name())
		if !found {
			parentScripts = append(parentScripts, e.Name())
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

	var stackname string
	if selected && cfg.IsStack() {
		stackname = cfg.Node.Stack.Name
	}

	thisNode := &scriptsTreeNode{
		DirName:   dirname,
		StackName: stackname,
		Visible:   true,
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

		var stackname string
		if isStack {
			stackname = childCfg.Node.Stack.Name

			p := cur
			for p != nil && !p.Visible {
				p.Visible = true
				p = p.Parent
			}
		}

		childNode := &scriptsTreeNode{
			DirName:   dirname,
			StackName: stackname,
			Visible:   isStack,
			Parent:    cur,
		}

		childNode.Scripts = append(childNode.Scripts, childCfg.Node.Scripts...)

		cur.Children = append(cur.Children, childNode)

		addChildScriptTreeNodes(childCfg, childNode)
	}
}
