// Copyright 2023 Mineiros GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package eval

import (
	"bytes"
	"strconv"

	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/terramate-io/terramate/errors"
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

		Range hhcl.Range
	}

	// RefStr is a string representation of the ref used as map keys.
	RefStr string

	// Refs is a list of references.
	Refs []Ref
)

// NewRef creates a new reference.
// The provided accessor is copied, and then safe to be modified.
func NewRef(varname string, accessor ...string) Ref {
	r := Ref{
		Object: varname,
		Path:   accessor,
	}
	r.Path = make([]string, len(accessor))
	copy(r.Path, accessor)
	return r
}

// AsKey returns a ref suitable to be used as a map key.
func (ref Ref) AsKey() RefStr { return RefStr(ref.String()) }

// Comb returns all sub references of this one.
func (ref Ref) Comb() Refs {
	refs := Refs{}
	for i := len(ref.Path) - 1; i >= 0; i-- {
		newRef := NewRef(ref.Object, ref.Path[:i+1]...)
		refs = append(refs, newRef)
	}
	return refs
}

// LastAccessor returns the last part of the accessor.
// Eg.: for `global.a.b.c` it returns "c".
// Eg.: for `global` it returns "global".
func (ref Ref) LastAccessor() string {
	if len(ref.Path) == 0 {
		return ref.Object
	}
	return ref.Path[len(ref.Path)-1]
}

// String returns a string representation of the Ref.
// Note that it does not represent the syntactic ref found in the source file.
// This is an internal representation that better fits the implementation design.
func (ref Ref) String() string {
	var out bytes.Buffer
	out.Grow(
		len(ref.Object) +
			len(ref.Path)*10 + /* average key size */
			+len(ref.Path)*2, /* brackets */
	)

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

// Has returns true if ref contains the other ref and false otherwise.
func (ref Ref) Has(other Ref) bool {
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
func refsOf(expr hhcl.Expression) (Refs, map[string]Ref) {
	ret := Refs{}
	uniqueRefs := map[string]Ref{}
	refsObjects := map[string]Ref{}
	for _, trav := range expr.Variables() {
		// they are all root traversals
		ref := Ref{
			Object: trav[0].(hhcl.TraverseRoot).Name,
			Range:  trav.SourceRange(),
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
			refsObjects[ref.Object] = ref
			ret = append(ret, ref)
		}
	}
	return ret, refsObjects
}
