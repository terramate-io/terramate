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

package cty

import (
	"fmt"
	"strings"

	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/project"
	"github.com/zclconf/go-cty/cty"
)

// ErrCannotExtendObject is the error when an object cannot be extended.
const ErrCannotExtendObject errors.Kind = "cannot extend object"

type (
	// Object is a object value supporting set at arbitrary paths.
	Object struct {
		// Keys is a map of key names to values.
		Keys map[string]interface{}
	}

	// Value is an evaluated global.
	Value struct {
		Origin project.Path

		val cty.Value
	}

	DotPath string
)

// NewObject creates a new object.
func NewObject() *Object {
	return &Object{
		Keys: make(map[string]interface{}),
	}
}

// Set a key value.
func (obj *Object) Set(key string, value interface{}) {
	if vvalue, ok := value.(Value); ok {
		if vvalue.Raw().Type().IsObjectType() {
			newobj := NewObject()
			newobj.SetFrom(vvalue.Raw().AsValueMap())
			value = newobj
		}
	}
	obj.Keys[key] = value
}

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
	subobj, ok := v.(*Object)
	if !ok {
		return nil, false
	}

	return subobj.GetKeyPath(next)
}

func (obj *Object) SetFrom(values map[string]cty.Value) {
	for k, v := range values {
		if v.Type().IsObjectType() {
			subtree := NewObject()
			subtree.SetFrom(v.AsValueMap())
			obj.Set(k, subtree)
		} else {
			obj.Set(k, v)
		}
	}
}

// SetAt sets a value at the specified path key.
func (obj *Object) SetAt(path DotPath, value interface{}) error {
	pathParts := strings.Split(string(path), ".")
	for len(pathParts) > 1 {
		key := pathParts[0]
		subobj, ok := obj.Keys[key]
		if !ok {
			subobj = NewObject()
			obj.Keys[key] = subobj
		}
		v, ok := subobj.(*Object)
		if !ok {
			return errors.E(ErrCannotExtendObject,
				"path %s contains non-object parts in the path (%s is %T)",
				path, key, subobj)
		}
		obj = v
		pathParts = pathParts[1:]
	}

	obj.Set(pathParts[0], value)
	return nil
}

func (obj *Object) AsValueMap() map[string]cty.Value {
	vmap := map[string]cty.Value{}
	for k, v := range obj.Keys {
		switch vv := v.(type) {
		case *Object:
			subvmap := vv.AsValueMap()
			vmap[k] = cty.ObjectVal(subvmap)
		case Value:
			vmap[k] = vv.Raw()
		case cty.Value:
			vmap[k] = vv
		default:
			panic(fmt.Errorf("%T %v", vv, vv))
		}
	}
	return vmap
}

func (obj *Object) String() string {
	return hcl.FormatAttributes(obj.AsValueMap())
}

func NewValue(val cty.Value, origin project.Path) Value {
	return Value{
		val:    val,
		Origin: origin,
	}
}

func (v Value) Raw() cty.Value {
	return v.val
}
