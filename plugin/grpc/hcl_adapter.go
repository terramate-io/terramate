// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package grpc

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	hhcl "github.com/terramate-io/hcl/v2"
	"github.com/terramate-io/hcl/v2/hclsyntax"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/hcl/ast"
	pb "github.com/terramate-io/terramate/plugin/proto/v1"
	"github.com/zclconf/go-cty/cty"
)

// HCLExternalData stores opaque plugin data from gRPC HCL processing.
type HCLExternalData struct {
	Plugins map[string]map[string][][]byte
}

// HCLOptionsFromSchema builds HCL parser options for a plugin schema.
// The binaryPath is used to create clients on-demand when parsing blocks.
func HCLOptionsFromSchema(pluginName string, binaryPath string, schemas []*pb.HCLBlockSchema) []hcl.Option {
	if len(schemas) == 0 || binaryPath == "" {
		return nil
	}

	var unmerged []hcl.UnmergedBlockHandlerConstructor
	var merged []hcl.MergedBlockHandlerConstructor
	var mergedLabels []hcl.MergedLabelsBlockHandlerConstructor
	var unique []hcl.UniqueBlockHandlerConstructor

	for _, schema := range schemas {
		switch schema.Kind {
		case pb.BlockKind_BLOCK_UNMERGED:
			unmerged = append(unmerged, newUnmergedHandler(pluginName, binaryPath, schema))
		case pb.BlockKind_BLOCK_MERGED:
			merged = append(merged, newMergedHandler(pluginName, binaryPath, schema))
		case pb.BlockKind_BLOCK_MERGED_LABELS:
			mergedLabels = append(mergedLabels, newMergedLabelsHandler(pluginName, binaryPath, schema))
		case pb.BlockKind_BLOCK_UNIQUE:
			unique = append(unique, newUniqueHandler(pluginName, binaryPath, schema))
		}
	}

	var opts []hcl.Option
	if len(unmerged) > 0 {
		opts = append(opts, hcl.WithUnmergedBlockHandlers(unmerged...))
	}
	if len(merged) > 0 {
		opts = append(opts, hcl.WithMergedBlockHandlers(merged...))
	}
	if len(mergedLabels) > 0 {
		opts = append(opts, hcl.WithMergedLabelsBlockHandlers(mergedLabels...))
	}
	if len(unique) > 0 {
		opts = append(opts, hcl.WithUniqueBlockHandlers(unique...))
	}
	return opts
}

func newUnmergedHandler(pluginName string, binaryPath string, schema *pb.HCLBlockSchema) hcl.UnmergedBlockHandlerConstructor {
	return func() hcl.UnmergedBlockHandler {
		return &unmergedHandler{pluginName: pluginName, binaryPath: binaryPath, schema: schema}
	}
}

func newMergedHandler(pluginName string, binaryPath string, schema *pb.HCLBlockSchema) hcl.MergedBlockHandlerConstructor {
	return func() hcl.MergedBlockHandler {
		return &mergedHandler{pluginName: pluginName, binaryPath: binaryPath, schema: schema}
	}
}

func newMergedLabelsHandler(pluginName string, binaryPath string, schema *pb.HCLBlockSchema) hcl.MergedLabelsBlockHandlerConstructor {
	return func() hcl.MergedLabelsBlockHandler {
		return &mergedLabelsHandler{pluginName: pluginName, binaryPath: binaryPath, schema: schema}
	}
}

func newUniqueHandler(pluginName string, binaryPath string, schema *pb.HCLBlockSchema) hcl.UniqueBlockHandlerConstructor {
	return func() hcl.UniqueBlockHandler {
		return &uniqueHandler{pluginName: pluginName, binaryPath: binaryPath, schema: schema}
	}
}

type unmergedHandler struct {
	pluginName string
	binaryPath string
	schema     *pb.HCLBlockSchema
}

func (h *unmergedHandler) Name() string { return h.schema.Name }

func (h *unmergedHandler) Parse(p *hcl.TerramateParser, block *ast.Block) error {
	parsed := parsedBlockFromBlock(block)
	return h.processParsedBlocks(p, parsed)
}

func (h *unmergedHandler) processParsedBlocks(p *hcl.TerramateParser, parsed *pb.ParsedBlock) error {
	client, err := NewHostClient(h.binaryPath)
	if err != nil {
		return err
	}
	defer client.Kill()

	resp, err := client.Client().HCLSchemaService.ProcessParsedBlocks(context.Background(), &pb.ParsedBlocksRequest{
		BlockType: h.schema.Name,
		Blocks:    []*pb.ParsedBlock{parsed},
	})
	if err != nil {
		return err
	}
	if err := diagnosticsError(resp.Diagnostics); err != nil {
		return err
	}
	return storePluginData(p, h.pluginName, h.schema.Name, resp.PluginData)
}

type uniqueHandler struct {
	pluginName string
	binaryPath string
	schema     *pb.HCLBlockSchema
}

func (h *uniqueHandler) Name() string { return h.schema.Name }

func (h *uniqueHandler) Parse(p *hcl.TerramateParser, block *ast.Block) error {
	parsed := parsedBlockFromBlock(block)
	client, err := NewHostClient(h.binaryPath)
	if err != nil {
		return err
	}
	defer client.Kill()

	resp, err := client.Client().HCLSchemaService.ProcessParsedBlocks(context.Background(), &pb.ParsedBlocksRequest{
		BlockType: h.schema.Name,
		Blocks:    []*pb.ParsedBlock{parsed},
	})
	if err != nil {
		return err
	}
	if err := diagnosticsError(resp.Diagnostics); err != nil {
		return err
	}
	return storePluginData(p, h.pluginName, h.schema.Name, resp.PluginData)
}

type mergedHandler struct {
	pluginName string
	binaryPath string
	schema     *pb.HCLBlockSchema
}

func (h *mergedHandler) Name() string { return h.schema.Name }

func (h *mergedHandler) Parse(p *hcl.TerramateParser, block *ast.MergedBlock) error {
	parsed := parsedBlockFromMerged(block)
	client, err := NewHostClient(h.binaryPath)
	if err != nil {
		return err
	}
	defer client.Kill()

	resp, err := client.Client().HCLSchemaService.ProcessParsedBlocks(context.Background(), &pb.ParsedBlocksRequest{
		BlockType: h.schema.Name,
		Blocks:    []*pb.ParsedBlock{parsed},
	})
	if err != nil {
		return err
	}
	if err := diagnosticsError(resp.Diagnostics); err != nil {
		return err
	}
	return storePluginData(p, h.pluginName, h.schema.Name, resp.PluginData)
}

func (h *mergedHandler) Validate(*hcl.TerramateParser) error {
	return nil
}

type mergedLabelsHandler struct {
	pluginName string
	binaryPath string
	schema     *pb.HCLBlockSchema
}

func (h *mergedLabelsHandler) Name() string { return h.schema.Name }

func (h *mergedLabelsHandler) Parse(p *hcl.TerramateParser, labelType ast.LabelBlockType, block *ast.MergedBlock) error {
	parsed := parsedBlockFromMerged(block)
	parsed.Labels = labelsFromLabelType(labelType)
	client, err := NewHostClient(h.binaryPath)
	if err != nil {
		return err
	}
	defer client.Kill()

	resp, err := client.Client().HCLSchemaService.ProcessParsedBlocks(context.Background(), &pb.ParsedBlocksRequest{
		BlockType: h.schema.Name,
		Blocks:    []*pb.ParsedBlock{parsed},
	})
	if err != nil {
		return err
	}
	if err := diagnosticsError(resp.Diagnostics); err != nil {
		return err
	}
	return storePluginData(p, h.pluginName, h.schema.Name, resp.PluginData)
}

func (h *mergedLabelsHandler) Validate(*hcl.TerramateParser) error {
	return nil
}

func parsedBlockFromBlock(block *ast.Block) *pb.ParsedBlock {
	parsed := &pb.ParsedBlock{
		FilePath:   block.Range.HostPath(),
		Labels:     append([]string{}, block.Labels...),
		Attributes: map[string]*pb.AttributeValue{},
		BlockType:  string(block.Type),
	}

	for name, attr := range block.Attributes {
		parsed.Attributes[name] = attributeValue(attr.Attribute)
	}

	for _, child := range block.Blocks {
		parsed.NestedBlocks = append(parsed.NestedBlocks, parsedBlockFromBlock(child))
	}

	return parsed
}

func parsedBlockFromMerged(block *ast.MergedBlock) *pb.ParsedBlock {
	parsed := &pb.ParsedBlock{
		BlockType:  string(block.Type),
		Labels:     append([]string{}, block.Labels...),
		Attributes: map[string]*pb.AttributeValue{},
	}
	if len(block.RawOrigins) > 0 {
		parsed.FilePath = block.RawOrigins[0].Range.HostPath()
	}

	for name, attr := range block.Attributes {
		parsed.Attributes[name] = attributeValue(attr.Attribute)
	}

	for labelType, child := range block.Blocks {
		childParsed := parsedBlockFromMerged(child)
		childParsed.BlockType = string(labelType.Type)
		if len(childParsed.Labels) == 0 {
			childParsed.Labels = labelsFromLabelType(labelType)
		}
		parsed.NestedBlocks = append(parsed.NestedBlocks, childParsed)
	}

	return parsed
}

func attributeValue(attr *hhcl.Attribute) *pb.AttributeValue {
	if attr == nil || attr.Expr == nil {
		return &pb.AttributeValue{Value: &pb.AttributeValue_ExpressionText{ExpressionText: ""}}
	}

	if lit, ok := attr.Expr.(*hclsyntax.LiteralValueExpr); ok {
		val := lit.Val
		if val.Type() == cty.String {
			return &pb.AttributeValue{Value: &pb.AttributeValue_StringValue{StringValue: val.AsString()}}
		}
		if val.Type() == cty.Bool {
			return &pb.AttributeValue{Value: &pb.AttributeValue_BoolValue{BoolValue: val.True()}}
		}
		if val.Type() == cty.Number {
			if i, acc := val.AsBigFloat().Int64(); acc == big.Exact {
				return &pb.AttributeValue{Value: &pb.AttributeValue_IntValue{IntValue: i}}
			}
			f, _ := val.AsBigFloat().Float64()
			return &pb.AttributeValue{Value: &pb.AttributeValue_FloatValue{FloatValue: f}}
		}
	}

	if val, diags := attr.Expr.Value(nil); !diags.HasErrors() && val.IsKnown() {
		if val.Type() == cty.String {
			return &pb.AttributeValue{Value: &pb.AttributeValue_StringValue{StringValue: val.AsString()}}
		}
		if val.Type() == cty.Bool {
			return &pb.AttributeValue{Value: &pb.AttributeValue_BoolValue{BoolValue: val.True()}}
		}
		if val.Type() == cty.Number {
			if i, acc := val.AsBigFloat().Int64(); acc == big.Exact {
				return &pb.AttributeValue{Value: &pb.AttributeValue_IntValue{IntValue: i}}
			}
			f, _ := val.AsBigFloat().Float64()
			return &pb.AttributeValue{Value: &pb.AttributeValue_FloatValue{FloatValue: f}}
		}
	}

	expr := string(ast.TokensForExpression(attr.Expr).Bytes())
	// Only trim leading whitespace. Trailing newlines must be preserved for
	// heredoc expressions (<<-EOT\n...\nEOT\n) because HCL's ParseExpression
	// requires the closing marker to be followed by a newline.
	expr = strings.TrimLeft(expr, " \t")
	return &pb.AttributeValue{Value: &pb.AttributeValue_ExpressionText{ExpressionText: expr}}
}

func labelsFromLabelType(labelType ast.LabelBlockType) []string {
	if labelType.NumLabels == 0 {
		var labels []string
		for _, label := range labelType.Labels {
			if label == "" {
				break
			}
			labels = append(labels, label)
		}
		if len(labels) == 0 {
			return nil
		}
		return labels
	}
	labels := make([]string, 0, labelType.NumLabels)
	for i := 0; i < labelType.NumLabels; i++ {
		labels = append(labels, labelType.Labels[i])
	}
	return labels
}

func storePluginData(p *hcl.TerramateParser, pluginName, blockType string, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	if p == nil {
		return nil
	}
	if p.ParsedConfig.External == nil {
		p.ParsedConfig.External = &HCLExternalData{Plugins: map[string]map[string][][]byte{}}
	}
	ext, ok := p.ParsedConfig.External.(*HCLExternalData)
	if !ok {
		return errors.E("plugin external data already set to incompatible type")
	}
	if ext.Plugins == nil {
		ext.Plugins = map[string]map[string][][]byte{}
	}
	if ext.Plugins[pluginName] == nil {
		ext.Plugins[pluginName] = map[string][][]byte{}
	}
	ext.Plugins[pluginName][blockType] = append(ext.Plugins[pluginName][blockType], data)
	return nil
}

func diagnosticsError(diags []*pb.Diagnostic) error {
	var messages []string
	for _, diag := range diags {
		if diag.Severity != pb.Diagnostic_ERROR {
			continue
		}
		msg := diag.Summary
		if diag.Detail != "" {
			msg = msg + ": " + diag.Detail
		}
		if diag.File != "" {
			msg = msg + fmt.Sprintf(" (%s:%d:%d)", diag.File, diag.Line, diag.Column)
		}
		messages = append(messages, msg)
	}
	if len(messages) == 0 {
		return nil
	}
	return errors.E(strings.Join(messages, "; "))
}

// DiagnosticsError exposes diagnosticsError for callers outside this package.
func DiagnosticsError(diags []*pb.Diagnostic) error {
	return diagnosticsError(diags)
}
