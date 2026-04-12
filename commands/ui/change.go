// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package ui

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"

	"github.com/google/uuid"
	hhcl "github.com/terramate-io/hcl/v2"
	"github.com/terramate-io/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"

	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/stdlib"
	"github.com/terramate-io/terramate/typeschema"
	"github.com/terramate-io/terramate/yaml"
)

// ChangeKind indicates the type of change.
type ChangeKind string

// ChangeCreate and the following constants enumerate the supported change kinds.
const (
	ChangeCreate   ChangeKind = "change_create"
	ChangeReconfig ChangeKind = "change_reconfig"
	ChangePromote  ChangeKind = "change_promote"
)

// Change represents a bundle change (create, reconfigure, or promote).
type Change struct {
	Kind        ChangeKind
	HostPath    string
	ProjectPath string
	Name        string
	UUID        string
	Source      string
	Alias       string

	DisplayName string
	Metadata    config.Metadata

	BundleDefEntry *config.BundleDefinitionEntry
	InputDefs      []*config.InputDefinition // Original input definitions
	Values         map[string]cty.Value      // Collected values at time of creation

	Env     *config.Environment
	FromEnv *config.Environment

	OriginalBundle *config.Bundle       // Only set for reconfig and promote
	OriginalValues map[string]cty.Value // Baseline values from disk (for change detection)

	Warnings []string // Non-fatal warnings surfaced after resolving the change

}

// NewCreateChange builds a Change that represents a new bundle creation.
func NewCreateChange(
	est *EngineState,
	activeEnv *config.Environment,
	bde *config.BundleDefinitionEntry,
	schemactx typeschema.EvalContext,
	inputDefs []*config.InputDefinition,
	values map[string]cty.Value,
) (Change, error) {
	schemactx = schemactx.ChildContext()

	// Rebind bundle() functions to the current registry so that references
	// to bundles created during this session (e.g. nested bundles that were
	// saved immediately) are resolvable.
	schemactx.Evalctx.SetFunction(stdlib.Name("bundle"), config.BundleFunc(context.TODO(), est.Registry.Registry, activeEnv, false))
	schemactx.Evalctx.SetFunction(stdlib.Name("bundles"), config.BundlesFunc(est.Registry.Registry, activeEnv))

	// The form may or may not contain values for all defaults.
	// In this step we re-run input evaluation like it would be done if this was a bundle instance that
	// has the form values as inputs. This will ensure we get all the inputs.
	allValues, err := reEvalAllInputs(
		schemactx,
		bde.Tree.HostDir(),
		bde.Define,
		values,
	)
	if err != nil {
		return Change{}, err
	}

	// We check if the bundle has an explicit alias and add it to the context if yes.
	alias, err := setupExplicitBundleAlias(schemactx.Evalctx, bde.Define)
	if err != nil {
		return Change{}, err
	}

	bundleDef, err := config.EvalBundleDefinition(schemactx.Evalctx, bde.Define)
	if err != nil {
		return Change{}, err
	}

	outputPath := bundleDef.ScaffoldingPath
	if outputPath == "" {
		outputPath = extractPseudoString(values, pseudoKeyOutputPath)
	}
	outputName := bundleDef.ScaffoldingName
	if outputName == "" {
		outputName = extractPseudoString(values, pseudoKeyOutputName)
	}

	if outputPath == "" {
		return Change{}, errors.E("Bundle path is not set.")
	}
	if outputName == "" {
		return Change{}, errors.E("Bundle name is not set.")
	}

	var hostPath string
	var projPath string
	if filepath.IsAbs(outputPath) {
		hostPath = project.NewPath(outputPath).HostPath(est.Root.HostDir())
		projPath = outputPath
	} else {

		hostPath = filepath.Join(est.WorkingDir, outputPath)
		projPath = project.PrjAbsPath(est.Root.HostDir(), hostPath).String()
	}
	hostPath = fixupFileExtension("yaml", hostPath)

	// If there is no explicit alias, we fallback to the <path>:<name> default.
	// But we must do this only after these values are known.
	// If there was an explicit alias, it can actually be used in path and name.
	// Sounds circular but it isn't...
	if alias == "" {
		alias = fmt.Sprintf("%s:%s", filepath.Dir(outputPath), outputName)
	}

	var env *config.Environment
	if activeEnv != nil && bundleRequiresEnv(schemactx.Evalctx, bde.Define) {
		env = activeEnv
	}

	// Final check: Is the bundle unique?
	if err := est.Registry.IsBundleUnique(alias, bde.Metadata.Class, hostPath, env); err != nil {
		return Change{}, err
	}

	return Change{
		Kind:           ChangeCreate,
		HostPath:       hostPath,
		ProjectPath:    projPath,
		Name:           outputName,
		UUID:           uuid.NewString(),
		Source:         bde.Source,
		Env:            env,
		Alias:          alias,
		DisplayName:    displayNameFromAlias(alias, outputName),
		Metadata:       *bde.Metadata,
		BundleDefEntry: bde,
		InputDefs:      inputDefs,
		Values:         allValues,
	}, nil
}

// NewReconfigChange builds a Change that represents reconfiguring an existing bundle.
func NewReconfigChange(
	est *EngineState,
	bundle *config.Bundle,
	bde *config.BundleDefinitionEntry,
	schemactx typeschema.EvalContext,
	inputDefs []*config.InputDefinition,
	values map[string]cty.Value,
) (Change, error) {
	schemactx = schemactx.ChildContext()

	// Rebind bundle() functions to the current registry so that references
	// to bundles created/reconfigured during this session are resolvable.
	schemactx.Evalctx.SetFunction(stdlib.Name("bundle"), config.BundleFunc(context.TODO(), est.Registry.Registry, bundle.Environment, false))
	schemactx.Evalctx.SetFunction(stdlib.Name("bundles"), config.BundlesFunc(est.Registry.Registry, bundle.Environment))

	hostPath := bundle.Info.HostPath()
	projPath := project.PrjAbsPath(est.Root.HostDir(), hostPath).String()

	// The form may or may not contain values for all defaults.
	// In this step we re-run input evaluation like it would be done if this was a bundle instance that
	// has the form values as inputs. This will ensure we get all the inputs.
	allValues, err := reEvalAllInputs(
		schemactx,
		hostPath,
		bde.Define,
		values,
	)
	if err != nil {
		return Change{}, err
	}

	// This will only be set if there is an explicit alias.
	newAlias, err := setupExplicitBundleAlias(schemactx.Evalctx, bde.Define)
	if err != nil {
		return Change{}, err
	}

	var warnings []string
	if bundle.Alias != "" && newAlias != "" && bundle.Alias != newAlias {
		warnings = append(warnings, fmt.Sprintf("Alias will change from %q to %q", bundle.Alias, newAlias))
	}

	// The alias can change, if it's an explicit alias. If it's the implicit alias,
	// it just remains path:name since neither will change.
	var alias, displayName string
	if newAlias == "" {
		alias = bundle.Alias
		displayName = bundle.Name
	} else {
		alias = newAlias
		displayName = newAlias
	}

	return Change{
		Kind:           ChangeReconfig,
		HostPath:       hostPath,
		ProjectPath:    projPath,
		Name:           bundle.Name,
		UUID:           bundle.UUID,
		Source:         bundle.Source,
		Env:            bundle.Environment,
		Alias:          alias,
		DisplayName:    displayName,
		Metadata:       bundle.DefinitionMetadata,
		BundleDefEntry: bde,
		InputDefs:      inputDefs,
		Values:         allValues,
		OriginalBundle: bundle,
		OriginalValues: inputsToValueMap(bundle.Inputs),
		Warnings:       warnings,
	}, nil
}

// NewPromoteChange builds a Change that represents promoting a bundle to another environment.
func NewPromoteChange(
	est *EngineState,
	env *config.Environment,
	bundle *config.Bundle,
	bde *config.BundleDefinitionEntry,
	schemactx typeschema.EvalContext,
	inputDefs []*config.InputDefinition,
	values map[string]cty.Value,
) (Change, error) {
	schemactx = schemactx.ChildContext()

	// Rebind bundle() functions to the current registry so that references
	// to bundles promoted during this session are resolvable.
	schemactx.Evalctx.SetFunction(stdlib.Name("bundle"), config.BundleFunc(context.TODO(), est.Registry.Registry, env, false))
	schemactx.Evalctx.SetFunction(stdlib.Name("bundles"), config.BundlesFunc(est.Registry.Registry, env))

	hostPath := bundle.Info.HostPath()
	projPath := project.PrjAbsPath(est.Root.HostDir(), hostPath).String()

	// The form may or may not contain values for all defaults.
	// In this step we re-run input evaluation like it would be done if this was a bundle instance that
	// has the form values as inputs. This will ensure we get all the inputs.
	allValues, err := reEvalAllInputs(
		schemactx,
		hostPath,
		bde.Define,
		values,
	)
	if err != nil {
		return Change{}, wrapMissingBundleRefError(err)
	}

	// This will only be set if there is an explicit alias.
	newAlias, err := setupExplicitBundleAlias(schemactx.Evalctx, bde.Define)
	if err != nil {
		return Change{}, wrapMissingBundleRefError(err)
	}

	var warnings []string
	if bundle.Alias != "" && newAlias != "" && bundle.Alias != newAlias {
		warnings = append(warnings, fmt.Sprintf("Alias will change from %q to %q", bundle.Alias, newAlias))
	}

	// The alias can change, if it's an explicit alias. If it's the implicit alias,
	// it just remains path:name since neither will change.
	var alias, displayName string
	if newAlias == "" {
		alias = bundle.Alias
		displayName = bundle.Name
	} else {
		alias = newAlias
		displayName = newAlias
	}

	return Change{
		Kind:           ChangePromote,
		HostPath:       hostPath,
		ProjectPath:    projPath,
		Name:           bundle.Name,
		UUID:           bundle.UUID,
		Source:         bundle.Source,
		Env:            env,
		FromEnv:        bundle.Environment,
		Alias:          alias,
		DisplayName:    displayName,
		Metadata:       bundle.DefinitionMetadata,
		BundleDefEntry: bde,
		InputDefs:      inputDefs,
		Values:         allValues,
		OriginalBundle: bundle,
		OriginalValues: inputsToValueMap(bundle.Inputs),
		Warnings:       warnings,
	}, nil
}

// reEvalAllInputs evaluates the bundle's input definitions using the prompted
// values as a simulated inputs block, filling in defaults for any inputs that
// were not prompted.
func reEvalAllInputs(
	schemactx typeschema.EvalContext,
	filename string,
	defineBundleHCL *hcl.DefineBundle,
	values map[string]cty.Value,
) (map[string]cty.Value, error) {
	inst := &hcl.Bundle{
		Inputs: ast.NewMergedBlock("inputs", []string{}),
	}
	for k, v := range values {
		if v == cty.NilVal || isPseudoKey(k) {
			continue
		}
		r := hhcl.Range{
			Filename: filename,
		}
		inst.Inputs.Attributes[k] = ast.NewAttribute(filename,
			&hhcl.Attribute{
				Name:  k,
				Expr:  &hclsyntax.LiteralValueExpr{Val: v},
				Range: r,
			})
	}

	tempInputs, err := config.EvalInputs(
		schemactx,
		"bundle",
		inst.Info,
		inst.Inputs,
		inst.InputsAttr,
		defineBundleHCL.Inputs,
	)
	if err != nil {
		return nil, err
	}

	result := make(map[string]cty.Value, len(values))
	for k, v := range values {
		if v != cty.NilVal {
			result[k] = v
		}
	}

	for k, v := range tempInputs {
		if _, exists := result[k]; exists {
			continue
		}
		vm := v.AsValueMap()
		result[k] = vm["value"]
	}
	return result, nil
}

// wrapMissingBundleRefError adds a user-friendly message when a promote/reconfigure
// fails because a referenced bundle doesn't exist in the target environment.
// Detects the HCL error pattern for accessing attributes on a null value.
func wrapMissingBundleRefError(err error) error {
	msg := err.Error()
	if strings.Contains(msg, "This value is null") || strings.Contains(msg, "does not have any attributes") {
		return errors.E(err, "A referenced bundle has not been promoted to this environment yet. Promote dependencies first.")
	}
	return err
}

// Save writes the change to disk as a YAML bundle instance file.
func (c *Change) Save(envs []*config.Environment) error {
	var existing *yaml.BundleInstance

	_, err := os.Stat(c.HostPath)
	overwrite := err == nil

	// If the change doesn't have an env, the bundle doesn't have envs, so no need for merging.
	if overwrite && c.Env != nil {
		var err error
		existing, err = loadBundleYAMLConfig(c.HostPath)
		if err != nil {
			return err
		}
	}

	content, err := c.generateBundleYAML(existing, envs)
	if err != nil {
		return err
	}
	return writeBundleInstance(c.HostPath, content)
}

func (c *Change) generateBundleYAML(existing *yaml.BundleInstance, envs []*config.Environment) (string, error) {
	inputs := yaml.Map[any]{}
	for _, def := range c.InputDefs {
		if isPseudoKey(def.Name) {
			continue
		}
		v, found := c.Values[def.Name]
		if !found {
			continue
		}

		// Bundle-ref values are stored as resolved objects internally
		// but must be written as alias strings in the YAML config.
		if _, isBundleType := def.Type.(*typeschema.BundleType); isBundleType {
			if v.IsKnown() && !v.IsNull() && v.Type().IsObjectType() && v.Type().HasAttribute("alias") {
				v = v.GetAttr("alias")
			}
		}

		yv, err := yaml.ConvertFromCty(v)
		if err != nil {
			return "", err
		}
		inputs = append(inputs, yaml.MapItem[any]{
			Key:   yaml.Attr(def.Name, 0, 0, formatTmdoc(def.Description)),
			Value: yaml.Attr(yv),
		})
	}

	var envID string
	if c.Env != nil {
		envID = c.Env.ID
	}

	if c.UUID == "" {
		c.UUID = uuid.NewString()
	}

	var bundle yaml.BundleInstance
	if envID != "" {
		envBlock := &yaml.BundleEnvironment{
			Source: yaml.Attr[any](c.Source),
			Inputs: yaml.Attr(inputs),
		}
		if existing != nil {
			bundle = mergeBundleYAMLEnv(*existing, envID, envBlock, envs)
		} else {
			bundle = yaml.BundleInstance{
				Name: yaml.Attr(c.Name),
				UUID: yaml.Attr(c.UUID),
				Environments: yaml.Attr(
					yaml.Map[*yaml.BundleEnvironment]{
						{Key: yaml.Attr(envID), Value: yaml.Attr(envBlock)},
					},
				),
			}
		}
	} else {
		bundle = yaml.BundleInstance{
			Name:   yaml.Attr(c.Name),
			UUID:   yaml.Attr(c.UUID),
			Source: yaml.Attr[any](c.Source),
			Inputs: yaml.Attr(inputs),
		}
	}

	var b strings.Builder
	err := yaml.Encode(&bundle, &b)
	if err != nil {
		return "", err
	}
	// Strip any trailling whitespace.
	output := trailingWSRE.ReplaceAllString(b.String(), "")
	return output, nil
}

func loadBundleYAMLConfig(p string) (*yaml.BundleInstance, error) {
	if !hasYAMLConfigExt(p) {
		return nil, errors.E("File %q is not a .tm.yml file.", p)
	}

	r, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = r.Close()
	}()

	var bundle yaml.BundleInstance
	err = yaml.Decode(r, &bundle)
	if err != nil {
		loc := hhcl.Range{
			Filename: p,
		}
		var yamlErr yaml.Error
		if errors.As(err, &yamlErr) {
			loc.Start = hhcl.Pos{Line: yamlErr.Line, Column: yamlErr.Column}
			loc.End = hhcl.Pos{Line: yamlErr.Line, Column: yamlErr.Column}
		}

		return nil, errors.E(err, loc)
	}

	return &bundle, nil
}

func hasYAMLConfigExt(fn string) bool {
	return strings.HasSuffix(fn, ".tm.yml") || strings.HasSuffix(fn, ".tm.yaml")
}

func mergeBundleYAMLEnv(existing yaml.BundleInstance, envID string, env *yaml.BundleEnvironment, envs []*config.Environment) yaml.BundleInstance {
	merged := existing

	// If the same entry already is in spec of the existing bundle, then don't write it.
	var filteredInputs yaml.Map[any]
	for _, input := range env.Inputs.V {
		if !slices.ContainsFunc(merged.Inputs.V, func(other yaml.MapItem[any]) bool {
			return input.Key.V == other.Key.V &&
				input.Key.HeadComment == other.Key.HeadComment &&
				input.Key.LineComment == other.Key.LineComment &&
				input.Key.FootComment == other.Key.FootComment &&
				input.Value.HeadComment == other.Value.HeadComment &&
				input.Value.LineComment == other.Value.LineComment &&
				input.Value.FootComment == other.Value.FootComment &&
				reflect.DeepEqual(input.Value.V, other.Value.V)
		}) {
			filteredInputs = append(filteredInputs, input)
		}
	}
	env.Inputs.V = filteredInputs

	newMapItem := yaml.MapItem[*yaml.BundleEnvironment]{Key: yaml.Attr(envID), Value: yaml.Attr(env)}

	replaceIndex := slices.IndexFunc(merged.Environments.V, func(a yaml.MapItem[*yaml.BundleEnvironment]) bool {
		return a.Key.V == envID
	})
	if replaceIndex != -1 {
		merged.Environments.V[replaceIndex] = newMapItem
	} else {
		indexMap := make(map[string]int, len(envs))
		for i, e := range envs {
			indexMap[e.ID] = i
		}
		merged.Environments.V = append(merged.Environments.V, newMapItem)
		slices.SortFunc(merged.Environments.V, func(a, b yaml.MapItem[*yaml.BundleEnvironment]) int {
			return cmp.Compare(indexMap[a.Key.V], indexMap[b.Key.V])
		})
	}

	return merged
}

var trailingWSRE = regexp.MustCompile(`(?m)[ \t]+$`)

func formatTmdoc(in string) string {
	lines := strings.Split(in, "\n")
	// Remove empty last line.
	if len(lines) > 1 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for i, s := range lines {
		lines[i] = "# tmdoc: " + s
	}
	return strings.Join(lines, "\n")
}

func writeBundleInstance(outpath string, content string) error {
	if err := os.MkdirAll(filepath.Dir(outpath), 0755); err != nil {
		return errors.E(err, "failed to create directory")
	}

	f, err := os.Create(outpath)
	if err != nil {
		return errors.E(err, "creating/truncating file")
	}

	defer func() {
		errClose := f.Close()
		if errClose != nil {
			err = errors.L(err, errClose)
		}
	}()

	_, err = f.WriteString(content)
	return err
}

func fixupFileExtension(format, fn string) string {
	switch format {
	case "yaml":
		if strings.HasSuffix(fn, ".hcl") {
			return strings.TrimSuffix(fn, "hcl") + "yml"
		}
		if strings.HasSuffix(fn, ".tm") {
			return fn + ".yml"
		}
	case "hcl":
		if strings.HasSuffix(fn, ".yml") {
			return strings.TrimSuffix(fn, "yml") + "hcl"
		}
		if strings.HasSuffix(fn, ".yaml") {
			return strings.TrimSuffix(fn, "yaml") + "hcl"
		}
		if strings.HasSuffix(fn, ".tm") {
			return fn + ".hcl"
		}
	}
	return fn
}
