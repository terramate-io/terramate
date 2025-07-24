// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package create provides the create stack command.
package create

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/cloud/api/resources"
	cloudstack "github.com/terramate-io/terramate/cloud/api/stack"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/stack"
	"github.com/terramate-io/terramate/tf"
	"github.com/terramate-io/terramate/tg"

	gencmd "github.com/terramate-io/terramate/commands/generate"
)

// Spec represents the create stack specification.
type Spec struct {
	Engine     *engine.Engine
	WorkingDir string
	Path       string

	AllTerraform   bool
	AllTerragrunt  bool
	EnsureStackIDs bool

	IgnoreExisting bool
	NoGenerate     bool
	Verbosity      int

	Imports []string

	StackID          string
	StackName        string
	StackDescription string
	StackTags        []string
	StackWatch       []string
	StackAfter       []string
	StackBefore      []string
	StackWants       []string
	StackWantedBy    []string

	Printers printer.Printers
}

// Name returns the name of the create stack command.
func (s *Spec) Name() string { return "create" }

// Exec executes the create stack command.
func (s *Spec) Exec(_ context.Context) error {
	scanFlags := 0
	if s.AllTerraform {
		scanFlags++
	}
	if s.AllTerragrunt {
		scanFlags++
	}
	if s.EnsureStackIDs {
		scanFlags++
	}

	if scanFlags == 0 && s.Path == "" {
		return errors.E("Missing args: path argument or one of --all-terraform, --all-terragrunt, --ensure-stack-ids must be provided")
	}

	if scanFlags > 1 {
		return errors.E("Invalid args: only one of --all-terraform, --all-terragrunt, --ensure-stack-ids can be provided")
	}

	if scanFlags == 1 {
		if s.Path != "" {
			return errors.E("Invalid args: path argument cannot be provided with --all-terraform, --all-terragrunt, --ensure-stack-ids")
		}
		return s.execScanCreate()
	}

	if s.Path == "" {
		return errors.E("Missing args: path argument or one of --all-terraform, --all-terragrunt, --ensure-stack-ids must be provided")
	}

	stackHostDir := filepath.Join(s.WorkingDir, s.Path)

	stackID := s.StackID
	if s.StackID == "" {
		id, err := uuid.NewRandom()
		if err != nil {
			return errors.E(err, "creating stack UUID")
		}
		stackID = id.String()
	}

	stackName := s.StackName
	if stackName == "" {
		stackName = filepath.Base(stackHostDir)
	}

	stackDescription := s.StackDescription
	if stackDescription == "" {
		stackDescription = stackName
	}

	var tags []string
	for _, tag := range s.StackTags {
		tags = append(tags, strings.Split(tag, ",")...)
	}

	rootdir := s.Engine.Config().HostDir()
	watch, err := config.ValidateWatchPaths(rootdir, stackHostDir, s.StackWatch)
	if err != nil {
		return errors.E(err, "invalid --watch argument value")
	}

	stackSpec := config.Stack{
		Dir:         project.PrjAbsPath(rootdir, stackHostDir),
		ID:          stackID,
		Name:        stackName,
		Description: stackDescription,
		After:       s.StackAfter,
		Before:      s.StackBefore,
		Wants:       s.StackWants,
		WantedBy:    s.StackWantedBy,
		Watch:       watch,
		Tags:        tags,
	}

	err = stack.Create(s.Engine.Config(), stackSpec, s.Imports...)
	if err != nil {
		logger := log.With().
			Stringer("stack", stackSpec.Dir).
			Logger()

		if s.IgnoreExisting &&
			(errors.IsKind(err, stack.ErrStackAlreadyExists) ||
				errors.IsKind(err, stack.ErrStackDefaultCfgFound)) {
			logger.Debug().Msg("stack already exists, ignoring")
			return nil
		}

		if errors.IsKind(err, stack.ErrStackDefaultCfgFound) {
			logger = logger.With().
				Str("file", stack.DefaultFilename).
				Logger()
		}

		return errors.E(err, "Cannot create stack")
	}

	s.Printers.Stdout.Success("Created stack " + stackSpec.Dir.String())

	if s.NoGenerate {
		log.Debug().Msg("code generation on stack creation disabled")
		return nil
	}

	err = s.Engine.Config().LoadSubTree(stackSpec.Dir)
	if err != nil {
		return errors.E(err, "Unable to load new stack")
	}

	generate := gencmd.Spec{
		Engine:        s.Engine,
		WorkingDir:    s.WorkingDir,
		MinimalReport: true,
		PrintReport:   s.Verbosity > 0,
		Printers:      s.Printers,
	}

	return generate.Exec(context.TODO())
}

func (s *Spec) execScanCreate() error {
	var flagname string
	switch {
	case s.EnsureStackIDs:
		flagname = "--ensure-stack-ids"
	case s.AllTerraform:
		flagname = "--all-terraform"
	case s.AllTerragrunt:
		flagname = "--all-terragrunt"
	default:
		panic(errors.E(errors.ErrInternal, "bug: no flag set"))
	}

	if s.StackID != "" ||
		s.StackName != "" ||
		s.Path != "" ||
		s.StackDescription != "" ||
		s.IgnoreExisting ||
		len(s.StackAfter) != 0 ||
		len(s.StackBefore) != 0 ||
		len(s.StackWants) != 0 ||
		len(s.StackWantedBy) != 0 ||
		len(s.StackWatch) != 0 ||
		len(s.Imports) != 0 {

		return errors.E(
			"Invalid args: %s is incompatible with path and the flags: "+
				"--id,"+
				" --name, "+
				"--description, "+
				"--after, "+
				"--before, "+
				"--watch, "+
				"--import, "+
				" --ignore-existing",
			flagname,
		)
	}

	switch flagname {
	case "--all-terraform":
		return s.initTerraform()
	case "--all-terragrunt":
		return s.initTerragrunt()
	case "--ensure-stack-ids":
		return s.ensureStackID()
	}
	panic("unexpected case")
}

func (s *Spec) initTerragrunt() error {
	rootdir := s.Engine.Config().HostDir()
	modules, err := tg.ScanModules(rootdir, project.PrjAbsPath(rootdir, s.WorkingDir), true)
	if err != nil {
		return errors.E(err, "scanning for Terragrunt modules")
	}
	errs := errors.L()
	for _, mod := range modules {
		tree, found := s.Engine.Config().Lookup(mod.Path)
		if found && tree.IsStack() {
			continue
		}

		stackID, err := uuid.NewRandom()
		dirBasename := filepath.Base(mod.Path.String())
		if err != nil {
			return errors.E(err, "creating stack UUID")
		}

		after := []string{}
		for _, otherMod := range mod.After.Strings() {
			// Parent stack modules must be excluded because of implicit filesystem ordering.
			// Parent stacks are always executed before child stacks.
			if otherMod == "/" || mod.Path.HasPrefix(otherMod+"/") {
				continue
			}
			// after stacks must not be defined as child stacks
			// because it contradicts the Terramate implicit filesystem ordering.
			if strings.HasPrefix(otherMod, mod.Path.String()+"/") {
				return errors.E(
					errors.E("Module %q is defined as a child of the module stack it depends on, which contradicts the Terramate implicit filesystem ordering.", otherMod),
					"You may consider moving stack %s elsewhere not conflicting with filesystem ordering.", otherMod,
				)
			}
			// Terragrunt will allow repeated dependencies, stack.Validate() doesnt.
			if !slices.Contains(after, otherMod) {
				after = append(after, otherMod)
			}
		}

		var tags []string
		for _, tag := range s.StackTags {
			tags = append(tags, strings.Split(tag, ",")...)
		}

		stackSpec := config.Stack{
			Dir:         mod.Path,
			ID:          stackID.String(),
			Name:        dirBasename,
			Description: dirBasename,
			Tags:        tags,
			After:       after,
		}

		err = stack.Create(s.Engine.Config(), stackSpec)
		if err != nil {
			errs.Append(err)
			continue
		}

		s.Printers.Stdout.Println(fmt.Sprintf("Created stack %s", stackSpec.Dir))
	}

	if err := errs.AsError(); err != nil {
		return errors.E(err, "failed to initialize Terragrunt modules")
	}
	return nil
}

func (s *Spec) initTerraform() error {
	err := s.initTerraformDir(s.WorkingDir)
	if err != nil {
		return errors.E(err, "failed to initialize some directories")
	}

	if s.NoGenerate {
		log.Debug().Msg("code generation on stack creation disabled")
		return nil
	}

	err = s.Engine.ReloadConfig()
	if err != nil {
		return errors.E(err, "reloading the configuration")
	}

	generate := gencmd.Spec{
		Engine:        s.Engine,
		WorkingDir:    s.WorkingDir,
		Printers:      s.Printers,
		MinimalReport: true,
	}

	return generate.Exec(context.Background())
}

func (s *Spec) initTerraformDir(baseDir string) error {
	rootdir := s.Engine.Config().HostDir()
	pdir := project.PrjAbsPath(rootdir, baseDir)
	var isStack bool
	tree, found := s.Engine.Config().Lookup(pdir)
	if found {
		isStack = tree.IsStack()
	}

	dirs, err := os.ReadDir(baseDir)
	if err != nil {
		return errors.E(err, "unable to read directory while listing directory entries")
	}

	var tags []string
	for _, tag := range s.StackTags {
		tags = append(tags, strings.Split(tag, ",")...)
	}

	errs := errors.L()
	for _, f := range dirs {
		path := filepath.Join(baseDir, f.Name())
		if strings.HasPrefix(f.Name(), ".") {
			continue
		}

		if f.IsDir() {
			errs.Append(s.initTerraformDir(path))
			continue
		}

		if isStack {
			continue
		}

		if filepath.Ext(f.Name()) != ".tf" {
			continue
		}

		found, err := tf.IsStack(path)
		if err != nil {
			return errors.E(err, "parsing terraform")
		}

		if !found {
			continue
		}

		stackDir := baseDir
		stackID, err := uuid.NewRandom()
		dirBasename := filepath.Base(stackDir)
		if err != nil {
			return errors.E(err, "creating stack UUID")
		}
		stackSpec := config.Stack{
			Dir:         project.PrjAbsPath(rootdir, stackDir),
			ID:          stackID.String(),
			Name:        dirBasename,
			Description: dirBasename,
			Tags:        tags,
		}

		err = stack.Create(s.Engine.Config(), stackSpec)
		if err != nil {
			errs.Append(err)
			continue
		}

		s.Printers.Stdout.Println(fmt.Sprintf("Created stack %s", stackSpec.Dir))

		// so other files in the same directory do not trigger stack creation.
		isStack = true
	}
	return errs.AsError()
}

func (s *Spec) ensureStackID() error {
	report, err := s.Engine.ListStacks(engine.NoGitFilter(), cloudstack.AnyTarget, resources.NoStatusFilters(), false)
	if err != nil {
		return errors.E(err, "listing stacks")
	}

	cfg := s.Engine.Config()
	for _, entry := range report.Stacks {
		if entry.Stack.ID != "" {
			continue
		}

		id, err := stack.UpdateStackID(cfg, entry.Stack.HostDir(cfg))
		if err != nil {
			return errors.E(err, "failed to update stack.id of stack %s", entry.Stack.Dir)
		}

		s.Printers.Stdout.Println(fmt.Sprintf("Generated ID %s for stack %s", id, entry.Stack.Dir))
	}
	return nil
}
