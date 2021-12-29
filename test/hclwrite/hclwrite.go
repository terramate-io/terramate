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

// Package hclwrite aims to provide some facilities making it easier/safer
// to generate HCL code for testing purposes. It aims at:
//
// - Close to how HCL is written.
// - Provide formatted string representation.
// - Avoid issues when raw HCL strings are used on tests in general.
//
// It is not a replacement to hclwrite: https://pkg.go.dev/github.com/hashicorp/hcl/v2/hclwrite
// It is just easier/nicer to use on tests + circumvents some limitations like:
//
// - https://stackoverflow.com/questions/67945463/how-to-use-hcl-write-to-set-expressions-with
package hclwrite

import (
	"fmt"

	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

type Block struct {
	name        string
	expressions map[string]string
	// Not cool to keep 2 copies of values but casting around
	// cty values is quite annoying, so this is a lazy solution.
	ctyvalues map[string]cty.Value
	values    map[string]interface{}
}

func (b *Block) AddExpr(key string, expr string) {
	b.expressions[key] = expr
}

func (b *Block) AddNumberInt(key string, v int64) {
	b.ctyvalues[key] = cty.NumberIntVal(v)
	b.values[key] = v
}

func (b *Block) AddString(key string, v string) {
	b.ctyvalues[key] = cty.StringVal(v)
	b.values[key] = fmt.Sprintf("%q", v)
}

func (b *Block) AddBoolean(key string, v bool) {
	b.ctyvalues[key] = cty.BoolVal(v)
	b.values[key] = v
}

func (b *Block) AttributesValues() map[string]cty.Value {
	return b.ctyvalues
}

func (b *Block) HasExpressions() bool {
	return len(b.expressions) > 0
}

func (b *Block) String() string {
	code := b.name + "{"
	// Tried properly using hclwrite, it doesnt work well with expressions:
	// - https://stackoverflow.com/questions/67945463/how-to-use-hcl-write-to-set-expressions-with
	for name, expr := range b.expressions {
		code += fmt.Sprintf("\n%s=%s\n", name, expr)
	}
	for name, val := range b.values {
		code += fmt.Sprintf("\n%s=%v\n", name, val)
	}
	code += "}"
	return string(hclwrite.Format([]byte(code)))
}

func NewBlock(name string) *Block {
	return &Block{
		name:        name,
		expressions: map[string]string{},
		ctyvalues:   map[string]cty.Value{},
		values:      map[string]interface{}{},
	}
}

type BlockBuilder func(*Block)

func NewBuilder(name string, builders ...BlockBuilder) *Block {
	b := NewBlock(name)
	for _, builder := range builders {
		builder(b)
	}
	return b
}

func Expression(key string, expr string) BlockBuilder {
	return func(g *Block) {
		g.AddExpr(key, expr)
	}
}

func String(key string, val string) BlockBuilder {
	return func(g *Block) {
		g.AddString(key, val)
	}
}

func Boolean(key string, val bool) BlockBuilder {
	return func(g *Block) {
		g.AddBoolean(key, val)
	}
}

func NumberInt(key string, val int64) BlockBuilder {
	return func(g *Block) {
		g.AddNumberInt(key, val)
	}
}
