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
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/hcl"
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
	return loadStackGlobals(rootdir, stackmeta.Path)
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

// Add adds a new global.
func (sg *StackGlobals) Add(key string, val cty.Value) {
	sg.data[key] = val
}

// String representation of the stack globals as HCL.
func (sg *StackGlobals) String() string {
	gen := hclwrite.NewEmptyFile()
	rootBody := gen.Body()
	tfBlock := rootBody.AppendNewBlock("globals", nil)
	tfBody := tfBlock.Body()

	for name, val := range sg.data {
		tfBody.SetAttributeValue(name, val)
	}

	return string(gen.Bytes())
}

func (sg *StackGlobals) merge(other *StackGlobals) {
	for k, v := range other.data {
		_, ok := sg.data[k]
		if ok {
			continue
		}
		sg.data[k] = v
	}
}

func loadStackGlobals(rootdir string, cfgdir string) (*StackGlobals, error) {
	cfgpath := filepath.Join(rootdir, cfgdir, config.Filename)
	blocks, err := hcl.ParseGlobalsBlocks(cfgpath)

	if os.IsNotExist(err) {
		parentcfg, ok := parentDir(cfgdir)
		if !ok {
			return NewStackGlobals(), nil
		}
		return loadStackGlobals(rootdir, parentcfg)

	}

	if err != nil {
		return nil, err
	}

	globals := NewStackGlobals()

	for _, block := range blocks {
		for name, attr := range block.Body.Attributes {
			val, err := attr.Expr.Value(nil)
			if err != nil {
				return nil, fmt.Errorf("evaluating attribute %q: %v", attr.Name, err)
			}
			globals.Add(name, val)
		}
	}

	parentcfg, ok := parentDir(cfgdir)
	if !ok {
		return globals, nil
	}

	parentGlobals, err := loadStackGlobals(rootdir, parentcfg)

	if err != nil {
		return nil, err
	}

	globals.merge(parentGlobals)
	return globals, nil
}

func parentDir(dir string) (string, bool) {
	parent := filepath.Dir(dir)
	return parent, parent != dir
}
