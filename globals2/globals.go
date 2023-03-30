package globals2

import "github.com/zclconf/go-cty/cty"

type Variables map[Accessor]cty.Value

type Query struct {
	Variables Variables
	Evaluated int
}

func EvalFor