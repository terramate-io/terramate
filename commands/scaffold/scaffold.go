// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package scaffold provides the scaffold command.
package scaffold

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/convert"

	hhcl "github.com/terramate-io/hcl/v2"
	"github.com/terramate-io/hcl/v2/hclsyntax"
	"github.com/terramate-io/terramate/commands"
	gencmd "github.com/terramate-io/terramate/commands/generate"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/di"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/generate/resolve"
	"github.com/terramate-io/terramate/globals"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/scaffold/manifest"
	"github.com/terramate-io/terramate/stdlib"
	"github.com/terramate-io/terramate/test/hclwrite"
	"github.com/terramate-io/terramate/test/hclwrite/hclutils"
	"github.com/terramate-io/terramate/typeschema"
	"github.com/terramate-io/terramate/yaml"
)

// Spec is the command specification for the scaffold command.
type Spec struct {
	OutputFormat string
	Generate     bool

	workingDir string
	engine     *engine.Engine
	printers   printer.Printers
}

// Name returns the name of the command.
func (s *Spec) Name() string { return "scaffold" }

// Requirements returns the requirements of the command.
func (s *Spec) Requirements(context.Context, commands.CLI) any { return commands.RequireEngine() }

// Exec executes the scaffold command.
func (s *Spec) Exec(ctx context.Context, cli commands.CLI) error {
	s.workingDir = cli.WorkingDir()
	s.engine = cli.Engine()
	s.printers = cli.Printers()

	resolveAPI, err := di.Get[resolve.API](ctx)
	if err != nil {
		return err
	}

	root := s.engine.Config()

	evalctx := eval.NewContext(stdlib.Functions(root.HostDir(), root.Tree().Node.Experiments()))
	evalctx.SetNamespace("terramate", root.Runtime())

	reg, err := engine.LoadProjectBundles(root, resolveAPI, evalctx, true)
	if err != nil {
		return err
	}

	evalctx = s.setupGlobals(evalctx)

	manifestSources, err := s.lookupPackageSources(evalctx)
	if err != nil {
		return err
	}

	// TODO: We could only do this when the local collection is selected,
	// but to simplify the local, we do it all the time.
	localBundleDefs, err := config.ListLocalBundleDefinitions(root, project.NewPath("/bundles"))
	if err != nil {
		return err
	}

	var localBundleOptions []huh.Option[int]
	for i, defEntry := range localBundleDefs {
		md, err := config.EvalMetadata(root, evalctx, defEntry.Tree, &defEntry.Define.Metadata)
		if err != nil {
			return err
		}

		key := fmt.Sprintf("%s (%s v%s)", md.Name, md.Class, md.Version)
		localBundleOptions = append(localBundleOptions, huh.NewOption(key, i))
	}

	var collections []*manifest.Package
	for _, manifestSrc := range manifestSources {
		c, err := s.loadManifest(manifestSrc, resolveAPI)
		if err != nil {
			return err
		}
		collections = append(collections, c...)
	}

	var collectionOptions []huh.Option[int]
	if len(localBundleOptions) > 0 {
		collectionOptions = append(collectionOptions, huh.NewOption("Local Repository", -1))
	}

	for i, col := range collections {
		collectionOptions = append(collectionOptions, huh.NewOption(col.Name, i))
	}

	if len(collectionOptions) == 0 {
		return fmt.Errorf("no bundles found")
	}

	selectedCollectionIdx := collectionOptions[0].Value
	selectedBundleIdx := 0

	updateBundleDescription := func() string {
		if isLocalCollection(selectedCollectionIdx) {
			return "Bundles from local collection"
		}
		col := collections[selectedCollectionIdx]
		if col.Description != "" {
			return col.Name + "\n" + col.Description
		}
		return col.Name
	}

	updateBundleSelectOptions := func() []huh.Option[int] {
		if isLocalCollection(selectedCollectionIdx) {
			return localBundleOptions
		}

		opts := []huh.Option[int]{}
		col := collections[selectedCollectionIdx]
		for i, bundle := range col.Bundles {
			key := fmt.Sprintf("%s (%s v%s)", bundle.Name, bundle.Class, bundle.Version)
			opts = append(opts, huh.NewOption(key, i))
		}
		return opts
	}

	inputValues := map[string]*cty.Value{}

	var selectedBundle *config.BundleDefinitionEntry
	var outputSource string

	var inputDefs []*config.InputDefinition
	var outputName string
	var outputPath string
	var outputEnv string
	var hostOutputPath string

	confirm := true
	hasAutomaticName := false
	hasAutomaticPath := false
	envRequired := false

	m, err := NewDynamicForm(
		func(state *DynamicFormState) (*huh.Form, error) {
			return huh.NewForm(
				huh.NewGroup(
					huh.NewSelect[int]().
						Value(&selectedCollectionIdx).
						Title("Select Collection").
						Options(collectionOptions...),
				),
				huh.NewGroup(
					huh.NewSelect[int]().
						Value(&selectedBundleIdx).
						Title("Select Bundle").
						DescriptionFunc(updateBundleDescription, &selectedCollectionIdx).
						OptionsFunc(updateBundleSelectOptions, &selectedCollectionIdx).
						Validate(func(idx int) error {
							if isLocalCollection(selectedCollectionIdx) {
								selectedBundle = &localBundleDefs[idx]
								outputSource = selectedBundle.Tree.Dir().String()
							} else {
								col := collections[selectedCollectionIdx]
								colBundle := &col.Bundles[idx]

								outputSource = bundleSourceFromManifest(col, colBundle)

								bundleDir, err := resolveAPI.Resolve(root.HostDir(), outputSource, resolve.Bundle, true)
								if err != nil {
									return err
								}

								selectedBundle, err = config.LoadSingleBundleDefinition(root, bundleDir)
								if err != nil {
									return errors.E(err, "failed to load remote bundle %s", outputSource)
								}
							}

							envRequired, err = checkEnvRequired(evalctx, selectedBundle.Define, reg.Environments)
							if err != nil {
								return err
							}

							if !envRequired {
								evalctx = s.setupBundleContext(evalctx, reg, nil)

								enabled, err := isBundleEnabled(evalctx, selectedBundle.Define)
								if err != nil {
									return err
								}
								if !enabled {
									return fmt.Errorf("bundle is not enabled")
								}
							} else {
								outputEnv = reg.Environments[0].ID
							}
							return nil
						}),
				),
				huh.NewGroup(
					huh.NewSelect[string]().
						Title("Select the target environment").
						Value(&outputEnv).
						OptionsFunc(func() []huh.Option[string] {
							return makeEnvOptions(reg.Environments)
						}, &reg.Environments).
						Validate(func(envID string) error {
							if state.NavigatingBack {
								return nil
							}

							var selectedEnv *config.Environment
							for _, env := range reg.Environments {
								if env.ID == envID {
									selectedEnv = env
									break
								}
							}
							evalctx = s.setupBundleContext(evalctx, reg, selectedEnv)

							enabled, err := isBundleEnabled(evalctx, selectedBundle.Define)
							if err != nil {
								return err
							}
							if !enabled {
								return fmt.Errorf("bundle is not enabled")
							}
							return nil
						}),
				).WithHideFunc(func() bool {
					return !envRequired
				}),
			), nil
		},
		func(_ *DynamicFormState) (*huh.Form, error) {
			schemas, err := config.EvalBundleSchemaNamespaces(root, resolveAPI, evalctx, selectedBundle.Define, true)
			if err != nil {
				return nil, errors.E(err, "failed to evalaute schema namespaces")
			}

			inputDefs, err = config.EvalBundleInputDefinitions(evalctx, selectedBundle.Define, schemas)
			if err != nil {
				return nil, errors.E(err, "failed to evaluate input definitions")
			}

			inctx := inputCtx{evalctx: evalctx, schemas: schemas}

			inputFields := []huh.Field{}
			for _, def := range inputDefs {
				if def.Prompt == "" {
					continue
				}

				var v cty.Value
				inputValues[def.Name] = &v

				field, err := makeInputField(inctx, def, &v)
				if err != nil {
					return nil, err
				}
				inputFields = append(inputFields, field)
			}

			hasAutomaticName = selectedBundle.Define.Scaffolding.Name != nil
			hasAutomaticPath = selectedBundle.Define.Scaffolding.Path != nil

			// If the there are no default values defined, then ask the user.
			// Otherwise, don't ask and use default values.
			if !hasAutomaticName {
				inputFields = append(inputFields,
					huh.NewInput().
						Title("Instance name").
						Description("Name of the created bundle instance.").
						Validate(newStringValidator(true)).
						Value(&outputName),
				)
			}

			if !hasAutomaticPath {
				inputFields = append(inputFields,
					huh.NewInput().
						Title("Output file").
						Description("Path of the created code file.\nPaths starting with / are relative to the project root.\nOtherwise, they are relative to the current directory.").
						Validate(newStringValidator(true)).
						Value(&outputPath),
				)
			}

			inputFields = append(inputFields,
				huh.NewConfirm().
					WithButtonAlignment(lipgloss.Left).
					Title("Please confirm or discard your current changes by choosing one of:").
					Affirmative("Create Bundle").
					Negative("Discard Changes").
					Value(&confirm).
					Validate(func(confirm bool) error {
						if !confirm {
							return nil
						}

						// TODO(snk): Hack
						allInputValues, evalctx, err := makeAllInputs(evalctx, selectedBundle, inputValues, schemas)
						if err != nil {
							return err
						}

						// Eval the bundle definition itself after the inputs have been collected to evaluate default label and path.
						bundleDef, err := config.EvalBundleDefinition(root, evalctx, selectedBundle.Define)
						if err != nil {
							return err
						}

						if hasAutomaticName {
							outputName = bundleDef.ScaffoldingName
						}
						if hasAutomaticPath {
							outputPath = bundleDef.ScaffoldingPath
						}

						if filepath.IsAbs(outputPath) {
							hostOutputPath = project.NewPath(outputPath).HostPath(root.HostDir())
						} else {
							hostOutputPath = filepath.Join(s.workingDir, outputPath)
						}

						hostOutputPath = fixupFileExtension(s.OutputFormat, hostOutputPath)

						var content string
						if s.OutputFormat == "yaml" {
							content, err = generateBundleYAML(outputName, outputSource, outputEnv, inputDefs, allInputValues)
							if err != nil {
								return err
							}
						} else {
							content = generateBundleHCL(outputName, outputSource, inputValues)
						}

						err = createBundleInstance(hostOutputPath, content)
						if err != nil {
							return err
						}

						return nil
					}),
			)

			return huh.NewForm(huh.NewGroup(inputFields...)).WithKeyMap(defaultKeymap), nil
		},
	)
	if err != nil {
		return err
	}

	if m, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		return err
	} else if err := m.(DynamicForm).Error(); err != nil {
		return err
	}

	if !confirm {
		s.printers.Stdout.Println("Cancelled by user...")
		return nil
	}

	s.printers.Stdout.Printf("Created bundle at %s\n", hostOutputPath)

	if s.Generate {
		if err := s.runGenerate(ctx, cli); err != nil {
			return err
		}
	}

	return nil
}

func (s *Spec) runGenerate(ctx context.Context, cli commands.CLI) error {
	if err := cli.Reload(ctx); err != nil {
		return err
	}

	generate := gencmd.Spec{
		MinimalReport: true,
		PrintReport:   true,
	}
	return generate.Exec(ctx, cli)
}

// TODO(snk): This is a temporary hack.
func makeAllInputs(evalctx *eval.Context, selectedBundle *config.BundleDefinitionEntry, inputMap map[string]*cty.Value, schemas typeschema.SchemaNamespaces) (map[string]cty.Value, *eval.Context, error) {
	inst := &hcl.Bundle{
		Inputs: ast.NewMergedBlock("inputs", []string{}),
	}
	for k, v := range inputMap {
		if v == nil || *v == cty.NilVal {
			continue
		}
		r := hhcl.Range{
			Filename: selectedBundle.Tree.HostDir(),
		}

		inst.Inputs.Attributes[k] = ast.NewAttribute(selectedBundle.Tree.HostDir(),
			&hhcl.Attribute{
				Name:  k,
				Expr:  &hclsyntax.LiteralValueExpr{Val: *v},
				Range: r,
			})
	}

	evalctx = evalctx.ChildContext()

	// These results still have the .value indirection, so we need to unwrap that again.
	tempInputs, err := config.EvalInputs(
		evalctx,
		"bundle",
		inst.Info,
		inst.Inputs,
		inst.InputsAttr,
		selectedBundle.Define.Inputs,
		schemas)
	if err != nil {
		return nil, nil, err
	}

	result := make(map[string]cty.Value, len(inputMap))
	for k, v := range inputMap {
		if v != nil && *v != cty.NilVal {
			result[k] = *v
		}
	}

	for k, v := range tempInputs {
		// Skip those we already have. EvalInputs will return values for everything, including what we just prompted.
		// Values should be the same though.
		if _, exists := result[k]; exists {
			continue
		}
		vm := v.AsValueMap()
		result[k] = vm["value"]
	}
	return result, evalctx, nil
}

// TODO: Use a different method to write the block.
func generateBundleHCL(label, source string, inputMap map[string]*cty.Value) string {
	inputAttrs := []hclwrite.BlockBuilder{}

	for k, v := range inputMap {
		if v.IsNull() {
			continue
		}
		s, _ := valueToHCLExpr(v)
		inputAttrs = append(inputAttrs, hclutils.Expr(k, s))
	}

	return hclutils.Doc(
		hclutils.Block("bundle", hclutils.Labels(label),
			hclutils.Str("source", source),
			hclutils.Str("uuid", uuid.NewString()),
			hclutils.Block("inputs", inputAttrs...),
		),
	).String()
}

func generateBundleYAML(label, source, env string, inputDefs []*config.InputDefinition, inputMap map[string]cty.Value) (string, error) {
	inputs := yaml.Map[any]{}
	for _, def := range inputDefs {
		v, found := inputMap[def.Name]
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

	var bundle yaml.BundleInstance
	if env != "" {
		bundle = yaml.BundleInstance{
			Name:   yaml.Attr(label),
			UUID:   yaml.Attr(uuid.NewString()),
			Source: yaml.Attr[any](source),
			Inputs: yaml.Attr(inputs),
			Environments: yaml.Attr(
				yaml.Map[*yaml.BundleEnvironment]{
					{Key: yaml.Attr(env), Value: yaml.Attr(&yaml.BundleEnvironment{})},
				},
			),
		}
	} else {
		bundle = yaml.BundleInstance{
			Name:   yaml.Attr(label),
			UUID:   yaml.Attr(uuid.NewString()),
			Source: yaml.Attr[any](source),
			Inputs: yaml.Attr(inputs),
		}
	}

	var b strings.Builder
	err := yaml.Encode(&bundle, &b)
	if err != nil {
		return "", err
	}
	// Strip trailing whitespace produced by the YAML encoder when rendering HeadComment
	// with a leading newline for visual separation.
	output := trailingWSRE.ReplaceAllString(b.String(), "")
	return output, nil
}

var trailingWSRE = regexp.MustCompile(`(?m)[ \t]+$`)

func formatTmdoc(in string) string {
	var out string
	lines := strings.Split(in, "\n")
	for i, line := range lines {
		if len(lines) > 1 && i == len(lines)-1 && line == "" {
			// Skip empty last line.
			continue
		}
		out += "tmdoc: " + line + "\n"
	}
	return "\n# " + out
}

// TODO: Use a different method to write the block.
func createBundleInstance(outpath string, content string) error {
	_, err := os.Stat(outpath)
	if err == nil {
		// Even if there is no stack block inside the file, we can't overwrite
		// the user file anyway.
		return errors.E("a bundle with the same filename already exists: %s", outpath)
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

type stringAccessor struct {
	v *cty.Value
}

func (i *stringAccessor) Get() string {
	if i.v.Type() == cty.String {
		return i.v.AsString()
	}
	return ""
}

func (i *stringAccessor) Set(s string) {
	s = strings.TrimSpace(s)

	if s != "" {
		*i.v = cty.StringVal(s)
	} else {
		*i.v = cty.NilVal
	}
}

type boolAccessor struct {
	v *cty.Value
}

func (i *boolAccessor) Get() bool {
	if i.v.Type() == cty.Bool {
		return i.v.True()
	}
	return false
}

func (i *boolAccessor) Set(s bool) {
	*i.v = cty.BoolVal(s)
}

type numberAccessor struct {
	v *cty.Value
}

func (i *numberAccessor) Get() string {
	if i.v.Type() == cty.Number {
		bf := i.v.AsBigFloat()
		return bf.Text('f', -1)
	}
	return ""
}

func (i *numberAccessor) Set(s string) {
	s = strings.TrimSpace(s)

	if s != "" {
		*i.v, _ = cty.ParseNumberVal(s)
	} else {
		*i.v = cty.NilVal
	}
}

type stringListAccessor struct {
	v *cty.Value
}

func (i *stringListAccessor) Get() string {
	if i.v.Type() != cty.List(cty.String) {
		return ""
	}

	var b strings.Builder

	it := i.v.ElementIterator()
	for it.Next() {
		_, elem := it.Element()
		_, _ = b.WriteString(elem.AsString() + "\n")
	}
	return b.String()
}

func (i *stringListAccessor) Set(s string) {
	if s == "" {
		*i.v = cty.NilVal
		return
	}

	parts := strings.Split(s, "\n")
	for i, e := range parts {
		parts[i] = strings.TrimSpace(e)
	}
	var vals []cty.Value
	for _, e := range parts {
		if e != "" {
			vals = append(vals, cty.StringVal(e))
		}
	}
	if len(vals) == 0 {
		*i.v = cty.NilVal
		return
	}
	*i.v = cty.ListVal(vals)
}

type numberListAccessor struct {
	v *cty.Value
}

func (i *numberListAccessor) Get() string {
	if i.v.Type() != cty.List(cty.Number) {
		return ""
	}

	var b strings.Builder

	it := i.v.ElementIterator()
	for it.Next() {
		_, elem := it.Element()
		bf := elem.AsBigFloat()
		s := bf.Text('f', -1)
		_, _ = b.WriteString(s + "\n")
	}
	return b.String()
}

func (i *numberListAccessor) Set(s string) {
	*i.v, _ = parseNumberList(s, false)
}

type hclAccessor struct {
	inctx inputCtx
	v     *cty.Value
}

func (i *hclAccessor) Get() string {
	if !i.v.IsNull() {
		v, _ := valueToHCLExpr(i.v)
		return v
	}
	return ""
}

func (i *hclAccessor) Set(s string) {
	if s == "" {
		*i.v = cty.NilVal
		return
	}
	parsed, err := ast.ParseExpression(s, "input")
	if err == nil {
		newv, err := i.inctx.evalctx.Eval(parsed)
		if err == nil {
			*i.v = newv
		}
	}
}

type multiSelectAccessor struct {
	v *cty.Value
}

func (i *multiSelectAccessor) Get() []cty.Value {
	if i.v.CanIterateElements() {
		return i.v.AsValueSlice()
	}
	return []cty.Value{}
}

func (i *multiSelectAccessor) Set(vals []cty.Value) {
	*i.v = cty.TupleVal(vals)
}

func makeDefaultKeymap() *huh.KeyMap {
	km := huh.NewDefaultKeyMap()
	km.Text = huh.TextKeyMap{
		Prev:    key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "back")),
		Next:    key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next")),
		Submit:  key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "submit")),
		NewLine: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "new line")),
		Editor:  key.NewBinding(key.WithKeys("ctrl+e"), key.WithHelp("ctrl+e", "open editor")),
	}
	return km
}

var defaultKeymap = makeDefaultKeymap()

func makeInputField(inctx inputCtx, def *config.InputDefinition, v *cty.Value) (huh.Field, error) {
	if def.Options != nil {
		if def.Multiselect {
			return makeMultiSelectInputField(def, v)
		}
		return makeSelectInputField(def, v)
	}
	switch def.Type.String() {
	case "bool":
		return makeBoolInputField(def, v)
	case "string":
		return makeStringInputField(def, v)
	case "number":
		return makeNumberInputField(def, v)
	case "list(string)":
		return makeStringListInputField(def, v)
	case "list(number)":
		return makeNumberListInputField(def, v)
	default:
		return makeHCLInputField(inctx, def, v)
	}
}

func makeSelectInputField(def *config.InputDefinition, v *cty.Value) (huh.Field, error) {
	bundleOptions := []huh.Option[cty.Value]{}
	defaultIndex := -1

	// Move the default to the top if found.
	if def.PromptDefault != cty.NilVal && !def.PromptDefault.IsNull() {
		for idx, e := range def.Options {
			if c := def.PromptDefault.Equals(e.Value); c.Type() == cty.Bool && c.True() {
				bundleOptions = append(bundleOptions, huh.NewOption(e.Name, e.Value))
				defaultIndex = idx
				break
			}
		}
	}

	for idx, e := range def.Options {
		if idx != defaultIndex {
			bundleOptions = append(bundleOptions, huh.NewOption(e.Name, e.Value))
		}
	}

	return huh.NewSelect[cty.Value]().
		Value(v).
		Title(def.Prompt).
		Description(def.Description).
		Options(bundleOptions...), nil
}

func makeEnvOptions(envs []*config.Environment) []huh.Option[string] {
	envOptions := []huh.Option[string]{}
	for _, env := range envs {
		var s string
		if env.Name != "" {
			s = fmt.Sprintf("%s (%s)", env.Name, env.ID)
		} else {
			s = env.ID
		}
		envOptions = append(envOptions, huh.NewOption(s, env.ID))
	}
	return envOptions
}

func makeMultiSelectInputField(def *config.InputDefinition, v *cty.Value) (huh.Field, error) {
	bundleOptions := []huh.Option[cty.Value]{}
	defaultIndex := -1

	// Move the default to the top if found.
	if def.PromptDefault != cty.NilVal && !def.PromptDefault.IsNull() {
		for idx, e := range def.Options {
			if c := def.PromptDefault.Equals(e.Value); c.Type() == cty.Bool && c.True() {
				bundleOptions = append(bundleOptions, huh.NewOption(e.Name, e.Value))
				defaultIndex = idx
				break
			}
		}
	}

	for idx, e := range def.Options {
		if idx != defaultIndex {
			bundleOptions = append(bundleOptions, huh.NewOption(e.Name, e.Value))
		}
	}

	return huh.NewMultiSelect[cty.Value]().
		Accessor(&multiSelectAccessor{v: v}).
		Title(def.Prompt).
		Description(def.Description).
		Options(bundleOptions...), nil
}

func makeBoolInputField(def *config.InputDefinition, v *cty.Value) (huh.Field, error) {
	if def.PromptDefault.Type() == cty.Bool {
		*v = def.PromptDefault
	} else {
		*v = cty.BoolVal(false)
	}
	return huh.NewConfirm().
		WithButtonAlignment(lipgloss.Left).
		Accessor(&boolAccessor{v: v}).
		Affirmative("Yes").
		Negative("No").
		Description(def.Description).
		Title(def.Prompt), nil
}

func makeStringInputField(def *config.InputDefinition, v *cty.Value) (huh.Field, error) {
	isRequired := def.PromptDefault == cty.NilVal
	if def.Multiline {
		return huh.NewText().
			Title(def.Prompt).
			Description(def.Description).
			Accessor(&stringAccessor{v: v}).
			ExternalEditor(false).
			Placeholder(makePlaceholder(def.PromptDefault)).
			Validate(newStringValidator(isRequired)), nil
	}
	return huh.NewInput().
		Title(def.Prompt).
		Description(def.Description).
		Accessor(&stringAccessor{v: v}).
		Placeholder(makePlaceholder(def.PromptDefault)).
		Validate(newStringValidator(isRequired)), nil
}

func makeNumberInputField(def *config.InputDefinition, v *cty.Value) (huh.Field, error) {
	isRequired := def.PromptDefault == cty.NilVal
	return huh.NewInput().
		Title(def.Prompt).
		Description(def.Description).
		Accessor(&numberAccessor{v: v}).
		Placeholder(makePlaceholder(def.PromptDefault)).
		Validate(newNumberValidator(isRequired)), nil
}

func makeStringListInputField(def *config.InputDefinition, v *cty.Value) (huh.Field, error) {
	isRequired := def.PromptDefault == cty.NilVal
	return huh.NewText().
		Title(def.Prompt).
		Description(def.Description).
		Accessor(&stringListAccessor{v: v}).
		ExternalEditor(false).
		Placeholder(makePlaceholder(def.PromptDefault)).
		Validate(newStringListValidator(isRequired)), nil
}

func makeNumberListInputField(def *config.InputDefinition, v *cty.Value) (huh.Field, error) {
	isRequired := def.PromptDefault == cty.NilVal
	return huh.NewText().
		Title(def.Prompt).
		Description(def.Description).
		Accessor(&numberListAccessor{v: v}).
		ExternalEditor(false).
		Placeholder(makePlaceholder(def.PromptDefault)).
		Validate(newNumberListValidator(isRequired)), nil
}

func makePlaceholder(promptDefault cty.Value) string {
	if promptDefault == cty.NilVal {
		return ""
	}

	// Already a string. Either because its a string, or it failed to evaluate and was turned into tokens.
	if promptDefault.Type() == cty.String {
		return fmt.Sprintf("[Default: %s]", promptDefault.AsString())
	}

	// Can be converted to a string, lets use it.
	convd, err := convert.Convert(promptDefault, cty.String)
	if err == nil {
		return fmt.Sprintf("[Default: %s]", convd.AsString())

	}

	// Show the tokens as fallback.
	tokens := ast.TokensForValue(promptDefault)
	return fmt.Sprintf("[Default: %s]", string(tokens.Bytes()))
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

type inputCtx struct {
	evalctx *eval.Context
	schemas typeschema.SchemaNamespaces
}

func makeHCLInputField(inctx inputCtx, def *config.InputDefinition, v *cty.Value) (huh.Field, error) {
	return huh.NewText().
		Title(def.Prompt).
		Placeholder(makePlaceholder(def.PromptDefault)).
		Accessor(&hclAccessor{inctx: inctx, v: v}).
		Description(def.Description).
		Validate(newHCLValidator(inctx, def.Type)), nil
}

func valueToHCLExpr(v *cty.Value) (string, error) {
	if v == nil {
		return "", errors.E("value was nil")
	}
	tokens := ast.TokensForValue(*v)
	return string(tokens.Bytes()), nil
}

func newHCLValidator(inctx inputCtx, typ typeschema.Type) func(string) error {
	return func(s string) error {
		if s == "" {
			return nil
		}
		parsed, err := ast.ParseExpression(s, "input")
		if err != nil {
			return err
		}
		v, err := inctx.evalctx.Eval(parsed)
		if err != nil {
			return err
		}

		_, err = typ.Apply(v, inctx.evalctx, inctx.schemas, true)
		return err
	}
}

func newStringValidator(required bool) func(string) error {
	return func(s string) error {
		if required && strings.TrimSpace(s) == "" {
			return errors.E("this value is required")
		}
		return nil
	}
}

func newNumberValidator(required bool) func(string) error {
	return func(s string) error {
		s = strings.TrimSpace(s)
		if s == "" {
			if required {
				return errors.E("this value is required")
			}
			return nil
		}

		_, err := cty.ParseNumberVal(s)
		if err != nil {
			return errors.E("value must be a number")
		}
		return nil
	}
}

func newStringListValidator(required bool) func(string) error {
	return newStringValidator(required)
}

func newNumberListValidator(required bool) func(string) error {
	return func(s string) error {
		_, err := parseNumberList(s, required)
		return err
	}
}

func parseNumberList(s string, required bool) (cty.Value, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		if required {
			return cty.NilVal, errors.E("this value is required")
		}
		return cty.NilVal, nil
	}

	parts := strings.Split(s, "\n")
	for i, e := range parts {
		parts[i] = strings.TrimSpace(e)
		if parts[i] == "" {
			return cty.NilVal, errors.E("list may not contain empty lines")
		}
	}

	var vals []cty.Value
	for _, e := range parts {
		v, err := cty.ParseNumberVal(e)
		if err != nil {
			return cty.NilVal, errors.E("value %s is not a number", e)
		}
		vals = append(vals, v)
	}
	return cty.ListVal(vals), nil
}

func (s *Spec) lookupPackageSources(evalctx *eval.Context) ([]string, error) {
	cfg := s.engine.RootNode()
	if cfg.Scaffold != nil {
		scaffold, err := config.EvalScaffold(evalctx, cfg.Scaffold)
		if err != nil {
			return nil, err
		}
		return scaffold.PackageSources, nil
	}
	return nil, nil
}

func (s *Spec) loadManifest(manifestSrc string, resolveAPI resolve.API) ([]*manifest.Package, error) {
	cfg := s.engine.Config()
	rootdir := cfg.HostDir()

	var manifestPath string
	isLocalSource := strings.HasPrefix(manifestSrc, "/")

	// File in the current repo.
	if isLocalSource {
		// Must specify the exact file.
		manifestPath = filepath.Join(rootdir, manifestSrc)
	} else {
		// First resolve the source to a dir, then append the default filename.
		manifestProjPath, err := resolveAPI.Resolve(rootdir, manifestSrc, resolve.Manifest, true)
		if err != nil {
			return nil, err
		}
		manifestPath = project.AbsPath(rootdir, manifestProjPath.String())
	}

	pkgs, err := manifest.LoadFile(manifestPath)
	if err != nil {
		return nil, err
	}

	// Fall back to remote source if location is not set.
	for _, p := range pkgs {
		if p.Location == "" && !isLocalSource {
			p.Location = manifestSrc
		}
	}

	return pkgs, nil
}

func bundleSourceFromManifest(pkg *manifest.Package, bundle *manifest.Bundle) string {
	addr, params, found := strings.Cut(pkg.Location, "?")
	if found {
		return fmt.Sprintf("%s//%s?%s", addr, bundle.Path, params)
	}
	return fmt.Sprintf("%s//%s", addr, bundle.Path)
}

func (s *Spec) setupGlobals(evalctx *eval.Context) *eval.Context {
	root := s.engine.Config()
	// Add globals from root to the context.
	// This is a best effort, there might be undefined stack. variables, so we ignore any errors.
	// Expressions that are evaluatable will still be set.
	_ = globals.ForDir(root, project.NewPath("/"), evalctx)
	return evalctx
}

func (s *Spec) setupBundleContext(evalctx *eval.Context, reg *config.Registry, env *config.Environment) *eval.Context {
	var bundleVals map[string]cty.Value
	if bundleNS, ok := evalctx.GetNamespace("bundle"); ok {
		bundleVals = bundleNS.AsValueMap()
	} else {
		bundleVals = map[string]cty.Value{}
	}
	bundleVals["environment"] = config.MakeEnvObject(env)
	evalctx.SetNamespace("bundle", bundleVals)

	evalctx.SetFunction(stdlib.Name("bundle"), config.BundleFunc(context.TODO(), reg, env, false))
	evalctx.SetFunction(stdlib.Name("bundles"), config.BundlesFunc(reg, env))
	return evalctx
}

func isLocalCollection(idx int) bool {
	return idx == -1
}

func isBundleEnabled(evalctx *eval.Context, def *hcl.DefineBundle) (bool, error) {
	for _, cond := range def.Scaffolding.Enabled {
		enabled, err := config.EvalBool(evalctx, cond.Condition.Expr, "scaffolding.enabled.condition")
		if err != nil {
			return false, errors.E(err, cond.Condition.Range)
		}
		errorMsg, err := config.EvalString(evalctx, cond.ErrorMessage.Expr, "scaffolding.enabled.error_message")
		if err != nil {
			return false, errors.E(err, cond.ErrorMessage.Range)
		}
		if !enabled {
			return false, errors.E(errorMsg)
		}
	}
	return true, nil
}

func checkEnvRequired(evalctx *eval.Context, def *hcl.DefineBundle, envs []*config.Environment) (bool, error) {
	envRequired := false
	if def.Environments.Required != nil {
		var err error
		envRequired, err = config.EvalBool(evalctx, def.Environments.Required.Expr, "environments.required")
		if err != nil {
			return false, errors.E(err, def.Environments.Required.Range)
		}
	}
	if envRequired && len(envs) == 0 {
		return false, errors.E("this bundle requires environments, but none are configured")
	}
	return envRequired, nil
}
