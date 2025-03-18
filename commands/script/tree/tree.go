package tree

import (
	"context"
	"fmt"
	"io"
	"path"
	"slices"
	"sort"
	"strings"

	"golang.org/x/exp/maps"

	hhcl "github.com/terramate-io/hcl/v2"

	"github.com/fatih/color"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/project"
)

// Spec is the "script tree" command specification.
type Spec struct {
	Engine     *engine.Engine
	WorkingDir string
	Printers   printer.Printers
}

// Name returns the name of the command.
func (s *Spec) Name() string { return "script tree" }

func (s *Spec) Exec(ctx context.Context) error {
	root := s.Engine.Config()
	srcpath := project.PrjAbsPath(root.HostDir(), s.WorkingDir)

	cfg, found := root.Lookup(srcpath)
	if !found {
		return nil
	}

	rootNode, topNode := addParentScriptTreeNodes(cfg, nil, true)
	addChildScriptTreeNodes(cfg, topNode)

	var sb strings.Builder
	rootNode.format(&sb, "", nil)
	s.Printers.Stdout.Println(sb.String())
	return nil
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
		fprintln(w, "#"+stackColor(text))
	} else {
		fprintln(w, text)
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
		fprintln(w, blockPrefix+"* "+scriptColor(sc.AccessorName()+": "))
		if sc.Name != nil {
			fprintln(w, blockPrefix+"  "+scriptColor("  Name: "+nameTruncation(exprString(sc.Name.Expr), "script.name")))
		}
		if sc.Description != nil {
			desc := exprString(sc.Description.Expr)
			fprintln(w, blockPrefix+"  "+scriptColor("  Description: "+desc))
		}
	}

	if node.IsStack {
		for _, p := range parentScripts {
			found := slices.ContainsFunc(node.Scripts,
				func(a *hcl.Script) bool {
					return a.AccessorName() == p
				})
			if !found {
				fprintln(w, blockPrefix+parentScriptColor("~ "+p))
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
			fprint(w, prefix+"└── ")
			child.format(w, prefix+"    ", parentScripts)
		} else {
			fprint(w, prefix+"├── ")
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

func fprint(w io.Writer, a ...any) {
	_, _ = fmt.Fprint(w, a...)
}

func fprintln(w io.Writer, a ...any) {
	_, _ = fmt.Fprintln(w, a...)
}

func exprString(expr hhcl.Expression) string {
	return string(ast.TokensForExpression(expr).Bytes())
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

func sortedKeys[V any](m map[string]V) []string {
	ks := maps.Keys(m)
	sort.Strings(ks)
	return ks
}
