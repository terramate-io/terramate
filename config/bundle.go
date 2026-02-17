// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package config

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/zclconf/go-cty/cty"

	hhcl "github.com/terramate-io/hcl/v2"
	"github.com/terramate-io/terramate/config/tag"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/generate/resolve"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/hcl/info"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/typeschema"
)

// BundleDefinition represents the definition of a bundle.
type BundleDefinition struct {
	Class string

	ScaffoldingName string
	ScaffoldingPath string
}

// Bundle represents the instantiation of a bundle.
type Bundle struct {
	Class          string
	Alias          string
	Name           string
	UUID           string
	Workdir        project.Path
	Source         string
	ResolvedSource project.Path
	Inst           *hcl.Bundle
	Environment    *Environment
	Stacks         map[project.Path]StackMetadata
	Generate       hcl.GenerateConfig
	Inputs         map[string]cty.Value
	Exports        map[string]cty.Value
	Info           info.Range
}

// Metadata is the evaluated metadata.
type Metadata struct {
	Dir project.Path

	Class       string
	Name        string
	Version     string
	Description string
}

// StackMetadata holds metadata for a stack within a bundle.
type StackMetadata struct {
	Dir project.Path

	Name        string
	Description string
	Tags        []string
	After       []string
	Before      []string
	Wants       []string
	WantedBy    []string
	Watch       []string

	Skipped    bool
	Components []*hcl.Component
}

// NamedValue is a name/value pair.
type NamedValue struct {
	Name  string
	Value cty.Value
}

// InputDefinition is the evaluated input definition.
type InputDefinition struct {
	Name          string
	Prompt        string
	PromptDefault cty.Value
	Description   string
	Type          typeschema.Type
	Options       []NamedValue
	Multiline     bool
	Multiselect   bool
}

// Validate if all stack fields are correct.
func (s StackMetadata) Validate() error {
	errs := errors.L()
	errs.AppendWrap(ErrStackValidation, s.ValidateSets(), s.ValidateTags())
	return errs.AsError()
}

// ValidateSets validate all stack set fields.
func (s StackMetadata) ValidateSets() error {
	errs := errors.L(
		validateSet("tags", s.Tags),
		validateSet("after", s.After),
		validateSet("before", s.Before),
		validateSet("wants", s.Wants),
		validateSet("wanted_by", s.WantedBy),
	)
	return errs.AsError()
}

// ValidateTags validates if tags are correctly used in all stack fields.
func (s StackMetadata) ValidateTags() error {
	for _, tagname := range s.Tags {
		err := tag.Validate(tagname)
		if err != nil {
			return errors.E(ErrStackInvalidTag, err)
		}
	}
	return nil
}

// FlattenBundleTemplate will flatten a bundle template into a list of concrete bundles.
// If the template contained environments, one bundle per environment is returned (base + env value overrides)
// If not, a single bundle is returned (base values only).
func FlattenBundleTemplate(bundleTpl *hcl.BundleTemplate) []*hcl.Bundle {
	if len(bundleTpl.EnvValues) == 0 {
		bundle := &hcl.Bundle{
			Name:       bundleTpl.Name,
			UUID:       bundleTpl.UUID,
			Source:     bundleTpl.Source,
			Workdir:    bundleTpl.Workdir,
			InputsAttr: bundleTpl.InputsAttr,
			Inputs:     bundleTpl.Inputs,
			Info:       bundleTpl.Info,
		}
		return []*hcl.Bundle{bundle}
	}

	bundles := make([]*hcl.Bundle, 0, len(bundleTpl.EnvValues))
	for _, envVals := range bundleTpl.EnvValues {
		// Environment source takes priority over bundle template source.
		source := bundleTpl.Source
		if envVals.Source != nil {
			source = envVals.Source
		}

		bundle := &hcl.Bundle{
			Name:        bundleTpl.Name,
			Environment: envVals.EnvID,
			UUID:        bundleTpl.UUID,
			Source:      source,
			Workdir:     bundleTpl.Workdir,
			Info:        bundleTpl.Info,
		}

		bundle.InputsAttr = bundleTpl.InputsAttr
		bundle.Inputs = envVals.Inputs

		bundles = append(bundles, bundle)
	}

	return bundles
}

// FlattenBundleTemplates is a helper that to [FlattenBundleTemplate] for a list of bundle templates.
func FlattenBundleTemplates(bundleTpls []*hcl.BundleTemplate) []*hcl.Bundle {
	r := []*hcl.Bundle{}
	for _, tpl := range bundleTpls {
		r = append(r, FlattenBundleTemplate(tpl)...)
	}
	return r
}

// EvalBundleSchemaNamespaces evaluates the uses_schemas blocks of a bundle definition.
func EvalBundleSchemaNamespaces(root *Root, resolveAPI resolve.API, evalctx *eval.Context, defHCL *hcl.DefineBundle, allowFetch bool) (typeschema.SchemaNamespaces, error) {
	schemas := typeschema.NewSchemaNamespaces()

	for _, usesSchemasHCL := range defHCL.UsesSchemas {
		usesSchemas, err := EvalUsesSchemas(root, resolveAPI, evalctx, usesSchemasHCL, allowFetch)
		if err != nil {
			return typeschema.SchemaNamespaces{}, errors.E(err, usesSchemasHCL.DefRange, "failed to evaluate uses schema")
		}
		schemas.Set(usesSchemasHCL.Name, usesSchemas)
	}

	inputSchemas, err := EvalBundleInputSchemas(evalctx, defHCL)
	if err != nil {
		return typeschema.SchemaNamespaces{}, err
	}

	schemas.Set("input", inputSchemas)
	return schemas, nil
}

// EvalBundle evaluates the bundle.
func EvalBundle(ctx context.Context, root *Root, resolveAPI resolve.API, evalctx *eval.Context, inst *hcl.Bundle, reg *Registry, allowFetch bool) (*Bundle, error) {
	logger := log.With().
		Str("action", "EvalBundle()").
		Str("bundle", inst.Name).
		Logger()

	evaluated := &Bundle{
		Name:    inst.Name,
		Inst:    inst,
		Workdir: inst.Workdir,
		Stacks:  map[project.Path]StackMetadata{},
		Info:    inst.Info,
	}

	// The source supports variables from the given evalctx, but it does not support inputs from the bundle itself.
	// That is because to be able to load the default values from inputs, we need to resolve definition, which requires the source.
	src, err := EvalString(evalctx, inst.Source.Expr, "source")
	if err != nil {
		return nil, err
	}

	evaluated.Source = src

	if inst.UUID != nil && inst.UUID.Expr != nil {
		uuidStr, err := EvalString(evalctx, inst.UUID.Expr, "uuid")
		if err != nil {
			return nil, err
		}
		uuidParsed, err := uuid.Parse(uuidStr)
		if err != nil {
			return nil, errors.E(err, inst.UUID.Range, "uuid '%s' is not valid", uuidStr)
		}
		evaluated.UUID = uuidParsed.String()
	}

	resolvedSrc, err := resolveAPI.Resolve(root.HostDir(), src, resolve.Bundle, allowFetch)
	if err != nil {
		return nil, errors.E(err, inst.Source.Range)
	}

	evaluated.ResolvedSource = resolvedSrc

	defineBundleTree, ok := root.Lookup(resolvedSrc)
	if !ok {
		err := root.LoadSubTree(resolvedSrc)
		if err != nil {
			return nil, errors.E(err, inst.Source.Range, "source '%s' could not be loaded", src)
		}

		defineBundleTree, ok = root.Lookup(resolvedSrc)
		if !ok {
			return nil, errors.E(inst.Source.Range, "source '%s' not found", src)
		}
	}

	bundlecfg := defineBundleTree.Node

	if len(bundlecfg.Defines) == 0 {
		return nil, errors.E(inst.Source.Range, "source '%s' is not a bundle definition", src)
	}

	var defineBundle *hcl.DefineBundle
	for _, define := range bundlecfg.Defines {
		if define.Bundle != nil {
			defineBundle = define.Bundle
			break
		}
	}

	if defineBundle == nil {
		return nil, errors.E(inst.Source.Range, "source '%s' is not a bundle definition", src)
	}

	evaluated.Environment, err = checkBundleEnvironment(evalctx, inst, defineBundle, reg.Environments)
	if err != nil {
		return nil, err
	}

	// Required attribute
	evaluated.Class, err = EvalString(evalctx, defineBundle.Metadata.Class.Expr, "class")
	if err != nil {
		return nil, err
	}

	var uuidVal cty.Value
	if evaluated.UUID != "" {
		uuidVal = cty.StringVal(evaluated.UUID)
	} else {
		uuidVal = cty.NullVal(cty.String)
	}

	// bundle loose generate blocks
	evaluated.Generate = defineBundleTree.Node.Generate

	schemas, err := EvalBundleSchemaNamespaces(root, resolveAPI, evalctx, defineBundle, allowFetch)
	if err != nil {
		return nil, err
	}

	evalctx = evalctx.ChildContext()

	bundleNS := map[string]cty.Value{
		"class":       cty.StringVal(evaluated.Class),
		"uuid":        uuidVal,
		"environment": MakeEnvObject(evaluated.Environment),
	}

	evalctx.SetNamespace("bundle", bundleNS)

	// We enable preemptable mode here. This function may suspend execution in case
	// tm_bundle(key) is not available yet.
	evalctx.SetFunction("tm_bundle", BundleFunc(ctx, reg, evaluated.Environment, true))

	evaluated.Inputs, err = EvalInputs(
		evalctx,
		"bundle",
		inst.Info,
		inst.Inputs,
		inst.InputsAttr,
		defineBundle.Inputs,
		schemas)
	if err != nil {
		return nil, err
	}

	if defineBundle.Alias != nil {
		evaluated.Alias, err = EvalString(evalctx, defineBundle.Alias.Expr, "alias")
		if err != nil {
			return nil, err
		}
	} else {
		// Fallback to path:name as alias.
		evaluated.Alias = fmt.Sprintf("%s:%s", inst.Workdir.String(), inst.Name)
	}

	evaluated.Exports, err = evalBundleExports(evalctx, inst, defineBundle, evaluated.Inputs)
	if err != nil {
		return nil, err
	}

	compBundleObject, _ := evalctx.GetNamespace("bundle")

	// instantiate each stack metadata
	for _, stackDef := range defineBundle.Stacks {
		envComponents := make([]*hcl.Component, len(stackDef.Components))
		for i, comp := range stackDef.Components {
			clonedComp := *comp
			clonedComp.Environment = inst.Environment
			clonedComp.BundleObject = &compBundleObject
			envComponents[i] = &clonedComp
		}

		stackMeta := StackMetadata{
			Components: envComponents,
		}

		if stackDef.Condition != nil {
			value, err := evalctx.Eval(stackDef.Condition.Expr)
			if err != nil {
				return nil, err
			}
			if value.Type() != cty.Bool {
				return nil, errors.E(
					ErrSchema,
					"condition has type %s but must be boolean",
					value.Type().FriendlyName(),
				)
			}
			stackMeta.Skipped = value.False()
		}

		attrValues := []*ast.Attribute{
			stackDef.Metadata.Path,
			stackDef.Metadata.Name,
			stackDef.Metadata.Description,
			stackDef.Metadata.Tags,
			stackDef.Metadata.After,
			stackDef.Metadata.Before,
			stackDef.Metadata.Wants,
			stackDef.Metadata.WantedBy,
			stackDef.Metadata.Watch,
		}
		for _, attr := range attrValues {
			if attr == nil {
				continue
			}

			logger.Debug().Msgf("evaluating attribute %s", attr.Name)

			// print bundle namespace values
			ns, ok := evalctx.GetNamespace("bundle")
			if !ok {
				logger.Error().Msg("bundle namespace not found")
				continue
			}

			values := ns.AsValueMap()
			for k, v := range values {
				logger.Debug().Msgf("bundle namespace value %s: %v", k, v)
			}

			switch attr.Name {
			case "path":
				var str string
				str, err = EvalString(evalctx, attr.Expr, attr.Name)
				if strings.HasPrefix(str, "/") {
					stackMeta.Dir = project.NewPath(str)
				} else {
					stackMeta.Dir = inst.Workdir.Join(str)
				}
			case "name":
				stackMeta.Name, err = EvalString(evalctx, attr.Expr, attr.Name)
			case "description":
				stackMeta.Description, err = evalOptionalString(evalctx, attr.Expr, attr.Name)
			case "tags":
				stackMeta.Tags, err = evalOptionalStringList(evalctx, attr.Expr, attr.Name)
			case "after":
				stackMeta.After, err = evalOptionalStringList(evalctx, attr.Expr, attr.Name)
			case "before":
				stackMeta.Before, err = evalOptionalStringList(evalctx, attr.Expr, attr.Name)
			case "wants":
				stackMeta.Wants, err = evalOptionalStringList(evalctx, attr.Expr, attr.Name)
			case "wanted_by":
				stackMeta.WantedBy, err = evalOptionalStringList(evalctx, attr.Expr, attr.Name)
			case "watch":
				stackMeta.Watch, err = evalOptionalStringList(evalctx, attr.Expr, attr.Name)
			}
			if err != nil {
				return nil, err
			}
		}

		if err := stackMeta.Validate(); err != nil {
			return nil, err
		}

		evaluated.Stacks[stackMeta.Dir] = stackMeta
	}

	return evaluated, nil
}

// EvalBundleInputSchemas evaluates input definitions into type schemas for a bundle.
func EvalBundleInputSchemas(evalctx *eval.Context, def *hcl.DefineBundle) ([]*typeschema.Schema, error) {
	var ret []*typeschema.Schema

	for _, inputHCL := range def.Inputs {
		schema, err := EvalInputSchema(evalctx, inputHCL)
		if err != nil {
			return nil, err
		}
		ret = append(ret, schema)
	}

	return ret, nil
}

// MakeEnvObject converts an Environment into a cty object value.
func MakeEnvObject(env *Environment) cty.Value {
	if env == nil {
		return cty.ObjectVal(map[string]cty.Value{
			"available": cty.BoolVal(false),
		})
	}
	return cty.ObjectVal(map[string]cty.Value{
		"available":    cty.BoolVal(true),
		"id":           cty.StringVal(env.ID),
		"name":         cty.StringVal(env.Name),
		"description":  cty.StringVal(env.Description),
		"promote_from": cty.StringVal(env.PromoteFrom),
	})
}

func extractInputVars(rootNS string, attr *ast.Attribute) []string {
	var results []string
	if attr == nil {
		return results
	}

	toBeEvaluatedVars := attr.Expr.Variables()
	for _, traversal := range toBeEvaluatedVars {
		if traversal.RootName() != rootNS {
			continue
		}

		if traversal.IsRelative() {
			continue
		}

		if len(traversal) < 3 {
			continue
		}

		// traversal[1] is probably "input"
		vv := traversal[1]
		inputAttr, ok := vv.(hhcl.TraverseAttr)
		if !ok || inputAttr.Name != "input" {
			continue
		}

		vv = traversal[2]
		var attrName string
		switch attr := vv.(type) {
		case hhcl.TraverseAttr:
			attrName = attr.Name
		case hhcl.TraverseSplat:
			// ignore
		case hhcl.TraverseIndex:
			if !attr.Key.Type().Equals(cty.String) {
				break
			}
			attrName = attr.Key.AsString()
		}

		if attrName == "" {
			continue
		}
		results = append(results, attrName)
	}
	return results
}

func evalBundleExports(evalctx *eval.Context, inst *hcl.Bundle, def *hcl.DefineBundle, inputs map[string]cty.Value) (map[string]cty.Value, error) {
	evalctx = evalctx.ChildContext()

	exports := map[string]cty.Value{}

	errs := errors.L()
	filePath := inst.Info.Path()

	filePathNS := cty.ObjectVal(map[string]cty.Value{
		"absolute": cty.StringVal(inst.Info.HostPath()),
		"basename": cty.StringVal(path.Base(filePath.String())),
		"relative": cty.StringVal(filePath.String()),
	})
	fileNS := cty.ObjectVal(map[string]cty.Value{
		"path": filePathNS,
	})

	// For the exports evaluation, move namespace "global" to "bundle.global".
	globalsNamespace, _ := evalctx.GetNamespace("global")

	bundleInputsNamespace := map[string]cty.Value{
		"input":  cty.ObjectVal(inputs),
		"global": globalsNamespace,
		"file":   fileNS,
	}
	evalctx.SetNamespace("bundle", bundleInputsNamespace)
	evalctx.SetNamespace("global", map[string]cty.Value{})

	for name, exportDef := range def.Exports {
		val, err := evalctx.Eval(exportDef.Value.Expr)
		if err != nil {
			errs.Append(err)
			continue
		}

		exports[name] = cty.ObjectVal(map[string]cty.Value{
			"value": val,
		})
	}
	if err := errs.AsError(); err != nil {
		return nil, err
	}

	return exports, nil
}

func checkBundleEnvironment(evalctx *eval.Context, inst *hcl.Bundle, def *hcl.DefineBundle, envs []*Environment) (*Environment, error) {
	required := false
	if def.Environments.Required != nil {
		var err error
		required, err = EvalBool(evalctx, def.Environments.Required.Expr, "required")
		if err != nil {
			return nil, err
		}
	}

	if required && len(envs) == 0 {
		return nil, errors.E(inst.Info, "bundle '%s' requires an environment but no environments are configured", inst.Name)
	}

	if inst.Environment == nil {
		if required {
			return nil, errors.E(inst.Info, "bundle '%s' requires an environment but none was specified", inst.Name)
		}
		return nil, nil
	} else if !required {
		return nil, errors.E(inst.Environment.Range, "the bundle defiend at '%s' does not support environments", inst.Info.String())
	}

	envID, err := EvalString(evalctx, inst.Environment.Expr, "environment")
	if err != nil {
		return nil, err
	}

	for _, env := range envs {
		if env.ID == envID {
			return env, nil
		}
	}

	return nil, errors.E(inst.Environment.Range, "bundle '%s' specifies environment '%s' but it was not found", inst.Name, envID)
}

// BundleDefinitionEntry pairs a config tree with its bundle definition.
type BundleDefinitionEntry struct {
	Tree   *Tree
	Define *hcl.DefineBundle
}

// ListLocalBundleDefinitions lists all bundle definitions found under the given directory.
func ListLocalBundleDefinitions(root *Root, dir project.Path) ([]BundleDefinitionEntry, error) {
	srcHostDir := dir.HostPath(root.HostDir())
	srcAbsDir := project.PrjAbsPath(root.HostDir(), srcHostDir)

	var r []BundleDefinitionEntry

	bundlesDir, ok := root.Lookup(srcAbsDir)
	if !ok {
		return r, nil
	}

	for _, subdir := range bundlesDir.AsList() {
		// Ignore the folder that contains the definitions of installed remote packages.
		if subdir.Dir().HasDirPrefix("/.terramate") {
			continue
		}

		for _, def := range subdir.Node.Defines {
			if def.Bundle == nil {
				continue
			}

			r = append(r, BundleDefinitionEntry{
				Tree:   subdir,
				Define: def.Bundle,
			})
		}
	}

	slices.SortFunc(r, func(a, b BundleDefinitionEntry) int {
		return strings.Compare(a.Tree.Dir().String(), b.Tree.Dir().String())
	})

	return r, nil
}

// LoadSingleBundleDefinition loads a single bundle definition from the given directory.
func LoadSingleBundleDefinition(root *Root, dir project.Path) (*BundleDefinitionEntry, error) {
	err := root.LoadSubTree(dir)
	if err != nil {
		return nil, errors.E(err, "failed to load bundle definitions")
	}

	tree, ok := root.Lookup(dir)
	if !ok {
		return nil, errors.E("no bundle definitions found")
	}

	for _, def := range tree.Node.Defines {
		if def.Bundle == nil {
			continue
		}

		return &BundleDefinitionEntry{
			Tree:   tree,
			Define: def.Bundle,
		}, nil
	}

	return nil, errors.E("no bundle definitions found")
}

// EvalMetadata evaluates a metadata block into a Metadata value.
func EvalMetadata(_ *Root, evalctx *eval.Context, tree *Tree, def *hcl.Metadata) (*Metadata, error) {
	var err error
	md := &Metadata{
		Dir: tree.Dir(),
	}

	// Required
	md.Class, err = EvalString(evalctx, def.Class.Expr, "class")
	if err != nil {
		return nil, err
	}

	// Required
	md.Name, err = EvalString(evalctx, def.Name.Expr, "name")
	if err != nil {
		return nil, err
	}

	// Required
	md.Version, err = EvalString(evalctx, def.Version.Expr, "version")
	if err != nil {
		return nil, err
	}

	// Optional
	if def.Description != nil {
		md.Description, err = EvalString(evalctx, def.Description.Expr, "description")
		if err != nil {
			return nil, err
		}
	}

	return md, nil
}

// EvalBundleDefinition evaluates a bundle definition's scaffolding metadata.
func EvalBundleDefinition(_ *Root, evalctx *eval.Context, def *hcl.DefineBundle) (*BundleDefinition, error) {
	r := &BundleDefinition{}
	var err error

	if def.Scaffolding.Name != nil {
		r.ScaffoldingName, err = EvalString(evalctx, def.Scaffolding.Name.Expr, "scaffolding.name")
		if err != nil {
			return nil, err
		}
	}

	if def.Scaffolding.Path != nil {
		r.ScaffoldingPath, err = EvalString(evalctx, def.Scaffolding.Path.Expr, "scaffolding.path")
		if err != nil {
			return nil, err
		}
	}

	return r, nil
}

// EvalBundleInputDefinitions evaluates the input definitions of a bundle.
// evalctx should contain exports.
func EvalBundleInputDefinitions(evalctx *eval.Context, def *hcl.DefineBundle, schemas typeschema.SchemaNamespaces) ([]*InputDefinition, error) {
	var r []*InputDefinition

	partitionByPrompt := func(a, b *hcl.DefineInput) int {
		if a.Prompt != nil && b.Prompt != nil {
			return 0
		} else if a.Prompt != nil {
			return -1
		} else if b.Prompt != nil {
			return 1
		}
		return 0
	}

	orderedInputs := slices.Collect(maps.Values(def.Inputs))
	slices.SortFunc(orderedInputs, func(a, b *hcl.DefineInput) int {
		return cmp.Or(
			partitionByPrompt(a, b),
			strings.Compare(a.DefRange.Filename, b.DefRange.Filename),
			cmp.Compare(a.DefRange.Start.Line, b.DefRange.Start.Line),
		)
	})

	for _, inputDef := range orderedInputs {
		var err error
		input := &InputDefinition{
			Name: inputDef.Name,
		}

		if inputDef.Prompt != nil {
			input.Prompt, err = EvalString(evalctx, inputDef.Prompt.Expr, "prompt")
			if err != nil {
				return nil, err
			}
		}

		if inputDef.Multiselect != nil {
			input.Multiselect, err = EvalBool(evalctx, inputDef.Multiselect.Expr, "multiselect")
			if err != nil {
				return nil, err
			}
		}

		// This has been evaluated before already.
		schema, err := schemas.Lookup("input." + inputDef.Name)
		if err != nil {
			return nil, err
		}

		var optionValueType typeschema.Type

		if input.Multiselect {
			if !typeschema.IsCollectionType(schema.Type) {
				return nil, errors.E(inputDef.DefRange, "type for multiselect must be a list/map, got %s instead", schema.Type.String())
			}
			optionValueType = typeschema.UnwrapValueType(schema.Type)
		} else {
			optionValueType = schema.Type
		}

		input.Type = schema.Type

		if inputDef.Default != nil {
			// Try to evaluate, but it may fail if it contains variables.
			input.PromptDefault, _ = tryEvaluateExpr(evalctx, inputDef.Default.Expr)
		}

		if inputDef.Multiline != nil {
			input.Multiline, err = EvalBool(evalctx, inputDef.Multiline.Expr, "multiline")
			if err != nil {
				return nil, err
			}
		}

		if inputDef.Options != nil {
			input.Options, err = evalOptions(evalctx, inputDef.Name, inputDef.Options.Expr, optionValueType, schemas)
			if err != nil {
				return nil, err
			}
		}

		if inputDef.Description != nil {
			input.Description, err = EvalString(evalctx, inputDef.Description.Expr, "description")
			if err != nil {
				return nil, err
			}
		}

		r = append(r, input)
	}

	return r, nil
}

func parseNamedValue(obj cty.Value, valueType typeschema.Type, evalctx *eval.Context, schemas typeschema.SchemaNamespaces) (NamedValue, error) {
	// precondition: val is an object/map
	iter := obj.ElementIterator()
	result := NamedValue{}

	var hasName, hasValue bool

	for iter.Next() {
		key, val := iter.Element()
		if key.Type() != cty.String {
			panic("unreachable")
		}
		switch key.AsString() {
		case "name":
			if val.Type() != cty.String {
				return NamedValue{}, errors.E("'name' must be of type string, got %s", val.Type().FriendlyName())
			}
			result.Name = val.AsString()
			hasName = true
		case "value":
			var err error
			val, err = valueType.Apply(val, evalctx, schemas, true)
			if err != nil {
				return NamedValue{}, err
			}
			result.Value = val
			hasValue = true
		default:
			return NamedValue{}, errors.E("invalid attribute '%s'", key.AsString())
		}

	}
	if !hasName || !hasValue {
		return NamedValue{}, errors.E("expected object with attributes 'name' and 'value'")
	}

	return result, nil
}

func evalOptions(evalctx *eval.Context, inputName string, expr hhcl.Expression, valueType typeschema.Type, schemas typeschema.SchemaNamespaces) ([]NamedValue, error) {
	options := []NamedValue{}

	val, err := evalctx.Eval(expr)
	if err != nil {
		return nil, errors.E(err, expr.Range(), "%s: evaluating options", inputName)
	}
	if val.IsNull() {
		return nil, errors.E(expr.Range(), "%s: options is null", inputName)
	}

	valType := val.Type()
	if !valType.IsListType() && !valType.IsTupleType() {
		return nil, errors.E(expr.Range(), "%s: options must be a list/tuple, got %s instead", inputName, valType.FriendlyName())
	}

	iter := val.ElementIterator()
	for iter.Next() {
		key, elem := iter.Element()
		if key.Type() != cty.Number {
			panic("unreachable")
		}
		index, _ := key.AsBigFloat().Int64()

		elemType := elem.Type()
		if elemType == cty.String {
			_, isBundleType := valueType.(*typeschema.BundleType)
			// A bundle reference is stored as a string, but the type doesn't say "string".
			if valueType.String() != "string" && !isBundleType {
				return nil, errors.E(expr.Range(), "%s: invalid value type in options at index %d", inputName, index)
			}
			options = append(options, NamedValue{Name: elem.AsString(), Value: elem})

		} else if elemType.IsObjectType() || elemType.IsMapType() {
			namedValue, err := parseNamedValue(elem, valueType, evalctx, schemas)
			if err != nil {
				return nil, errors.E(err, expr.Range(), "%s: invalid element in options at index %d", inputName, index)
			}
			options = append(options, namedValue)

		} else {
			return nil, errors.E(expr.Range(), "%s: options element at index %d must be a string or an object/map, got %s instead",
				inputName, index, elemType.FriendlyName())
		}
	}

	return options, nil
}

func tryEvaluateExpr(evalctx *eval.Context, expr hhcl.Expression) (cty.Value, bool) {
	val, err := evalctx.Eval(expr)
	if err == nil {
		return val, true
	}

	// Fall back to raw tokens as a string.
	tokens := ast.TokensForExpression(expr).Bytes()
	return cty.StringVal(strings.TrimSpace(string(tokens))), false
}
