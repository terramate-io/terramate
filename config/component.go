// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package config

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/zclconf/go-cty/cty"

	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/generate/resolve"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/hcl/info"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/run/dag"
	"github.com/terramate-io/terramate/typeschema"
)

// Component represents the instantiation of a component.
type Component struct {
	Name           string
	Source         string
	ResolvedSource project.Path
	Environment    *Environment
	Inputs         map[string]cty.Value
	Info           info.Range
	Skipped        bool
}

// EvalComponentSchemaNamespaces evaluates the uses_schemas blocks of a component definition.
func EvalComponentSchemaNamespaces(root *Root, resolveAPI resolve.API, evalctx *eval.Context, defHCL *hcl.DefineComponent, allowFetch bool) (typeschema.SchemaNamespaces, error) {
	schemas := typeschema.NewSchemaNamespaces()

	for _, usesSchemasHCL := range defHCL.UsesSchemas {
		usesSchemas, err := EvalUsesSchemas(root, resolveAPI, evalctx, usesSchemasHCL, allowFetch)
		if err != nil {
			return typeschema.SchemaNamespaces{}, errors.E(err, usesSchemasHCL.DefRange, "failed to evaluate uses schema")
		}
		schemas.Set(usesSchemasHCL.Name, usesSchemas)
	}

	inputSchemas, err := EvalComponentInputSchemas(evalctx, defHCL)
	if err != nil {
		return typeschema.SchemaNamespaces{}, err
	}

	schemas.Set("input", inputSchemas)
	return schemas, nil
}

// EvalComponentInputSchemas evaluates input definitions into type schemas for a component.
func EvalComponentInputSchemas(evalctx *eval.Context, def *hcl.DefineComponent) ([]*typeschema.Schema, error) {
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

// EvalComponent evaluates a component and returns the component with the inputs resolved.
// evalctx will be modified to contain `component.input`.
func EvalComponent(root *Root, resolveAPI resolve.API, evalctx *eval.Context, component *hcl.Component, reg *Registry, allowFetch bool) (*Component, *hcl.Config, error) {
	if component.Source == nil {
		return nil, nil, errors.E(
			hcl.ErrComponentMissingSourceAttribute,
			"component %s is missing the required source attribute",
			component.Name,
		)
	}

	evaluated := &Component{
		Name:   component.Name,
		Inputs: map[string]cty.Value{},
		Info:   component.Info,
	}

	src, err := EvalString(evalctx, component.Source.Expr, "source")
	if err != nil {
		return nil, nil, err
	}

	if component.Condition != nil {
		value, err := evalctx.Eval(component.Condition.Expr)
		if err != nil {
			return nil, nil, err
		}
		if value.Type() != cty.Bool {
			return nil, nil, errors.E(
				ErrSchema,
				"condition has type %s but must be boolean",
				value.Type().FriendlyName(),
			)
		}
		evaluated.Skipped = value.False()
	}

	if evaluated.Skipped {
		return evaluated, nil, nil
	}

	evaluated.Source = src

	resolvedSrc, err := resolveAPI.Resolve(root.HostDir(), src, resolve.Component, allowFetch, resolve.WithParentSource(component.FromBundleSource))
	if err != nil {
		return nil, nil, errors.E(err, component.Source.Range)
	}

	evaluated.ResolvedSource = resolvedSrc

	compTree, ok := root.Lookup(evaluated.ResolvedSource)
	if !ok {
		err := root.LoadSubTree(evaluated.ResolvedSource)
		if err != nil {
			return nil, nil, errors.E(err, component.Source.Range, "source '%s' could not be loaded", evaluated.Source)
		}

		compTree, ok = root.Lookup(evaluated.ResolvedSource)
		if !ok {
			return nil, nil, errors.E(component.Source.Range, "source '%s' not found", evaluated.Source)
		}
	}

	compCfg := &compTree.Node
	if len(compCfg.Defines) == 0 {
		return nil, nil, errors.E(component.Source.Range, "source '%s' is not a component definition", evaluated.Source)
	}

	var compDef *hcl.DefineComponent
	for _, define := range compCfg.Defines {
		if define.Component != nil {
			compDef = define.Component
			break
		}
	}

	if compDef == nil {
		return nil, nil, errors.E(component.Source.Range, "source '%s' is not a component definition", evaluated.Source)
	}

	evaluated.Environment, err = checkComponentEnvironment(evalctx, component, reg.Environments)
	if err != nil {
		return nil, nil, err

	}

	schemas, err := EvalComponentSchemaNamespaces(root, resolveAPI, evalctx, compDef, allowFetch)
	if err != nil {
		return nil, nil, err
	}

	evalctx.SetFunction("tm_bundle", BundleFunc(context.TODO(), reg, evaluated.Environment, false))
	evalctx.SetFunction("tm_bundles", BundlesFunc(reg, evaluated.Environment))

	compNS := map[string]cty.Value{
		"environment": MakeEnvObject(evaluated.Environment),
	}
	evalctx.SetNamespace("component", compNS)

	evaluated.Inputs, err = EvalInputs(
		evalctx,
		"component",
		component.Info,
		component.Inputs,
		component.InputsAttr,
		compDef.Inputs,
		schemas)
	if err != nil {
		return nil, nil, err
	}

	return evaluated, compCfg, nil
}

// EvalInputs evaluates input values against their definitions and schemas.
// evalctx will be modified to contain `{rootNS}.input`.
func EvalInputs(evalctx *eval.Context, rootNS string, instRange info.Range, inputs *ast.MergedBlock, inputsAttr *ast.Attribute, inputDefs map[string]*hcl.DefineInput, schemas typeschema.SchemaNamespaces) (map[string]cty.Value, error) {
	type pendingProvided struct {
		value    cty.Value
		defRange info.Range
	}

	type pendingDefault struct {
		expr *ast.Attribute
	}

	providedInputs := map[string]*pendingProvided{}

	// input blocks have precedence over inputs attribute
	if inputsAttr != nil {
		inputsObj, err := evalObject(evalctx, inputsAttr.Expr, "inputs")
		if err != nil {
			return nil, errors.E(err, inputsAttr.Range)
		}
		vals := inputsObj.AsValueMap()
		for name, v := range vals {
			if _, found := inputDefs[name]; !found {
				// Silently ignore input attributes that are not part of the input definition.
				continue
			}

			providedInputs[name] = &pendingProvided{
				value:    v,
				defRange: inputsAttr.Range,
			}
		}
	}

	errs := errors.L()
	if inputs != nil {
		for name, attr := range inputs.Attributes {
			if _, found := inputDefs[name]; !found {
				errs.Append(errors.E(hcl.ErrTerramateSchema, attr.Range, "unknown input '%s' in inputs block", name))
				continue
			}
			// Override
			v, err := evalctx.Eval(attr.Expr)
			if err != nil {
				errs.Append(errors.E(attr.Range, err))
				continue
			}
			providedInputs[name] = &pendingProvided{
				value:    v,
				defRange: attr.Range,
			}
		}
		if err := errs.AsError(); err != nil {
			return nil, err
		}
	}

	// This will contain all inputs with defaults that can reference other inputs.
	// There are two cases for this:
	// 1. No value was provided: Compute the default and apply defaults from object attributes.
	// 2. A value was provided: Only apply defaults from object attributes.
	inputsDAG := dag.New[any]()

	for name, input := range inputDefs {
		var nodeVal any
		var ancestors []string

		if providedVal, isProvided := providedInputs[name]; !isProvided {
			if input.Default == nil {
				errs.Append(errors.E(instRange, "input '%s' is neither provided, nor has a default (defined at '%s')", name, input.DefRange.String()))
				continue
			}

			// TODO(snk): This will not work transitively on A + B
			ancestors = append(ancestors, extractInputVars(rootNS, input.Default)...)
			for _, attr := range input.ObjectAttributes {
				ancestors = append(ancestors, extractInputVars(rootNS, attr.Default)...)
			}
			nodeVal = &pendingDefault{
				expr: input.Default,
			}
		} else {
			for _, attr := range input.ObjectAttributes {
				ancestors = append(ancestors, extractInputVars(rootNS, attr.Default)...)
			}
			nodeVal = providedVal
		}

		ancestorIDs := make([]dag.ID, 0, len(ancestors))
		for _, e := range ancestors {
			if _, found := inputDefs[e]; found {
				ancestorIDs = append(ancestorIDs, dag.ID(e))
			}
		}

		err := inputsDAG.AddNode(dag.ID(name), nodeVal, nil, ancestorIDs)
		errs.Append(err)
	}

	if err := errs.AsError(); err != nil {
		return nil, err
	}

	if reason, err := inputsDAG.Validate(); err != nil {
		return nil, errors.E(fmt.Sprintf("reference cycle: %s", reason), err)
	}

	var nsVals map[string]cty.Value
	if ns, ok := evalctx.GetNamespace(rootNS); ok {
		nsVals = ns.AsValueMap()
	} else {
		nsVals = map[string]cty.Value{}
	}

	evalctx.SetNamespace(rootNS, nsVals)

	resultingInputs := map[string]cty.Value{}

	for _, inputName := range inputsDAG.Order() {
		nodeVal, err := inputsDAG.Node(inputName)
		if err != nil {
			// We catch this later because there will be an undefined variable reference.
			continue
		}

		var v cty.Value

		switch nodeVal := nodeVal.(type) {
		case *pendingDefault:
			v, err = evalctx.Eval(nodeVal.expr.Expr)
			if err != nil {
				return nil, errors.E(nodeVal.expr.Expr.Range(), err)
			}
			v, err = applyInputSchema(string(inputName), v, evalctx, schemas)
			if err != nil {
				return nil, errors.E(err, nodeVal.expr.Expr.Range(), "%s: failed to validate input type", inputName)
			}

		case *pendingProvided:
			v, err = applyInputSchema(string(inputName), nodeVal.value, evalctx, schemas)
			if err != nil {
				return nil, errors.E(err, nodeVal.defRange, "%s: failed to validate input type", inputName)
			}
		default:
			panic("internal error: report this as a bug")
		}

		resultingInputs[string(inputName)] = cty.ObjectVal(map[string]cty.Value{
			"value": v,
		})
		nsVals["input"] = cty.ObjectVal(resultingInputs)
		evalctx.SetNamespace(rootNS, nsVals)
	}
	return resultingInputs, nil
}

func checkComponentEnvironment(evalctx *eval.Context, inst *hcl.Component, envs []*Environment) (*Environment, error) {
	if inst.Environment == nil {
		return nil, nil
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

	return nil, errors.E(inst.Environment.Range, "component '%s' specifies environment '%s' but it was not found", inst.Name, envID)
}

// ComponentDefinitionEntry pairs a config tree with its component definition.
type ComponentDefinitionEntry struct {
	Tree   *Tree
	Define *hcl.DefineComponent
}

// ListLocalComponentDefinitions lists all component definitions found under the given directory.
func ListLocalComponentDefinitions(root *Root, dir project.Path) ([]ComponentDefinitionEntry, error) {
	srcHostDir := dir.HostPath(root.HostDir())
	srcAbsDir := project.PrjAbsPath(root.HostDir(), srcHostDir)

	var r []ComponentDefinitionEntry

	componentsDir, ok := root.Lookup(srcAbsDir)
	if !ok {
		return r, nil
	}

	for _, subdir := range componentsDir.AsList() {
		// Ignore the folder that contains the definitions of installed remote packages.
		if subdir.Dir().HasDirPrefix("/.terramate") {
			continue
		}

		for _, def := range subdir.Node.Defines {
			if def.Component == nil {
				continue
			}

			r = append(r, ComponentDefinitionEntry{
				Tree:   subdir,
				Define: def.Component,
			})
		}
	}

	slices.SortFunc(r, func(a, b ComponentDefinitionEntry) int {
		return strings.Compare(a.Tree.Dir().String(), b.Tree.Dir().String())
	})

	return r, nil
}
