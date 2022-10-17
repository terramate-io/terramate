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

// Package genhcl implements generate_hcl code generation.
package genhcl

import (
	"fmt"
	"sort"

	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/hcl/ast"
	mcty "github.com/mineiros-io/terramate/hcl/cty"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/lets"
	"github.com/mineiros-io/terramate/project"
	"github.com/mineiros-io/terramate/stack"
	"github.com/rs/zerolog/log"
	"github.com/zclconf/go-cty/cty"
)

// HCL represents generated HCL code from a single block.
// Is contains parsed and evaluated code on it and information
// about the origin of the generated code.
type HCL struct {
	label     string
	origin    project.Path
	body      string
	condition bool
}

const (
	// Header is the current header string used by generate_hcl code generation.
	Header = "// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT"

	// HeaderV0 is the deprecated header string used by generate_hcl code generation.
	HeaderV0 = "// GENERATED BY TERRAMATE: DO NOT EDIT"
)

const (
	// ErrParsing indicates the failure of parsing the generate_hcl block.
	ErrParsing errors.Kind = "parsing generate_hcl block"

	// ErrContentEval indicates the failure to evaluate the content block.
	ErrContentEval errors.Kind = "evaluating content block"

	// ErrConditionEval indicates the failure to evaluate the condition attribute.
	ErrConditionEval errors.Kind = "evaluating condition attribute"

	// ErrInvalidConditionType indicates the condition attribute
	// has an invalid type.
	ErrInvalidConditionType errors.Kind = "invalid condition type"

	// ErrInvalidDynamicIterator indicates that the iterator of a tm_dynamic block
	// is invalid.
	ErrInvalidDynamicIterator errors.Kind = "invalid tm_dynamic.iterator"

	// ErrInvalidDynamicLabels indicates that the labels of a tm_dynamic block is invalid.
	ErrInvalidDynamicLabels errors.Kind = "invalid tm_dynamic.labels"

	// ErrDynamicAttrsEval indicates that the attributes of a tm_dynamic cant be evaluated.
	ErrDynamicAttrsEval errors.Kind = "evaluating tm_dynamic.attributes"

	// ErrDynamicConditionEval indicates that the condition of a tm_dynamic cant be evaluated.
	ErrDynamicConditionEval errors.Kind = "evaluating tm_dynamic.condition"
)

// Label of the original generate_hcl block.
func (h HCL) Label() string {
	return h.label
}

// Header returns the header of the generated HCL file.
func (h HCL) Header() string {
	return fmt.Sprintf(
		"%s\n// TERRAMATE: originated from generate_hcl block on %s\n\n",
		Header,
		h.origin,
	)
}

// Body returns a string representation of the HCL code
// or an empty string if the config itself is empty.
func (h HCL) Body() string {
	return string(h.body)
}

// Origin returns the path, relative to the project root,
// of the configuration that originated the code.
func (h HCL) Origin() project.Path {
	return h.origin
}

// Condition returns the evaluated condition attribute for the generated code.
func (h HCL) Condition() bool {
	return h.condition
}

func (h HCL) String() string {
	return fmt.Sprintf("Generating file %q (condition %t) (body %q) (origin %q)",
		h.Label(), h.Condition(), h.Body(), h.Origin())
}

// Load loads from the file system all generate_hcl for
// a given stack. It will navigate the file system from the stack dir until
// it reaches rootdir, loading generate_hcl and merging them appropriately.
//
// All generate_file blocks must have unique labels, even ones at different
// directories. Any conflicts will be reported as an error.
//
// Metadata and globals for the stack are used on the evaluation of the
// generate_hcl blocks.
//
// The rootdir MUST be an absolute path.
func Load(
	tree *config.Tree,
	projmeta project.Metadata,
	sm stack.Metadata,
	globals *mcty.Object,
) ([]HCL, error) {
	logger := log.With().
		Str("action", "genhcl.Load()").
		Str("path", sm.HostPath()).
		Logger()

	logger.Trace().Msg("loading generate_hcl blocks.")

	loadedHCLs, err := loadGenHCLBlocks(tree, sm.Path())
	if err != nil {
		return nil, errors.E("loading generate_hcl", err)
	}

	logger.Trace().Msg("generating HCL code.")

	var hcls []HCL
	for _, loadedHCL := range loadedHCLs {
		name := loadedHCL.name

		logger := logger.With().
			Str("block", name).
			Logger()

		evalctx := stack.NewEvalCtx(projmeta, sm, globals)
		err := lets.Load(loadedHCL.lets, evalctx.Context)
		if err != nil {
			return nil, err
		}

		condition := true
		if loadedHCL.condition != nil {
			logger.Trace().Msg("has condition attribute, evaluating it")
			value, err := evalctx.Eval(loadedHCL.condition.Expr)
			if err != nil {
				return nil, errors.E(ErrConditionEval, err)
			}
			if value.Type() != cty.Bool {
				return nil, errors.E(
					ErrInvalidConditionType,
					"condition has type %s but must be boolean",
					value.Type().FriendlyName(),
				)
			}
			condition = value.True()
		}

		if !condition {
			logger.Trace().Msg("condition=false, block wont be evaluated")

			hcls = append(hcls, HCL{
				label:     name,
				origin:    loadedHCL.origin,
				condition: condition,
			})

			continue
		}

		logger.Trace().Msg("evaluating block")

		gen := hclwrite.NewEmptyFile()
		if err := copyBody(gen.Body(), loadedHCL.block.Body, evalctx); err != nil {
			return nil, errors.E(ErrContentEval, sm, err,
				"generate_hcl %q", name,
			)
		}

		// TODO(i4k): filename in line below must be an absolute host path.
		formatted, err := hcl.FormatMultiline(string(gen.Bytes()), loadedHCL.origin.String())
		if err != nil {
			// genhcl must always generate valid code that is formatable
			// this is a severe internal error
			logger.Error().
				Err(err).
				Stringer("origin", loadedHCL.origin).
				Str("code", string(gen.Bytes())).
				Str("label", name).
				Msg("internal error formatting generated code")

			panic(errors.E(sm, err,
				"internal error: formatting generated code for generate_hcl %q", name,
			))
		}
		hcls = append(hcls, HCL{
			label:     name,
			origin:    loadedHCL.origin,
			body:      formatted,
			condition: condition,
		})
	}

	sort.Slice(hcls, func(i, j int) bool {
		return hcls[i].String() < hcls[j].String()
	})

	logger.Trace().Msg("evaluated all blocks with success")
	return hcls, nil
}

type loadedHCL struct {
	name      string
	origin    project.Path
	lets      hclsyntax.Blocks
	block     *hclsyntax.Block
	condition *hclsyntax.Attribute
}

type dynBlockAttributes struct {
	attributes *hclsyntax.Attribute
	iterator   *hclsyntax.Attribute
	foreach    *hclsyntax.Attribute
	labels     *hclsyntax.Attribute
	condition  *hclsyntax.Attribute
}

// loadGenHCLBlocks will load all generate_hcl blocks.
// The returned map maps the name of the block (its label)
// to the original block and the path (relative to project root) of the config
// from where it was parsed.
func loadGenHCLBlocks(tree *config.Tree, cfgdir project.Path) ([]loadedHCL, error) {
	logger := log.With().
		Str("action", "genhcl.loadGenHCLBlocks()").
		Str("root", tree.RootDir()).
		Stringer("configDir", cfgdir).
		Logger()

	logger.Trace().Msg("Parsing generate_hcl blocks.")

	res := []loadedHCL{}
	cfg, ok := tree.Lookup(cfgdir)
	if ok && !cfg.IsEmptyConfig() {
		blocks := cfg.Node.Generate.HCLs

		logger.Trace().Msg("Parsed generate_hcl blocks.")

		for _, genhclBlock := range blocks {
			name := genhclBlock.Label
			origin := project.PrjAbsPath(tree.RootDir(), genhclBlock.Origin)

			res = append(res, loadedHCL{
				name:      name,
				origin:    origin,
				lets:      genhclBlock.Lets,
				block:     genhclBlock.Content,
				condition: genhclBlock.Condition,
			})

			logger.Trace().Msg("loaded generate_hcl block.")
		}
	}

	parentCfgDir := cfgdir.Dir()
	if parentCfgDir == cfgdir {
		return res, nil
	}

	parentRes, err := loadGenHCLBlocks(tree, parentCfgDir)
	if err != nil {
		return nil, err
	}

	res = append(res, parentRes...)

	logger.Trace().Msg("loaded generate_hcl blocks with success.")
	return res, nil
}

// copyBody will copy the src body to the given target, evaluating attributes
// using the given evaluation context.
//
// Scoped traversals, like name.traverse, for unknown namespaces will be copied
// as is (original expression form, no evaluation).
//
// Returns an error if the evaluation fails.
func copyBody(dest *hclwrite.Body, src *hclsyntax.Body, eval hcl.Evaluator) error {
	logger := log.With().
		Str("action", "genhcl.copyBody()").
		Logger()

	logger.Trace().Msg("sorting attributes")

	attrs := ast.SortRawAttributes(ast.AsHCLAttributes(src.Attributes))
	for _, attr := range attrs {
		logger := logger.With().
			Str("attrName", attr.Name).
			Logger()

		logger.Trace().Msg("evaluating.")
		tokens, err := eval.PartialEval(attr.Expr)
		if err != nil {
			return errors.E(err, attr.Expr.Range())
		}

		logger.Trace().Str("attribute", attr.Name).Msg("Setting evaluated attribute.")
		dest.SetAttributeRaw(attr.Name, tokens)
	}

	logger.Trace().Msg("appending blocks")

	for _, block := range src.Blocks {
		err := appendBlock(dest, block, eval)
		if err != nil {
			return err
		}
	}

	return nil
}

func appendBlock(target *hclwrite.Body, block *hclsyntax.Block, eval hcl.Evaluator) error {
	if block.Type == "tm_dynamic" {
		return appendDynamicBlocks(target, block, eval)
	}

	targetBlock := target.AppendNewBlock(block.Type, block.Labels)
	if block.Body != nil {
		err := copyBody(targetBlock.Body(), block.Body, eval)
		if err != nil {
			return err
		}
	}
	return nil
}

func appendDynamicBlock(
	destination *hclwrite.Body,
	evaluator hcl.Evaluator,
	genBlockType string,
	attrs dynBlockAttributes,
	contentBlock *hclsyntax.Block,
) error {
	var labels []string
	if attrs.labels != nil {
		labelsVal, err := evaluator.Eval(attrs.labels.Expr)
		if err != nil {
			return errors.E(ErrInvalidDynamicLabels,
				err, attrs.labels.Range(),
				"failed to evaluate tm_dynamic.labels")
		}

		labels, err = hcl.ValueAsStringList(labelsVal)
		if err != nil {
			return errors.E(ErrInvalidDynamicLabels,
				err, attrs.labels.Range(),
				"tm_dynamic.labels is not a string list")
		}
	}

	logger := log.With().
		Str("action", "genhcl.appendDynamicBlock").
		Str("type", genBlockType).
		Strs("labels", labels).
		Logger()

	newblock := destination.AppendBlock(hclwrite.NewBlock(genBlockType, labels))

	if contentBlock != nil {
		logger.Trace().Msg("using content block to define new block body")

		err := copyBody(newblock.Body(), contentBlock.Body, evaluator)
		if err != nil {
			return err
		}

		return nil
	}

	logger.Trace().Msg("using attributes to define new block body")

	attrsTokens, err := evaluator.PartialEval(attrs.attributes.Expr)
	if err != nil {
		return errors.E(ErrDynamicAttrsEval, err, attrs.attributes.Range())
	}

	// Sadly hclsyntax doesn't export an easy way to build an expression from tokens,
	// even though it would be easy to do so:
	//
	// - https://github.com/hashicorp/hcl2/blob/fb75b3253c80b3bc7ca99c4bfa2ad6743841b1af/hcl/hclsyntax/public.go#L41
	//
	// So here we need to convert the parsed attributes to []byte so it can be
	// converted again to tokens :-).
	attrsExpr, diags := hclsyntax.ParseExpression(attrsTokens.Bytes(), "", hhcl.InitialPos)
	if diags.HasErrors() {
		// Panic here since Terramate generated an invalid expression after
		// partial evaluation and it is a guaranteed invariant that partial
		// evaluation only produces valid expressions.
		log.Error().
			Err(diags).
			Str("partiallyEvaluated", string(attrsTokens.Bytes())).
			Msg("partially evaluated `attributes` should be a valid expression")
		panic(wrapAttrErr(err, attrs.attributes,
			"internal error: partially evaluated `attributes` produced invalid expression: %v", diags))
	}

	objectExpr, ok := attrsExpr.(*hclsyntax.ObjectConsExpr)
	if !ok {
		return attrErr(attrs.attributes,
			"tm_dynamic attributes must be an object, got %T instead", objectExpr)
	}

	newbody := newblock.Body()

	for _, item := range objectExpr.Items {
		keyVal, err := evaluator.Eval(item.KeyExpr)
		if err != nil {
			return errors.E(ErrDynamicAttrsEval, err,
				attrs.attributes.Range(),
				"evaluating tm_dynamic.attributes key")
		}
		if keyVal.Type() != cty.String {
			return attrErr(attrs.attributes,
				"tm_dynamic.attributes key %q has type %q, must be a string",
				keyVal.GoString(),
				keyVal.Type().FriendlyName())
		}

		// hclwrite lib will accept any arbitrary string as attr name
		// allowing it to generate invalid HCL code, so here we check if
		// the attribute is a proper HCL identifier.
		attrName := keyVal.AsString()
		if !hclsyntax.ValidIdentifier(attrName) {
			return attrErr(attrs.attributes,
				"tm_dynamic.attributes key %q is not a valid HCL identifier",
				attrName)
		}

		exprRange := item.ValueExpr.Range()
		exprBytes := attrsTokens.Bytes()[exprRange.Start.Byte:exprRange.End.Byte]
		valExpr, err := eval.TokensForExpressionBytes(exprBytes)
		if err != nil {
			// Panic here since Terramate generated an invalid expression after
			// partial evaluation and it is a guaranteed invariant that partial
			// evaluation only produces valid expressions.
			log.Error().
				Err(err).
				Str("attribute", attrName).
				Str("partiallyEvaluated", string(attrsTokens.Bytes())).
				Msg("partially evaluated `attributes` has invalid value expression inside object")
			panic(wrapAttrErr(err, attrs.attributes,
				"internal error: partially evaluated `attributes` has invalid value expressions inside object: %v", err))
		}

		logger.Trace().
			Str("attribute", attrName).
			Str("value", string(valExpr.Bytes())).
			Msg("adding attribute on generated block")

		newbody.SetAttributeRaw(attrName, valExpr)
	}

	return nil
}

func appendDynamicBlocks(target *hclwrite.Body, dynblock *hclsyntax.Block, evaluator hcl.Evaluator) error {
	logger := log.With().
		Str("action", "genhcl.appendDynamicBlock").
		Logger()

	logger.Trace().Msg("appending tm_dynamic block")

	errs := errors.L()

	if len(dynblock.Labels) != 1 {
		errs.Append(errors.E(ErrParsing,
			dynblock.LabelRanges, "tm_dynamic requires a single label"))
	}

	attrs, err := getDynamicBlockAttrs(dynblock)
	errs.Append(err)

	contentBlock, err := getContentBlock(dynblock.Body.Blocks)
	errs.Append(err)

	if contentBlock == nil && attrs.attributes == nil {
		errs.Append(errors.E(ErrParsing, dynblock.Body.Range(),
			"`content` block or `attributes` obj must be defined"))
	}

	if contentBlock != nil && attrs.attributes != nil {
		errs.Append(errors.E(ErrParsing, dynblock.Body.Range(),
			"`content` block and `attributes` obj are not allowed together"))
	}

	if err := errs.AsError(); err != nil {
		return err
	}

	genBlockType := dynblock.Labels[0]

	logger = logger.With().
		Str("genBlockType", genBlockType).
		Logger()

	if attrs.condition != nil {
		condition, err := evaluator.Eval(attrs.condition.Expr)
		if err != nil {
			return errors.E(ErrDynamicConditionEval, err)
		}
		if condition.Type() != cty.Bool {
			return errors.E(ErrDynamicConditionEval, "want boolean got %s", condition.Type().FriendlyName())
		}
		if !condition.True() {
			logger.Trace().Msg("condition is false, ignoring block")
			return nil
		}
	}

	var foreach cty.Value

	if attrs.foreach != nil {
		logger.Trace().Msg("evaluating for_each attribute")

		foreach, err = evaluator.Eval(attrs.foreach.Expr)
		if err != nil {
			return wrapAttrErr(err, attrs.foreach, "evaluating `for_each` expression")
		}

		if !foreach.CanIterateElements() {
			return attrErr(attrs.foreach,
				"`for_each` expression of type %s cannot be iterated",
				foreach.Type().FriendlyName())
		}
	}

	if foreach.IsNull() {
		logger.Trace().Msg("no for_each, generating single block")

		if attrs.iterator != nil {
			return errors.E(ErrInvalidDynamicIterator,
				attrs.iterator.Range(),
				"iterator should not be defined when for_each is omitted")
		}

		return appendDynamicBlock(target, evaluator,
			genBlockType, attrs, contentBlock)
	}

	logger.Trace().Msg("defining iterator name")

	iterator := genBlockType

	if attrs.iterator != nil {
		iteratorTraversal, diags := hhcl.AbsTraversalForExpr(attrs.iterator.Expr)
		if diags.HasErrors() {
			return errors.E(ErrInvalidDynamicIterator,
				attrs.iterator.Range(),
				"dynamic iterator must be a single variable name")
		}
		if len(iteratorTraversal) != 1 {
			return errors.E(ErrInvalidDynamicIterator,
				attrs.iterator.Range(),
				"dynamic iterator must be a single variable name")
		}
		iterator = iteratorTraversal.RootName()
	}

	logger = logger.With().
		Str("iterator", iterator).
		Logger()

	logger.Trace().Msg("generating blocks")

	var tmDynamicErr error

	foreach.ForEachElement(func(key, value cty.Value) (stop bool) {
		evaluator.SetNamespace(iterator, map[string]cty.Value{
			"key":   key,
			"value": value,
		})

		if err := appendDynamicBlock(target, evaluator,
			genBlockType, attrs, contentBlock); err != nil {
			tmDynamicErr = err
			return true
		}

		return false
	})

	evaluator.DeleteNamespace(iterator)
	return tmDynamicErr
}

func getDynamicBlockAttrs(block *hclsyntax.Block) (dynBlockAttributes, error) {
	dynAttrs := dynBlockAttributes{}
	errs := errors.L()

	for name, attr := range block.Body.Attributes {
		switch name {
		case "attributes":
			dynAttrs.attributes = attr
		case "for_each":
			dynAttrs.foreach = attr
		case "labels":
			dynAttrs.labels = attr
		case "iterator":
			dynAttrs.iterator = attr
		case "condition":
			dynAttrs.condition = attr
		default:
			errs.Append(attrErr(
				attr, "tm_dynamic unsupported attribute %q", name))
		}
	}

	// Unusual but we return the value so further errors can still be added
	// based on properties of the attributes that are valid.
	return dynAttrs, errs.AsError()
}

func getContentBlock(blocks hclsyntax.Blocks) (*hclsyntax.Block, error) {
	var contentBlock *hclsyntax.Block

	errs := errors.L()

	for _, b := range blocks {
		if b.Type != "content" {
			errs.Append(errors.E(ErrParsing,
				b.TypeRange, "unrecognized block %s", b.Type))

			continue
		}

		if contentBlock != nil {
			errs.Append(errors.E(ErrParsing, b.TypeRange,
				"multiple definitions of the `content` block"))

			continue
		}

		contentBlock = b
	}

	if err := errs.AsError(); err != nil {
		return nil, err
	}

	return contentBlock, nil
}

func attrErr(attr *hclsyntax.Attribute, msg string, args ...interface{}) error {
	return errors.E(ErrParsing, attr.Expr.Range(), fmt.Sprintf(msg, args...))
}

func wrapAttrErr(err error, attr *hclsyntax.Attribute, msg string, args ...interface{}) error {
	return errors.E(ErrParsing, err, attr.Expr.Range(), fmt.Sprintf(msg, args...))
}
