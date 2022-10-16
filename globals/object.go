package globals

import (
	"strings"

	"github.com/mineiros-io/terramate/errors"
	"github.com/zclconf/go-cty/cty"
)

type object struct {
	name string
	val  cty.Value
}

func newObject(name string, val map[string]cty.Value) object {
	return object{
		val:  cty.ObjectVal(val),
		name: name,
	}
}

func (obj *object) Get(path string) (cty.Value, bool, error) {
	components := strings.Split(path, ".")
	val := obj.val
	for i, comp := range components {
		if !val.Type().IsObjectType() {
			return cty.NilVal, false, errors.E("the value %s.%s is of type %s "+
				" and GetAttr object indexing as %s.%s is not allowed",
				obj.name, strings.Join(components[0:i], "."),
				strings.Join(components, "."))
		}

		mapval := val.AsValueMap()
		v, ok := mapval[comp]
		if !ok {
			return cty.NilVal, false, nil
		}
		val = v
	}
	return val, true, nil
}
