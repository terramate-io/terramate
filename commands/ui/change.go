// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package ui

import (
	"cmp"
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

	"github.com/terramate-io/terramate/commands"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/typeschema"
	"github.com/terramate-io/terramate/yaml"
)

// ChangeKind indicates the type of pending change.
type ChangeKind string

// ChangeCreate and the following constants enumerate the supported change kinds.
const (
	ChangeCreate   ChangeKind = "change_create"
	ChangeReconfig ChangeKind = "change_reconfig"
	ChangePromote  ChangeKind = "change_promote"
)

// Change represents a pending change in the summary.
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

	// Simple incremental ID returned to the LLM so it can track this change reliably.
	ProposalID int

	// Set to exclude from uniqueness checks, so an edited change doesn't conflict with itself.
	MarkedForReplacement bool
}

// SavedChange is a lightweight summary of a change that was persisted to disk.
type SavedChange struct {
	Kind        ChangeKind
	Name        string
	EnvID       string
	FromEnvID   string
	ProjectPath string
	HostPath    string
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
	hostPath = commands.FixupFileExtension("yaml", hostPath)

	// If there is no explicit alias, we fallback to the <path>:<name> default.
	// But we must do this only after these values are known.
	// If there was an explicit alias, it can actually be used in path and name.
	// Sounds circular but it isn't...
	if alias == "" {
		alias = fmt.Sprintf("%s:%s", outputPath, outputName)
	}

	var env *config.Environment
	if activeEnv != nil && bundleRequiresEnv(schemactx.Evalctx, bde.Define) {
		env = activeEnv
	}

	// Final check: Is the bundle unique?
	if err := est.Registry.IsBundleUnique(alias, bde.Metadata.Class, hostPath); err != nil {
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

// NewChangeFromExisting rebuilds a Change from a previously created change with updated values.
func NewChangeFromExisting(
	est *EngineState,
	oldChange Change,
	schemactx typeschema.EvalContext,
	inputDefs []*config.InputDefinition,
	values map[string]cty.Value,
) (Change, error) {
	switch oldChange.Kind {
	case ChangeCreate:
		return NewCreateChange(
			est,
			oldChange.Env,
			oldChange.BundleDefEntry,
			schemactx,
			inputDefs,
			values,
		)
	case ChangeReconfig:
		return NewReconfigChange(
			est,
			oldChange.OriginalBundle,
			oldChange.BundleDefEntry,
			schemactx,
			inputDefs,
			values,
		)
	case ChangePromote:
		return NewPromoteChange(
			est,
			oldChange.Env,
			oldChange.OriginalBundle,
			oldChange.BundleDefEntry,
			schemactx,
			inputDefs,
			values,
		)
	default:
		panic("unsupported ChangeKind " + oldChange.Kind)
	}
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

// Save writes the change to disk as a YAML bundle instance file.
func (c *Change) Save(envs []*config.Environment) error {
	var existing *yaml.BundleInstance
	overwrite := c.Kind == ChangeReconfig || c.Kind == ChangePromote

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
	return writeBundleInstance(c.HostPath, content, overwrite)
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

func writeBundleInstance(outpath string, content string, overwrite bool) (err error) {
	if !overwrite {
		_, err := os.Stat(outpath)
		if err == nil {
			return errors.E("a bundle with the same filename already exists: %s", outpath)
		}
	}

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
