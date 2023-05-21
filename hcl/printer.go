// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package hcl

import (
	"fmt"
	"io"

	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/rs/zerolog/log"
	"github.com/zclconf/go-cty/cty"
)

// PrintImports will print the given imports list as import blocks.
func PrintImports(w io.Writer, imports []string) error {
	logger := log.With().
		Str("action", "PrintImports()").
		Str("imports", fmt.Sprint(imports)).
		Logger()

	logger.Trace().Msg("Create empty hcl file")

	f := hclwrite.NewEmptyFile()
	rootBody := f.Body()

	for _, source := range imports {
		block := rootBody.AppendNewBlock("import", nil)
		body := block.Body()
		body.SetAttributeValue("source", cty.StringVal(source))
		rootBody.AppendNewline()
	}

	_, err := w.Write(f.Bytes())
	return err
}

// PrintConfig will print the given config as HCL on the given writer.
func PrintConfig(w io.Writer, cfg Config) error {
	logger := log.With().
		Str("action", "PrintConfig()").
		Str("stack", cfg.Stack.Name).
		Logger()

	logger.Trace().Msg("Create empty hcl file")

	f := hclwrite.NewEmptyFile()
	rootBody := f.Body()

	if cfg.Terramate != nil {
		logger.Trace().Msg("Append terramate block")

		tm := cfg.Terramate
		tmBlock := rootBody.AppendNewBlock("terramate", nil)
		tmBody := tmBlock.Body()
		tmBody.SetAttributeValue("required_version", cty.StringVal(tm.RequiredVersion))
	}

	if cfg.Terramate != nil && cfg.Stack != nil {
		rootBody.AppendNewline()
	}

	if cfg.Stack != nil {
		logger.Trace().Msg("Append 'stack' block")

		stack := cfg.Stack
		stackBlock := rootBody.AppendNewBlock("stack", nil)
		stackBody := stackBlock.Body()

		if stack.Name != "" {
			stackBody.SetAttributeValue("name", cty.StringVal(stack.Name))
		}

		if stack.Description != "" {
			stackBody.SetAttributeValue("description", cty.StringVal(stack.Description))
		}

		if len(stack.Tags) > 0 {
			stackBody.SetAttributeValue("tags", cty.SetVal(listToValue(stack.Tags)))
		}

		if len(stack.After) > 0 {
			stackBody.SetAttributeValue("after", cty.SetVal(listToValue(stack.After)))
		}

		if len(stack.Before) > 0 {
			stackBody.SetAttributeValue("before", cty.SetVal(listToValue(stack.Before)))
		}

		if len(stack.Wants) > 0 {
			stackBody.SetAttributeValue("wants", cty.SetVal(listToValue(stack.Wants)))
		}

		if len(stack.WantedBy) > 0 {
			stackBody.SetAttributeValue("wanted_by", cty.SetVal(listToValue(stack.WantedBy)))
		}

		if len(stack.Watch) > 0 {
			stackBody.SetAttributeValue("watch", cty.SetVal(listToValue(stack.Watch)))
		}

		if stack.ID != "" {
			stackBody.SetAttributeValue("id", cty.StringVal(stack.ID))
		}
	}

	logger.Debug().Msg("write to output")
	_, err := w.Write(f.Bytes())
	return err
}

func listToValue(list []string) []cty.Value {
	vlist := make([]cty.Value, len(list))
	for i, val := range list {
		vlist[i] = cty.StringVal(val)
	}

	return vlist
}
