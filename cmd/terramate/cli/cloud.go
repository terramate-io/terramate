// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"context"
	"fmt"
	"os"

	"time"

	"github.com/hashicorp/go-uuid"
	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/api/drift"
	"github.com/terramate-io/terramate/cloud/api/resources"
	"github.com/terramate-io/terramate/cloud/api/stack"
	"github.com/terramate-io/terramate/cloudsync"

	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/ui/tui/cliauth"
	"github.com/terramate-io/terramate/ui/tui/clitest"

	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/errors/verbosity"
	"github.com/terramate-io/terramate/printer"
)

const (
	cloudFeatStatus          = "--status' is a Terramate Cloud feature to filter stacks that failed to deploy or have drifted."
	cloudFeatSyncDeployment  = "'--sync-deployment' is a Terramate Cloud feature to synchronize deployment details to Terramate Cloud."
	cloudFeatSyncDriftStatus = "'--sync-drift-status' is a Terramate Cloud feature to synchronize drift and health check results to Terramate Cloud."
	cloudFeatSyncPreview     = "'--sync-preview' is a Terramate Cloud feature to synchronize deployment previews to Terramate Cloud."
)

// newCloudLoginRequiredError creates an error indicating that a cloud login is required to use requested features.
func newCloudLoginRequiredError(requestedFeatures []string) *errors.DetailedError {
	err := errors.D(clitest.CloudLoginRequiredMessage)

	for _, s := range requestedFeatures {
		err = err.WithDetailf(verbosity.V1, "%s", s)
	}

	err = err.WithDetailf(verbosity.V1, "To login with an existing account, run 'terramate cloud login'.").
		WithDetailf(verbosity.V1, "To create a free account, visit https://cloud.terramate.io.")

	return err.WithCode(clitest.ErrCloud)
}

func newCloudOnboardingIncompleteError(region cloud.Region) *errors.DetailedError {
	err := errors.D(clitest.CloudOnboardingIncompleteMessage)
	err = err.WithDetailf(verbosity.V1, "Visit %s to setup your account.", cloud.HTMLURL(region))
	return err.WithCode(clitest.ErrCloudOnboardingIncomplete)
}

func (c *cli) checkCloudSync(state *cloudsync.CloudRunState) {
	if !c.parsedArgs.Run.SyncDeployment && !c.parsedArgs.Run.SyncDriftStatus && !c.parsedArgs.Run.SyncPreview {
		return
	}

	var feats []string
	if c.parsedArgs.Run.SyncDeployment {
		feats = append(feats, cloudFeatSyncDeployment)
	}
	if c.parsedArgs.Run.SyncDriftStatus {
		feats = append(feats, cloudFeatSyncDriftStatus)
	}
	if c.parsedArgs.Run.SyncPreview {
		feats = append(feats, cloudFeatSyncPreview)
	}

	err := c.engine.SetupCloudConfig(feats)
	err = c.engine.HandleCloudCriticalError(err)
	if err != nil {
		fatal(err)
	}

	if c.engine.IsCloudDisabled() {
		return
	}

	if c.parsedArgs.Run.SyncDeployment {
		uuid, err := uuid.GenerateUUID()
		err = c.engine.HandleCloudCriticalError(err)
		if err != nil {
			fatal(err)
		} else {
			state.RunUUID = resources.UUID(uuid)
		}
	}
}

func (c *cli) cloudOrgName() string {
	orgName := os.Getenv("TM_CLOUD_ORGANIZATION")
	if orgName != "" {
		return orgName
	}

	cfg := c.rootNode()
	if cfg.Terramate != nil &&
		cfg.Terramate.Config != nil &&
		cfg.Terramate.Config.Cloud != nil {
		return cfg.Terramate.Config.Cloud.Organization
	}

	return ""
}

func (c *cli) ssoLogin() {
	if !c.parsedArgs.Cloud.Login.SSO {
		panic(errors.E(errors.ErrInternal, "please report this as a bug"))
	}

	orgName := c.cloudOrgName()
	if orgName == "" {
		fatalWithDetailf(
			errors.E("No Terramate Cloud organization configured."),
			"Set `terramate.config.cloud.organization` or export `TM_CLOUD_ORGANIZATION` to the organization shortname that you intend to login.",
		)
	}

	region := c.cloudRegion()
	cloudURL, envFound := cliauth.EnvBaseURL()
	if !envFound {
		cloudURL = cloud.BaseURL(region)
	}

	opts := cloud.Options{
		cloud.WithRegion(region),
		cloud.WithHTTPClient(&c.httpClient),
	}
	if envFound {
		opts = append(opts, cloud.WithBaseURL(cloudURL))
	}

	client := cloud.NewClient(opts...)

	ctx, cancel := context.WithTimeout(context.Background(), cloud.DefaultTimeout)
	defer cancel()
	ssoOrgID, err := client.GetOrgSingleSignOnID(ctx, orgName)
	if err != nil {
		fatal(errors.E("Organization %s doesn't have SSO enabled", orgName))
	}

	err = cliauth.SSOLogin(printer.DefaultPrinters, c.parsedArgs.Verbose, ssoOrgID, c.clicfg)
	if err != nil {
		fatalWithDetailf(err, "Failed to authenticate")
	}

	err = c.engine.LoadCredential("oidc.workos")
	if err != nil {
		fatalWithDetailf(err, "failed to load credentials")
	}

	client = c.engine.CloudClient()
	ctx, cancel = context.WithTimeout(context.Background(), cloud.DefaultTimeout)
	defer cancel()
	user, err := client.Users(ctx)
	if err != nil {
		fatalWithDetailf(err, "failed to test token")
	}

	c.output.MsgStdOut("Logged in as %s", user.DisplayName)
	c.output.MsgStdOutV("Expire at: %s", c.engine.Credential().ExpireAt().Format(time.RFC822Z))
}

func (c *cli) cloudInfo() {
	err := c.engine.LoadCredential()
	if err != nil {
		if errors.IsKind(err, cliauth.ErrLoginRequired) {
			fatalWithDetailf(
				newCloudLoginRequiredError([]string{"The `terramate cloud info` shows information about your current credentials to Terramate Cloud."}).WithCause(err),
				"failed to load the cloud credentials",
			)
		}
		if errors.IsKind(err, clitest.ErrCloudOnboardingIncomplete) {
			fatal(newCloudOnboardingIncompleteError(c.engine.CloudClient().Region()))
		}
		fatalWithDetailf(err, "failed to load the cloud credentials")
	}
	cred := c.engine.Credential()
	cred.Info(c.cloudOrgName())

	// verbose info
	if c.parsedArgs.Verbose > 0 && cred.HasExpiration() {
		printer.Stdout.Println(fmt.Sprintf("next token refresh in: %s", time.Until(cred.ExpireAt())))
	}
}

func (c *cli) cloudDriftShow() {
	prj := c.project()
	if !prj.IsRepo() {
		fatal("drift show requires a repository")
	}

	err := c.engine.SetupCloudConfig([]string{"drift show"})
	if err != nil {
		fatal(err)
	}
	st, found, err := config.TryLoadStack(c.cfg(), project.PrjAbsPath(c.rootdir(), c.wd()))
	if err != nil {
		fatalWithDetailf(err, "loading stack in current directory")
	}
	if !found {
		fatal("No stack selected. Please enter a stack to show a potential drift.")
	}
	if st.ID == "" {
		fatal("The stack must have an ID for using TMC features")
	}

	target := c.parsedArgs.Cloud.Drift.Show.Target

	isTargetConfigEnabled := false
	err = c.engine.CheckTargetsConfiguration(target, "", func(isTargetEnabled bool) error {
		if !isTargetEnabled {
			return errors.E("--target must be set when terramate.config.cloud.targets.enabled is true")
		}
		isTargetConfigEnabled = isTargetEnabled
		return nil
	})
	if err != nil {
		fatal(err)
	}

	if target == "" {
		target = "default"
	}

	prettyRepo, err := prj.PrettyRepo()
	if err != nil {
		fatalWithDetailf(err, "unable to canonicalize repository")
	}

	ctx, cancel := context.WithTimeout(context.Background(), cloud.DefaultTimeout)
	defer cancel()
	client := c.engine.CloudClient()
	orgUUID := c.engine.CloudState().Org.UUID
	stackResp, found, err := client.GetStack(ctx, orgUUID, prettyRepo, target, st.ID)
	if err != nil {
		fatalWithDetailf(err, "unable to fetch stack")
	}
	if !found {
		if isTargetConfigEnabled {
			fatalf("Stack %s was not yet synced for target %s with the Terramate Cloud.", st.Dir.String(), target)
		} else {
			fatalf("Stack %s was not yet synced with the Terramate Cloud.", st.Dir.String())
		}
	}

	if stackResp.Status != stack.Drifted && stackResp.DriftStatus != drift.Drifted {
		c.output.MsgStdOut("Stack %s is not drifted.", st.Dir.String())
		return
	}

	ctx, cancel = context.WithTimeout(context.Background(), cloud.DefaultTimeout)
	defer cancel()

	// stack is drifted
	driftsResp, err := client.StackLastDrift(ctx, orgUUID, stackResp.ID)
	if err != nil {
		fatalWithDetailf(err, "unable to fetch drift")
	}
	if len(driftsResp.Drifts) == 0 {
		fatalf("Stack %s is drifted, but no details are available.", st.Dir.String())
	}
	driftData := driftsResp.Drifts[0]

	ctx, cancel = context.WithTimeout(context.Background(), cloud.DefaultTimeout)
	defer cancel()
	driftData, err = client.DriftDetails(ctx, orgUUID, stackResp.ID, driftData.ID)
	if err != nil {
		fatalWithDetailf(err, "unable to fetch drift details")
	}
	if driftData.Status != drift.Drifted || driftData.Details == nil || driftData.Details.Provisioner == "" {
		fatalf("Stack %s is drifted, but no details are available.", st.Dir.String())
	}
	c.output.MsgStdOutV("drift provisioner: %s", driftData.Details.Provisioner)
	c.output.MsgStdOut(driftData.Details.ChangesetASCII)
}

func (c *cli) ensureAllStackHaveIDs(stacks config.List[*config.SortableStack]) {
	logger := log.With().
		Str("action", "cli.ensureAllStackHaveIDs").
		Logger()

	var stacksMissingIDs []string
	for _, st := range stacks {
		if st.ID == "" {
			stacksMissingIDs = append(stacksMissingIDs, st.Dir().String())
		}
	}

	if len(stacksMissingIDs) > 0 {
		for _, stackPath := range stacksMissingIDs {
			logger.Error().Str("stack", stackPath).Msg("stack is missing the ID field")
		}
		logger.Warn().Msg("Stacks are missing IDs. You can use 'terramate create --ensure-stack-ids' to add missing IDs to all stacks.")

		err := c.engine.HandleCloudCriticalError(errors.E(clitest.ErrCloudStacksWithoutID))
		if err != nil {
			fatal(err)
		}
	}
}
