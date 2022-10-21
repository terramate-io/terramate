// Copyright 2022 Mineiros GmbH
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
	"strings"

	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl/fmt"
	"github.com/mineiros-io/terramate/project"
	"github.com/zclconf/go-cty/cty"
)

// ErrCannotExtendObject is the error when an object cannot be extended.
const ErrCannotExtendObject errors.Kind = "cannot extend object"

type (
	// Object is a object value supporting set at arbitrary paths using a
	// dot notation.
	//
	// Eg.:
	//   obj := cty.NewObject()
	//   obj.Set("val", cty.NewObject())
	//
	// The snippet above creates the object below:
	//   {
	//       val = {}
	//   }
	//
	// Then values can be set inside obj.val by doing:
	//
	//   obj.SetAt("val.test", 1)
	Object struct {
		origin project.Path
		// Keys is a map of key names to values.
		Keys map[string]Value
	}

	// Value is an evaluated value.
	Value interface {
		Origin() project.Path
		IsObject() bool
	}

	// CtyValue is a wrapper for a raw cty value.
	CtyValue struct {
		origin project.Path
		cty.Value
	}

	// DotPath represents a path inside the object using a dot-notation.
	DotPath string
)

// NewObject creates a new object.
func NewObject(origin project.Path) *Object {
	return &Object{
		origin: origin,
		Keys:   make(map[string]Value),
	}
}

// Set a key value into object.
func (obj *Object) Set(key string, value Value) {
	obj.Keys[key] = value
}

// GetKeyPath retrieves the value at path.
func (obj *Object) GetKeyPath(path DotPath) (interface{}, bool) {
	parts := strings.Split(string(path), ".")
	key := parts[0]
	next := DotPath(strings.Join(parts[1:], "."))

	v, ok := obj.Keys[key]
	if !ok {
		return nil, false
	}
	if next == "" {
		return v, true
	}
	if !v.IsObject() {
		return nil, false
	}

	return v.(*Object).GetKeyPath(next)
}

// Origin of the object.
func (obj *Object) Origin() project.Path { return obj.origin }

// IsObject returns true for [Object] values.
func (obj *Object) IsObject() bool { return true }

// SetFrom sets the object keys and values from the map.
func (obj *Object) SetFrom(values map[string]Value) *Object {
	for k, v := range values {
		if _, ok := obj.Keys[k]; ok {
			panic(errors.E("SetFrom failed: object has key %s", k))
		}
		obj.Set(k, v)
	}
	return obj
}

// SetFromCtyValues sets the object from the values map.
func (obj *Object) SetFromCtyValues(values map[string]cty.Value) *Object {
	for k, v := range values {
		_, marks := v.Unmark()
		var origin project.Path
		for mark := range marks {
			switch v := mark.(type) {
			case project.Path:
				origin = v
			default:
				panic("unreachable")
			}
		}
		if v.Type().IsObjectType() {
			subtree := NewObject(origin)
			subtree.SetFromCtyValues(v.AsValueMap())
			obj.Set(k, subtree)
		} else {
			obj.Set(k, NewCtyValue(v, origin))
		}
	}
	return obj
}

// SetAt sets a value at the specified path key.
func (obj *Object) SetAt(path DotPath, value Value) error {
	pathParts := strings.Split(string(path), ".")
	for len(pathParts) > 1 {
		key := pathParts[0]
		subobj, ok := obj.Keys[key]
		if !ok {
			subobj = NewObject(value.Origin())
			obj.Set(key, subobj)
		}
		if !subobj.IsObject() {
			return errors.E(ErrCannotExtendObject,
				"path part %s (from %s) contains non-object parts in the path (%s is %T)",
				key, path, key, subobj)
		}
		obj = subobj.(*Object)
		pathParts = pathParts[1:]
	}

	obj.Set(pathParts[0], value)
	return nil
}

// AsValueMap returns a map of string to Hashicorp cty.Value.
func (obj *Object) AsValueMap() map[string]cty.Value {
	vmap := map[string]cty.Value{}
	for k, v := range obj.Keys {
		switch vv := v.(type) {
		case *Object:
			subvmap := vv.AsValueMap()
			vmap[k] = cty.ObjectVal(subvmap)
		case CtyValue:
			vmap[k] = vv.Raw()
		default:
			panic("unreachable")
		}
	}
	return vmap
}

// String representation of the object.
func (obj *Object) String() string {
	return fmt.FormatAttributes(obj.AsValueMap())
}

// NewCtyValue creates a new cty.Value wrapper.
func NewCtyValue(val cty.Value, origin project.Path) CtyValue {
	return CtyValue{
		origin: origin,
		Value:  val,
	}
}

// NewValue returns a new object Value from a cty.Value.
// Note: this is not a wrapper as it returns an [Object] if val is a cty.Object.
func NewValue(val cty.Value, origin project.Path) Value {
	if val.Type().IsObjectType() {
		obj := NewObject(origin)
		obj.SetFromCtyValues(val.AsValueMap())
		return obj
	}
	return NewCtyValue(val, origin)
}

// Origin of the CtyValue val.
func (v CtyValue) Origin() project.Path { return v.origin }

// IsObject returns false for CtyValue values.
func (v CtyValue) IsObject() bool { return false }

// Raw returns the original cty.Value value.
func (v CtyValue) Raw() cty.Value {
	return v.Value
}
