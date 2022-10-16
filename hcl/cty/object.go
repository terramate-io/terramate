package cty

import (
	"strings"

	"github.com/mineiros-io/terramate/errors"
)

// ErrCannotExtendObject is the error when an object cannot be extended.
const ErrCannotExtendObject errors.Kind = "cannot extend object"

type (
	// Object is a object value supporting set at arbitrary paths.
	Object struct {
		// Keys is a map of key names to values.
		Keys map[string]interface{}
	}
)

// NewObject creates a new object.
func NewObject() *Object {
	return &Object{
		Keys: make(map[string]interface{}),
	}
}

// Set a key value.
func (obj *Object) Set(key string, value interface{}) {
	obj.Keys[key] = value
}

// SetAt sets a value at the specified path key.
func (obj *Object) SetAt(path string, value interface{}) error {
	pathParts := strings.Split(path, ".")
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
				"path %s contains non-object parts in the path", path)
		}
		obj = v
		pathParts = pathParts[1:]
	}

	obj.Keys[pathParts[0]] = value
	return nil
}
