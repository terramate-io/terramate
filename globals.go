// Copyright 2021 Mineiros GmbH
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

package terramate

import (
	"strings"

	"github.com/zclconf/go-cty/cty"
)

// StackGlobals holds all the globals defined for a stack.
type StackGlobals struct {
	data map[string]cty.Value
}

// LoadStackGlobals loads from the file system all globals defined for
// a given stack. It will navigate the file system from the stack dir until
// it reaches rootdir, loading globals and merging them appropriately.
//
// More specific globals (closer or at the stack) have precedence over
// less specific globals (closer or at the root dir).
//
// Metadata for the stack is used on the evaluation of globals, defined on stackmeta.
// The rootdir MUST be an absolute path.
func LoadStackGlobals(rootdir string, stackmeta StackMetadata) (*StackGlobals, error) {
	return NewStackGlobals(), nil
}

func NewStackGlobals() *StackGlobals {
	return &StackGlobals{
		data: map[string]cty.Value{},
	}
}

// Equal checks if two StackGlobals are equal. They are equal if both
// have globals with the same name=value.
func (sg *StackGlobals) Equal(other *StackGlobals) bool {
	if len(sg.data) != len(other.data) {
		return false
	}

	for k, v := range other.data {
		val, ok := sg.data[k]
		if !ok {
			return false
		}
		if !v.RawEquals(val) {
			return false
		}
	}

	return true
}

// AddString adds a new global of the string type
func (sg *StackGlobals) AddString(key, val string) {
	sg.data[key] = cty.StringVal(val)
}

// AddInt adds a new global of the int type
func (sg *StackGlobals) AddInt(key string, val int64) {
	sg.data[key] = cty.NumberIntVal(val)
}

// AddBool adds a new global of the bool type
func (sg *StackGlobals) AddBool(key string, val bool) {
	sg.data[key] = cty.BoolVal(val)
}

func (sg *StackGlobals) String() string {
	strrepr := make([]string, 0, len(sg.data))

	for k, v := range sg.data {
		line := k + "=" + v.GoString()
		strrepr = append(strrepr, line)
	}

	return strings.Join(strrepr, "\n")
}
