// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package trigger provides the trigger command.
package trigger

import (
	"context"
	"fmt"

	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/terramate-io/terramate/cloud/api/resources"
	cloudstack "github.com/terramate-io/terramate/cloud/api/stack"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/stack/trigger"
)

const defaultTriggerReason = "Created using Terramate CLI without setting specific reason."

// PathSpec represents the trigger specification for when you trigger an specific path.
type PathSpec struct {
	WorkingDir   string
	Printers     printer.Printers
	Engine       *engine.Engine
	Path         string
	Change       bool
	IgnoreChange bool
	Tags         []string
	NoTags       []string
	Reason       string
	Recursive    bool
}

// Name returns the name of the filter.
func (s *PathSpec) Name() string { return "trigger" }

// Exec executes the trigger command.
func (s *PathSpec) Exec(_ context.Context) error {
	changeFlag := s.Change
	ignoreFlag := s.IgnoreChange

	if changeFlag && ignoreFlag {
		return errors.E("flags --change and --ignore-change are conflicting")
	}

	var (
		kind     trigger.Kind
		kindName string
	)
	switch {
	case ignoreFlag:
		kind = trigger.Ignored
		kindName = "ignore"
	case changeFlag:
		fallthrough
	default:
		kind = trigger.Changed
		kindName = "change"
	}

	reason := s.Reason
	if reason == "" {
		reason = defaultTriggerReason
	}
	cfg := s.Engine.Config()
	rootdir := cfg.HostDir()
	basePath := s.Path
	if !path.IsAbs(basePath) {
		basePath = filepath.Join(s.WorkingDir, filepath.FromSlash(basePath))
	} else {
		basePath = filepath.Join(rootdir, filepath.FromSlash(basePath))
	}
	basePath = filepath.Clean(basePath)
	_, err := os.Lstat(basePath)
	if errors.Is(err, os.ErrNotExist) {
		return errors.E(err, "path not found")
	}
	tmp, err := filepath.EvalSymlinks(basePath)
	if err != nil {
		return errors.E(err, "failed to evaluate stack path symlinks")
	}
	if tmp != basePath {
		return errors.E(fmt.Sprintf("symlinks are disallowed in the path: %s links to %s", basePath, tmp))
	}
	if !strings.HasPrefix(basePath, rootdir) {
		return errors.E("path %s is outside project", basePath)
	}
	prjBasePath := project.PrjAbsPath(rootdir, basePath)
	var stacks config.List[*config.SortableStack]
	if !s.Recursive {
		st, found, err := config.TryLoadStack(cfg, prjBasePath)
		if err != nil {
			return errors.E(err, "loading stack in current directory")
		}
		if !found {
			return errors.E("path is not a stack and --recursive is not provided")
		}
		stacks = append(stacks, st.Sortable())
	} else {
		stacksReport, err := s.Engine.ListStacks(engine.NoGitFilter(), cloudstack.AnyTarget, resources.NoStatusFilters(), false)
		if err != nil {
			return errors.E(err, "listing stacks")
		}
		filteredStacks, err := s.Engine.FilterStacks(stacksReport.Stacks,
			engine.ByBasePath(prjBasePath),
			engine.ByTags(s.Tags, s.NoTags),
		)
		if err != nil {
			return errors.E(err, "filtering stacks")
		}
		for _, entry := range filteredStacks {
			stacks = append(stacks, entry.Stack.Sortable())
		}
	}
	for _, st := range stacks {
		if err := trigger.Create(cfg, st.Dir(), kind, reason); err != nil {
			return errors.E(err, "unable to create trigger")
		}
		s.Printers.Stdout.Println(fmt.Sprintf("Created %s trigger for stack %q", kindName, st.Dir()))
	}
	return nil
}
