package globals2

import (
	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/mineiros-io/terramate/config"
	"github.com/zclconf/go-cty/cty"
)

type G struct {
	tree *config.Tree
}

func New(tree *config.Tree) *G {
	return &G{
		tree: tree,
	}
}

func (g *G) Eval(expr hhcl.Expression) (cty.Value, error) {

}
