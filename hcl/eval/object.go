// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package eval

import (
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/fmt"
	"github.com/terramate-io/terramate/project"
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
	target, key, err := computeTargetFrom(obj, path, value.Info())
	if err != nil {
		return err
	}

	target.Set(key, value)
	return nil
}

// MergeFailsIfKeyExists merge the value into obj but fails if any key in value exists in
// obj.
func (obj *Object) MergeFailsIfKeyExists(path ObjectPath, value Value) error {
	target, key, err := computeTargetFrom(obj, path, value.Info())
	if err != nil {
		return err
	}

	old, ok := target.GetKeyPath([]string{key})
	if !ok {
		target.Set(key, value)
		return nil
	}

	if old.IsObject() != value.IsObject() {
		return errors.E("failed to merge object and value for key path %v", key)
	}
	if !value.IsObject() {
		return errors.E("cannot overwrite key path %v", key)
	}
	valObj := value.(*Object)
	oldObj := old.(*Object)
	for k, v := range valObj.Keys {
		_, ok := oldObj.GetKeyPath([]string{k})
		if ok {
			return errors.E("cannot overwrite")
		}
		err := oldObj.SetAt([]string{k}, v)
		if err != nil {
			return err
		}
	}
	return nil
}

// MergeOverwrite merges value into obj by overwriting each key.
func (obj *Object) MergeOverwrite(path ObjectPath, value Value) error {
	target, key, err := computeTargetFrom(obj, path, value.Info())
	if err != nil {
		return err
	}

	old, ok := target.GetKeyPath([]string{key})
	if !ok {
		target.Set(key, value)
		return nil
	}

	if old.IsObject() != value.IsObject() {
		target.Set(key, value)
		return nil
	}
	if !old.IsObject() {
		target.Set(key, value)
		return nil
	}
	valObj := value.(*Object)
	oldObj := old.(*Object)
	for k, v := range valObj.Keys {
		err := oldObj.SetAt([]string{k}, v)
		if err != nil {
			return err
		}
	}
	return nil
}

// MergeNewKeys merge the keys from value that doesn't exist in obj.
func (obj *Object) MergeNewKeys(path ObjectPath, value Value) error {
	target, key, err := computeTargetFrom(obj, path, value.Info())
	if err != nil {
		return err
	}

	old, ok := target.GetKeyPath([]string{key})
	if !ok {
		target.Set(key, value)
		return nil
	}

	if old.IsObject() != value.IsObject() {
		return errors.E("failed to merge object and value")
	}
	if !value.IsObject() {
		return errors.E("cannot overwrite")
	}
	valObj := value.(*Object)
	oldObj := old.(*Object)
	for k, v := range valObj.Keys {
		_, ok := oldObj.GetKeyPath([]string{k})
		if !ok {
			err := oldObj.SetAt([]string{k}, v)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func computeTargetFrom(obj *Object, path ObjectPath, info Info) (*Object, string, error) {
	for len(path) > 1 {
		key := path[0]
		subobj, ok := obj.Keys[key]
		if !ok {
			subobj = NewObject(info)
			obj.Set(key, subobj)
		}
		if !subobj.IsObject() {
			return nil, "", errors.E(ErrCannotExtendObject,
				"path part %s (from %s) contains non-object parts in the path (%v is %T)",
				key, path, key, subobj)
		}
		obj = subobj.(*Object)
		path = path[1:]
	}
	return obj, path[0], nil
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
	val, _ := v.Unmark()
	return val
}
