package stack

import (
	"context"
	"fmt"

	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/run"
	"github.com/terramate-io/terramate/stack"
)

type Spec struct {
	Engine        *engine.Engine
	GitFilter     engine.GitFilter
	Reason        bool
	Target        string
	StatusFilters StatusFilters
	RunOrder      bool
	Tags          []string
	NoTags        []string
	Printers      printer.Printers
}

type StatusFilters struct {
	StackStatus      string
	DeploymentStatus string
	DriftStatus      string
}

func (s *Spec) Name() string { return "list" }

func (s *Spec) Exec(ctx context.Context) error {
	if s.Reason && !s.GitFilter.IsChanged {
		return errors.E("the --why flag must be used together with --changed")
	}

	s.Engine.CheckTargetsConfiguration(s.Target, "", func(isTargetSet bool) error {
		isStatusSet := s.StatusFilters.StackStatus != ""
		isDeploymentStatusSet := s.StatusFilters.DeploymentStatus != ""
		isDriftStatusSet := s.StatusFilters.DriftStatus != ""

		if isTargetSet && (!isStatusSet && !isDeploymentStatusSet && !isDriftStatusSet) {
			return errors.E("--target must be used together with --status or --deployment-status or --drift-status")
		} else if !isTargetSet && (isStatusSet || isDeploymentStatusSet || isDriftStatusSet) {
			return errors.E("--status, --deployment-status and --drift-status requires --target when terramate.config.cloud.targets.enabled is true")
		}
		return nil
	})

	cloudFilters, err := cloud.ParseStatusFilters(s.StatusFilters.StackStatus, s.StatusFilters.DeploymentStatus, s.StatusFilters.DriftStatus)
	if err != nil {
		return err
	}

	report, err := s.Engine.ListStacks(s.GitFilter, s.Target, cloudFilters, false)
	if err != nil {
		return err
	}
	return s.printStacksList(report.Stacks)
}

func (s *Spec) printStacksList(allStacks []stack.Entry) error {
	tags, err := engine.ParseFilterTags(s.Tags, s.NoTags)
	if err != nil {
		return err
	}
	filteredStacks := s.Engine.FilterStacks(allStacks, tags)

	reasons := map[string]string{}
	stacks := make(config.List[*config.SortableStack], len(filteredStacks))
	for i, entry := range filteredStacks {
		stacks[i] = entry.Stack.Sortable()
		reasons[entry.Stack.ID] = entry.Reason
	}

	if s.RunOrder {
		var failReason string
		var err error
		failReason, err = run.Sort(s.Engine.Config(), stacks,
			func(s *config.SortableStack) *config.Stack { return s.Stack })
		if err != nil {
			return errors.E(err, "Invalid stack configuration: "+failReason)
		}
	}

	for _, st := range stacks {
		dir := st.Dir().String()
		friendlyDir, ok := s.Engine.FriendlyFmtDir(dir)
		if !ok {
			printer.Stderr.Error(fmt.Sprintf("Unable to format stack dir %s", dir))
			printer.Stdout.Println(dir)
			continue
		}

		if s.Reason {
			printer.Stdout.Println(fmt.Sprintf("%s - %s", friendlyDir, reasons[st.ID]))
		} else {
			printer.Stdout.Println(friendlyDir)
		}
	}
	return nil
}
