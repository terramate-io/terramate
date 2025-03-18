package show

import (
	"context"
	"fmt"

	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/drift"
	"github.com/terramate-io/terramate/cloud/stack"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/project"
)

type Spec struct {
	WorkingDir  string
	Engine      *engine.Engine
	Printers    printer.Printers
	Verbosiness int

	Target string
}

func (s *Spec) Name() string { return "cloud drift show" }

func (s *Spec) Exec(ctx context.Context) error {
	err := s.Engine.SetupCloudConfig([]string{fmt.Sprintf("%q command shows the drift status of the stack", s.Name())})
	if err != nil {
		return err
	}

	cfg := s.Engine.Config()
	rootdir := cfg.HostDir()
	st, found, err := config.TryLoadStack(cfg, project.PrjAbsPath(rootdir, s.WorkingDir))
	if err != nil {
		return errors.E(err, "loading stack in current directory")
	}
	if !found {
		return errors.E("No stack selected. Please enter a stack to show a potential drift.")
	}
	if st.ID == "" {
		return errors.E("The stack must have an ID for using TMC features")
	}

	target := s.Target

	isTargetConfigEnabled := false
	err = s.Engine.CheckTargetsConfiguration(target, "", func(isTargetEnabled bool) error {
		if !isTargetEnabled {
			return errors.E("--target must be set when terramate.config.cloud.targets.enabled is true")
		}
		isTargetConfigEnabled = isTargetEnabled
		return nil
	})
	if err != nil {
		return err
	}

	if target == "" {
		target = "default"
	}

	client := s.Engine.CloudClient()
	org := s.Engine.CloudState().Org
	repo, err := s.Engine.Project().PrettyRepo()
	if err != nil {
		return err
	}
	getStackCtx, cancel := context.WithTimeout(ctx, cloud.DefaultTimeout)
	defer cancel()
	stackResp, found, err := client.GetStack(getStackCtx, org.UUID, repo, target, st.ID)
	if err != nil {
		return errors.E(err, "unable to fetch stack")
	}
	if !found {
		if isTargetConfigEnabled {
			return errors.E("Stack %s was not yet synced for target %s with the Terramate Cloud.", st.Dir.String(), target)
		} else {
			return errors.E("Stack %s was not yet synced with the Terramate Cloud.", st.Dir.String())
		}
	}

	if stackResp.Status != stack.Drifted && stackResp.DriftStatus != drift.Drifted {
		s.Printers.Stdout.Println(fmt.Sprintf("Stack %s is not drifted.", st.Dir.String()))
		return nil
	}

	getStackListCtx, cancel := context.WithTimeout(context.Background(), cloud.DefaultTimeout)
	defer cancel()

	// stack is drifted
	driftsResp, err := client.StackLastDrift(getStackListCtx, org.UUID, stackResp.ID)
	if err != nil {
		return errors.E(err, "unable to fetch drift")
	}
	if len(driftsResp.Drifts) == 0 {
		return errors.E("Stack %s is drifted, but no details are available.", st.Dir.String())
	}
	driftData := driftsResp.Drifts[0]

	getDriftDetailsCtx, cancel := context.WithTimeout(context.Background(), cloud.DefaultTimeout)
	defer cancel()
	driftData, err = client.DriftDetails(getDriftDetailsCtx, org.UUID, stackResp.ID, driftData.ID)
	if err != nil {
		return errors.E(err, "unable to fetch drift details")
	}
	if driftData.Status != drift.Drifted || driftData.Details == nil || driftData.Details.Provisioner == "" {
		return errors.E("Stack %s is drifted, but no details are available.", st.Dir.String())
	}
	if s.Verbosiness > 0 {
		s.Printers.Stdout.Println(fmt.Sprintf("drift provisioner: %s", driftData.Details.Provisioner))
	}
	s.Printers.Stdout.Println(driftData.Details.ChangesetASCII)
	return nil
}
