// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package engine

import (
	"context"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/api/resources"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/errors/verbosity"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/ui/tui/cliauth"
	"github.com/terramate-io/terramate/ui/tui/clitest"
	tel "github.com/terramate-io/terramate/ui/tui/telemetry"
)

const (
	defaultCloudTimeout     = 60 * time.Second
	defaultGoogleTimeout    = defaultCloudTimeout
	defaultGithubTimeout    = defaultCloudTimeout
	defaultGitlabTimeout    = defaultCloudTimeout
	defaultBitbucketTimeout = defaultCloudTimeout
)

// CloudState represents the state of the cloud.
type CloudState struct {
	disabled bool
	client   *cloud.Client

	Org CloudOrgState
}

// CloudOrgState represents the state of a cloud organization.
type CloudOrgState struct {
	Name string
	UUID resources.UUID
}

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

// CloudClient returns the cloud client.
func (e *Engine) CloudClient() *cloud.Client {
	return e.state.cloud.client
}

// Credential returns the cloud credential.
func (e *Engine) Credential() cliauth.Credential {
	return e.CloudClient().Credential().(cliauth.Credential)
}

// LoadCredential loads the cloud credential from the environment or configuration.
func (e *Engine) LoadCredential(preferences ...string) error {
	region := e.CloudRegion()
	cloudURL, envFound := cliauth.EnvBaseURL()
	if !envFound {
		cloudURL = cloud.BaseURL(region)
	}
	clientLogger := log.With().
		Str("tmc_url", cloudURL).
		Logger()

	opts := []cloud.Option{
		cloud.WithRegion(region), // always set so we can use it in error messages
		cloud.WithHTTPClient(&e.HTTPClient),
		cloud.WithLogger(&clientLogger),
	}
	if envFound {
		opts = append(opts, cloud.WithBaseURL(cloudURL))
	}
	e.state.cloud.client = cloud.NewClient(opts...)

	// checks if this client version can communicate with Terramate Cloud.
	ctx, cancel := context.WithTimeout(context.Background(), defaultCloudTimeout)
	defer cancel()
	err := e.state.cloud.client.CheckVersion(ctx)
	if err != nil {
		return errors.E(err, clitest.ErrCloudCompat)
	}

	probes := cliauth.ProbingPrecedence(e.printers, e.verbosity, e.state.cloud.client, e.usercfg)
	var found bool
	for _, probe := range probes {
		var err error
		found, err = probe.Load()
		if err != nil {
			return err
		}
		if len(preferences) > 0 && !slices.Contains(preferences, probe.Name()) {
			continue
		}
		if found {
			break
		}
	}
	if !found {
		return errors.E("no credential found", cliauth.ErrLoginRequired)
	}
	return nil
}

// CloudState returns the cloud state.
func (e *Engine) CloudState() CloudState {
	return e.state.cloud
}

// SetupCloudConfig sets up the cloud configuration.
func (e *Engine) SetupCloudConfig(requestedFeatures []string) error {
	if e.state.cloud.Org.UUID != "" {
		// already setup
		return nil
	}
	err := e.LoadCredential()
	if err != nil {
		if errors.IsKind(err, cliauth.ErrLoginRequired) {
			e.printers.Stderr.Warn(err)
			return newCloudLoginRequiredError(requestedFeatures).WithCause(err)
		}
		if errors.IsKind(err, clitest.ErrCloudOnboardingIncomplete) {
			return newCloudOnboardingIncompleteError(e.state.cloud.client.Region()).WithCause(err)
		}
		printer.Stderr.ErrorWithDetails("failed to load the cloud credentials", err)
		return cloudError()
	}

	// at this point we know user is onboarded, ie has at least 1 organization.
	orgs := e.Credential().Organizations()

	var activeOrgs resources.MemberOrganizations
	var invitedOrgs resources.MemberOrganizations
	var ssoInvitedOrgs resources.MemberOrganizations
	for _, org := range orgs {
		if org.Status == "active" || org.Status == "trusted" {
			activeOrgs = append(activeOrgs, org)
		} else if org.Status == "invited" {
			invitedOrgs = append(invitedOrgs, org)
		} else if org.Status == "sso_invited" {
			ssoInvitedOrgs = append(ssoInvitedOrgs, org)
		}
	}

	useOrgName := e.CloudOrgName()
	e.state.cloud.Org.Name = useOrgName
	if useOrgName != "" {
		var useOrgUUID resources.UUID
		for _, org := range orgs {
			if strings.EqualFold(org.Name, useOrgName) {
				if org.Status != "active" && org.Status != "trusted" {
					printer.Stderr.ErrorWithDetails(
						"Invalid membership status",
						errors.E(
							"You are not yet an active member of organization %s. Please accept the invitation first.",
							useOrgName,
						),
					)

					return cloudError()
				}

				useOrgUUID = org.UUID
				break
			}
		}

		if useOrgUUID == "" {
			printer.Stderr.ErrorWithDetails(
				"Invalid membership status",
				errors.E(
					"You are not a member of organization %q or the organization does not exist. Available organizations: %s",
					useOrgName,
					orgs,
				),
			)

			return cloudError()
		}

		e.state.cloud.Org.UUID = useOrgUUID
	} else {
		if len(activeOrgs) == 0 {
			printer.Stderr.Error(clitest.CloudNoMembershipMessage)

			for _, org := range invitedOrgs {
				domainStr := ""
				if org.Domain != "" {
					domainStr = " (" + org.Domain + ") "
				}
				printer.Stderr.WarnWithDetails(
					"Pending invitation",
					errors.E(
						"You have pending invitation for the organization %s%s",
						org.Name, domainStr,
					),
				)
			}

			for _, org := range ssoInvitedOrgs {
				domainStr := ""
				if org.Domain != "" {
					domainStr = " (" + org.Domain + ") "
				}
				printer.Stderr.WarnWithDetails(
					"Pending SSO invitation",
					errors.E(
						"If you trust the %s%s organization, go to %s to join it",
						org.Name, domainStr, cloud.HTMLURL(e.CloudRegion()),
					),
				)
			}

			return errors.E(clitest.ErrCloudOnboardingIncomplete)
		}
		printer.Stderr.ErrorWithDetails(
			"Missing cloud configuration",
			errors.E("Please set TM_CLOUD_ORGANIZATION environment variable or "+
				"terramate.config.cloud.organization configuration attribute to a specific organization",
			),
		)
		return cloudError()
	}

	if e.state.cloud.Org.UUID != "" {
		tel.DefaultRecord.Set(tel.OrgUUID(e.state.cloud.Org.UUID))
	}

	return nil
}

// CloudOrgName returns the name of the cloud organization.
func (e *Engine) CloudOrgName() string {
	orgName := os.Getenv("TM_CLOUD_ORGANIZATION")
	if orgName != "" {
		return orgName
	}
	cfg := e.RootNode()
	if cfg.Terramate != nil &&
		cfg.Terramate.Config != nil &&
		cfg.Terramate.Config.Cloud != nil {
		return cfg.Terramate.Config.Cloud.Organization
	}
	return ""
}

// IsCloudEnabled returns true if cloud features are enabled.
func (e *Engine) IsCloudEnabled() bool {
	return !e.state.cloud.disabled
}

// IsCloudDisabled returns true if cloud features are disabled.
func (e *Engine) IsCloudDisabled() bool {
	return !e.IsCloudEnabled()
}

// DisableCloudFeatures disables cloud features and prints an error message.
func (e *Engine) DisableCloudFeatures(err error) {
	printer.Stderr.WarnWithDetails(clitest.CloudDisablingMessage, errors.E(err.Error()))

	e.state.cloud.disabled = true
}

// SelectCloudStackTasks selects cloud tasks from a list of stack runs.
func SelectCloudStackTasks(runs []StackRun, pred func(StackRunTask) bool) []StackCloudRun {
	var cloudRuns []StackCloudRun
	for _, run := range runs {
		for _, t := range run.Tasks {
			if pred(t) {
				cloudRuns = append(cloudRuns, StackCloudRun{
					Stack: run.Stack,
					Task:  t,
				})
				// Currently, only a single task per stackRun group may be selected.
				break
			}
		}
	}
	return cloudRuns
}

// IsDeploymentTask returns true if the task is a deployment task.
func IsDeploymentTask(t StackRunTask) bool { return t.CloudSyncDeployment }

// IsDriftTask returns true if the task is a drift task.
func IsDriftTask(t StackRunTask) bool { return t.CloudSyncDriftStatus }

// IsPreviewTask returns true if the task is a preview task.
func IsPreviewTask(t StackRunTask) bool { return t.CloudSyncPreview }

func cloudError() error {
	return errors.E(clitest.ErrCloud)
}

// HandleCloudCriticalError handles the error depending on the UI mode set.
// The error is logged as a warning and ignored if the UI mode is [AutomationMode].
func (e *Engine) HandleCloudCriticalError(err error) error {
	if err != nil {
		if e.uimode == HumanMode {
			return err
		}

		e.DisableCloudFeatures(err)
	}
	return nil
}
