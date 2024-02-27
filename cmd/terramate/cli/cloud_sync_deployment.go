// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/hashicorp/go-uuid"
	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/deployment"
	"github.com/terramate-io/terramate/cmd/terramate/cli/clitest"
	"github.com/terramate-io/terramate/errors"
	prj "github.com/terramate-io/terramate/project"
)

// Assume each task in deployRuns has the CloudSyncDeployment set.
func (c *cli) createCloudDeployment(deployRuns []stackCloudRun) {
	logger := log.With().
		Logger()

	err := c.setupAuthMethod()
	c.handleCriticalError(err)

	if !c.cloudEnabled() {
		return
	}

	uuid, err := uuid.GenerateUUID()
	c.handleCriticalError(err)

	if !c.cloudEnabled() {
		return
	}

	c.cloudCtx.run.runUUID = cloud.UUID(uuid)

	logger = logger.With().
		Str("organization", string(c.cloudCtx.run.orgUUID)).
		Logger()

	ctx, cancel := context.WithTimeout(context.Background(), defaultCloudTimeout)
	defer cancel()

	var (
		deploymentCommitSHA string
		deploymentURL       string
		ghRepo              string
	)

	if c.prj.isRepo {
		r, err := repository.Parse(c.prj.prettyRepo())
		if err != nil {
			logger.Debug().
				Msg("repository cannot be normalized: skipping pull request retrievals for commit")
		} else {
			ghRepo = r.Owner + "/" + r.Name
		}

		deploymentCommitSHA = c.prj.headCommit()
	}

	ghRunID := os.Getenv("GITHUB_RUN_ID")
	ghAttempt := os.Getenv("GITHUB_RUN_ATTEMPT")
	if ghRunID != "" && ghAttempt != "" && ghRepo != "" {
		deploymentURL = fmt.Sprintf(
			"https://github.com/%s/actions/runs/%s/attempts/%s",
			ghRepo,
			ghRunID,
			ghAttempt,
		)

		logger.Debug().
			Str("deployment_url", deploymentURL).
			Msg("detected deployment url")
	}

	payload := cloud.DeploymentStacksPayloadRequest{
		ReviewRequest: c.cloudCtx.run.reviewRequest,
		Workdir:       prj.PrjAbsPath(c.rootdir(), c.wd()),
		Metadata:      c.cloudCtx.run.metadata,
	}

	for _, run := range deployRuns {
		tags := run.Stack.Tags
		if tags == nil {
			tags = []string{}
		}
		payload.Stacks = append(payload.Stacks, cloud.DeploymentStackRequest{
			Stack: cloud.Stack{
				MetaID:          strings.ToLower(run.Stack.ID),
				MetaName:        run.Stack.Name,
				MetaDescription: run.Stack.Description,
				MetaTags:        tags,
				Repository:      c.prj.prettyRepo(),
				DefaultBranch:   c.prj.gitcfg().DefaultBranch,
				Path:            run.Stack.Dir.String(),
			},
			CommitSHA:         deploymentCommitSHA,
			DeploymentCommand: strings.Join(run.Task.Cmd, " "),
			DeploymentURL:     deploymentURL,
		})
	}
	res, err := c.cloudCtx.client.CreateDeploymentStacks(ctx, c.cloudCtx.run.orgUUID, c.cloudCtx.run.runUUID, payload)
	if err != nil {
		logger.Error().
			Err(err).
			Msg("failed to create cloud deployment")

		c.disableCloudFeatures(cloudError())
		return
	}

	if len(res) != len(deployRuns) {
		logger.Error().
			Msgf("the backend respond with an invalid number of stacks in the deployment: %d instead of %d",
				len(res), len(deployRuns))

		c.disableCloudFeatures(cloudError())
		return
	}

	for _, r := range res {
		logger.Debug().Msgf("deployment created: %+v\n", r)
		if r.StackMetaID == "" {
			logger.Error().
				Msg("backend returned empty meta_id")

			c.disableCloudFeatures(cloudError())
			return
		}
		c.cloudCtx.run.meta2id[r.StackMetaID] = r.StackID
	}
}

func (c *cli) cloudSyncDeployment(run stackCloudRun, err error) {
	var status deployment.Status
	switch {
	case err == nil:
		status = deployment.OK
	case errors.IsKind(err, ErrRunCanceled):
		status = deployment.Canceled
	case errors.IsAnyKind(err, ErrRunFailed, ErrRunCommandNotFound):
		status = deployment.Failed
	default:
		panic(errors.E(errors.ErrInternal, "unexpected run status"))
	}

	c.doCloudSyncDeployment(run, status)
}

func (c *cli) doCloudSyncDeployment(run stackCloudRun, status deployment.Status) {
	logger := log.With().
		Str("organization", string(c.cloudCtx.run.orgUUID)).
		Str("stack", run.Stack.RelPath()).
		Stringer("status", status).
		Logger()

	stackID, ok := c.cloudCtx.run.meta2id[run.Stack.ID]
	if !ok {
		logger.Error().Msg("unable to update deployment status due to invalid API response")
		return
	}

	var details *cloud.ChangesetDetails

	if planfile := run.Task.CloudSyncTerraformPlanFile; planfile != "" {
		var err error
		details, err = c.getTerraformChangeset(run, planfile)
		if err != nil {
			logger.Error().Err(err).Msg(clitest.CloudSkippingTerraformPlanSync)
		}
	}

	payload := cloud.UpdateDeploymentStacks{
		Stacks: []cloud.UpdateDeploymentStack{
			{
				StackID: stackID,
				Status:  status,
				Details: details,
			},
		},
	}

	logger.Debug().Msg("updating deployment status")

	ctx, cancel := context.WithTimeout(context.Background(), defaultCloudTimeout)
	defer cancel()
	err := c.cloudCtx.client.UpdateDeploymentStacks(ctx, c.cloudCtx.run.orgUUID, c.cloudCtx.run.runUUID, payload)
	if err != nil {
		logger.Err(err).Str("stack_id", run.Stack.ID).Msg("failed to update deployment status for each")
	} else {
		logger.Debug().Msg("deployment status synced successfully")
	}
}
