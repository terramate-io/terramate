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
	// Object is an object container for cty.Value values supporting set at
	// arbitrary accessor paths.
	//
	// Eg.:
	//   obj := eval.NewObject(origin)
	//   obj.Set("val", eval.NewObject())
	//
	// The snippet above creates the object below:
	//   {
	//       val = {}
	//   }
	//
	// Then values can be set inside obj.val by doing:
	//
	//   obj.SetAt(ObjectPath{"val", "test"}, eval.NewValue(cty.StringVal("test"), origin))
	//
	// Of which creates the object below:
	//
	//   {
	//       val = {
	//           test = "test"
	//       }
	//   }
	Object struct {
		origin Info
		// Keys is a map of key names to values.
		Keys map[string]Value
	}

	// Value is an evaluated value.
	Value interface {
		// Info provides extra information for the value.
		Info() Info

		// IsObject tells if the value is an object.
		IsObject() bool
	}

	// CtyValue is a wrapper for a raw cty value.
	CtyValue struct {
		origin Info
		cty.Value
	}

	// Info provides extra information for the configuration value.
	Info struct {
		// Dir defines the directory from there the value is being instantiated,
		// which means it's the scope directory (not the file where it's defined).
		// For values that comes from imports, the Dir will be the directory
		// which imports the value.
		Dir project.Path

		// DefinedAt provides the source file where the value is defined.
		DefinedAt project.Path
	}

	// ObjectPath represents a path inside the object.
	ObjectPath []string
)

// NewObject creates a new object for configdir directory.
func NewObject(origin Info) *Object {
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
func (obj *Object) GetKeyPath(path ObjectPath) (Value, bool) {
	key := path[0]
	next := path[1:]

	v, ok := obj.Keys[key]
	if !ok {
		return nil, false
	}
	if len(next) == 0 {
		return v, true
	}
	if !v.IsObject() {
		return nil, false
	}

	return v.(*Object).GetKeyPath(next)
}

// Info provides extra information for the object value.
func (obj *Object) Info() Info { return obj.origin }

// IsObject returns true for [Object] values.
func (obj *Object) IsObject() bool { return true }

// SetFrom sets the object keys and values from the map.
func (obj *Object) SetFrom(values map[string]Value) *Object {
	for k, v := range values {
		obj.Set(k, v)
	}
	return obj
}

// SetFromCtyValues sets the object from the values map.
func (obj *Object) SetFromCtyValues(values map[string]cty.Value, origin Info) *Object {
	for k, v := range values {
		if v.Type().IsObjectType() {
			subtree := NewObject(origin)
			subtree.SetFromCtyValues(v.AsValueMap(), origin)
			obj.Set(k, subtree)
		} else {
			obj.Set(k, NewCtyValue(v, origin))
		}
	}
	return obj
}

// SetAt sets a value at the specified path key.
func (obj *Object) SetAt(path ObjectPath, value Value) error {
	for len(path) > 1 {
		key := path[0]
		subobj, ok := obj.Keys[key]
		if !ok {
			subobj = NewObject(value.Info())
			obj.Set(key, subobj)
		}
		if !subobj.IsObject() {
			return errors.E(ErrCannotExtendObject,
				"path part %s (from %s) contains non-object parts in the path (%v is %T)",
				key, path, key, subobj)
		}
		obj = subobj.(*Object)
		path = path[1:]
	}

	old, ok := obj.GetKeyPath(path)
	if ok {
		// When a parent global depend on child globals or stack metadata,
		// they are set in the globals object after all dependencies were evaluated,
		// which means the child globals can be set before the parent ones,
		// breaking the parent -> child hierarchical order.
		// Have a look in the example below:
		//   # parent/file.tm
		//   globals {
		//       a = {
		//           b = global.b
		//       }
		//   }
		//
		//   # parent/child/file.tm
		//   globals a {
		//       c = 1
		//   }
		//
		//   globals {
		//       b = 1
		//   }
		//
		// In this case, the parent global.a evaluation is postponed to happen
		// after global.b is evaluated. When the child globals are evaluated,
		// they set the temporary globals object as:
		//
		//   {
		//       a = {
		//           c = 1
		//       }
		//       b = 1
		//   }
		//
		// Then later the postponed parent global.a evaluation succeeds and we
		// have the value below to be merged in globals object:
		//
		//   {
		//       a = {
		//           b = 1
		//       }
		//   }
		//
		// But we cannot overwrite the globals.a because it comes from the child
		// (that must overwrite parent scopes), so the solution is to look for
		// where's the definition of each value and overwrite the values if they
		// come from a child scope or ignore new values that comes from the parent
		// but conflicts with definitions in the child.
		if !strings.HasPrefix(value.Info().Dir.String(), old.Info().Dir.String()) {
			// old is from a child scope, we must recursively merge the objects
			// or ignore the value as it was overwriten by the child scope.
			if old.IsObject() && value.IsObject() {
				oldv := old.(*Object)
				for k, v := range value.(*Object).Keys {
					err := oldv.SetAt([]string{k}, v)
					if err != nil {
						return err
					}
				}
			}
			return nil
		}
	}

	obj.Set(path[0], value)
	return nil
}

// DeleteAt deletes the value at the specified path.
func (obj *Object) DeleteAt(path ObjectPath) error {
	for len(path) > 1 {
		key := path[0]
		subobj, ok := obj.Keys[key]
		if !ok {
			return nil
		}
		if !subobj.IsObject() {
			return errors.E(ErrCannotExtendObject,
				"path part %s (from %v) contains non-object parts in the path (%s is %T)",
				key, path, key, subobj)
		}
		obj = subobj.(*Object)
		path = path[1:]
	}

	delete(obj.Keys, path[0])
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
// Note: The cty.Value val is marked with the origin path and must be unmarked
// before use with any hashicorp API otherwise it panics.
func NewCtyValue(val cty.Value, origin Info) CtyValue {
	val = val.Mark(origin)
	return CtyValue{
		origin: origin,
		Value:  val,
	}
}

// NewValue returns a new object Value from a cty.Value.
// Note: this is not a wrapper as it returns an [Object] if val is a cty.Object.
func NewValue(val cty.Value, origin Info) Value {
	if val.Type().IsObjectType() {
		obj := NewObject(origin)
		obj.SetFromCtyValues(val.AsValueMap(), origin)
		return obj
	}
	return NewCtyValue(val, origin)
}

// Info provides extra information for the value.
func (v CtyValue) Info() Info { return v.origin }

// IsObject returns false for CtyValue values.
func (v CtyValue) IsObject() bool { return false }

// Raw returns the original cty.Value value (unmarked).
func (v CtyValue) Raw() cty.Value {
	val, _ := v.Value.Unmark()
	return val
}
