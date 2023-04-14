package globals3

import (
	"bytes"
	"strconv"

	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/mineiros-io/terramate/errors"
	"github.com/zclconf/go-cty/cty"
)

type (
	// Ref is a Terramate variable reference.
	// It implements the `dot operator` or `member access` syntaxex like:
	//   global.a.b
	//   global[a][b]
	// In the examples above, the `global` is the Object and Path is `["a", "b"]`.
	Ref struct {
		Object string
		Path   []string
	}

	RefStr string

	// Refs is a list of references.
	Refs []Ref
)

func (ref Ref) AsKey() RefStr { return RefStr(ref.String()) }

// String returns a string representation of the Ref.
// Note that it does not represent the syntactic ref found in the source file.
// This is an internal representation that better fits the implementation design.
func (ref Ref) String() string {
	var out bytes.Buffer

	// NOTE: the buffer methods never return errors and always write the full content.
	// it panics if more memory cannot be allocated.
	out.WriteString(ref.Object)
	for _, p := range ref.Path {
		out.WriteRune('[')
		out.WriteString(strconv.Quote(p))
		out.WriteRune(']')
	}
	return out.String()
}

func (ref Ref) has(other Ref) bool {
	if ref.Object != other.Object {
		return false
	}
	if len(ref.Path) < len(other.Path) {
		return false
	}
	var max int
	if len(ref.Path) == len(other.Path) {
		max = len(ref.Path)
	} else {
		max = len(other.Path)
	}

	for i := 0; i < max; i++ {
		if ref.Path[i] != other.Path[i] {
			return false
		}
	}
	return true
}

// equal tells if two refs are the same.
func (ref Ref) equal(other Ref) bool {
	if ref.Object != other.Object || len(ref.Path) != len(other.Path) {
		return false
	}
	for i, p := range ref.Path {
		if other.Path[i] != p {
			return false
		}
	}
	return true
}

func (refs Refs) equal(other Refs) bool {
	if len(refs) != len(other) {
		return false
	}
	for i, ref := range refs {
		if !ref.equal(other[i]) {
			return false
		}
	}
	return true
}

// refsOf returns a distinct set of Refs contained in the expression.
func refsOf(expr hhcl.Expression) Refs {
	ret := Refs{}
	uniqueRefs := map[string]Ref{}
	for _, trav := range expr.Variables() {
		// they are all root traversals
		ref := Ref{
			Object: trav[0].(hhcl.TraverseRoot).Name,
		}

	inner:
		for _, tt := range trav[1:] {
			switch t := tt.(type) {
			case hhcl.TraverseAttr:
				ref.Path = append(ref.Path, t.Name)
			case hhcl.TraverseSplat:
				break inner
			case hhcl.TraverseIndex:
				if !t.Key.Type().Equals(cty.String) {
					break inner
				}
				ref.Path = append(ref.Path, t.Key.AsString())
			default:
				panic(errors.E(errors.ErrInternal, "unexpected traversal: %v", t))
			}
		}

		if _, ok := uniqueRefs[ref.String()]; !ok {
			uniqueRefs[ref.String()] = ref
			ret = append(ret, ref)
		}
	}
	return ret
}
