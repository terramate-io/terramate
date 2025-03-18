package run

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/deployment"
	"github.com/terramate-io/terramate/cmd/terramate/cli/clitest"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/project"
)

func CreateCloudDeployment(e *engine.Engine, wd string, deployRuns []engine.StackCloudRun, state *CloudRunState) error {
	// Assume each task in deployRuns has the CloudSyncDeployment set.
	logger := log.With().
		Logger()

	if !e.IsCloudEnabled() {
		return nil
	}

	orgUUID := e.CloudState().Org.UUID

	logger = logger.With().
		Str("organization", string(orgUUID)).
		Logger()

	ctx, cancel := context.WithTimeout(context.Background(), defaultCloudTimeout)
	defer cancel()

	var (
		err                 error
		deploymentCommitSHA string
		deploymentURL       string
		ghRepo              string
	)

	prj := e.Project()
	prettyRepo, err := prj.PrettyRepo()
	if err != nil {
		return e.HandleCloudCriticalError(err)
	}

	if prj.IsRepo() {
		r, err := repository.Parse(prettyRepo)
		if err != nil {
			logger.Debug().
				Msg("repository cannot be normalized: skipping pull request retrievals for commit")
		} else {
			ghRepo = r.Owner + "/" + r.Name
		}

		deploymentCommitSHA = prj.Git.HeadCommit
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

	root := e.Config()
	payload := cloud.DeploymentStacksPayloadRequest{
		ReviewRequest: state.ReviewRequest,
		Workdir:       project.PrjAbsPath(root.HostDir(), wd),
		Metadata:      state.Metadata,
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
				Repository:      prettyRepo,
				Target:          run.Task.CloudTarget,
				FromTarget:      run.Task.CloudFromTarget,
				DefaultBranch:   prj.GitConfig().DefaultBranch,
				Path:            run.Stack.Dir.String(),
			},
			CommitSHA:         deploymentCommitSHA,
			DeploymentCommand: strings.Join(run.Task.Cmd, " "),
			DeploymentURL:     deploymentURL,
		})
	}
	res, err := e.CloudClient().CreateDeploymentStacks(ctx, orgUUID, state.RunUUID, payload)
	if err != nil {
		logger.Error().
			Err(err).
			Msg("failed to create cloud deployment")

		return e.HandleCloudCriticalError(err)
	}

	if len(res) != len(deployRuns) {
		logger.Error().
			Msgf("the backend respond with an invalid number of stacks in the deployment: %d instead of %d",
				len(res), len(deployRuns))

		return e.HandleCloudCriticalError(err)
	}

	for _, r := range res {
		logger.Debug().Msgf("deployment created: %+v\n", r)
		if r.StackMetaID == "" {
			logger.Error().
				Msg("backend returned empty meta_id")

			return e.HandleCloudCriticalError(err)
		}
		state.SetMeta2CloudID(r.StackMetaID, r.StackID)
	}
	return nil
}

func cloudSyncDeployment(e *engine.Engine, run engine.StackCloudRun, state *CloudRunState, err error) {
	var status deployment.Status
	switch {
	case err == nil:
		status = deployment.OK
	case errors.IsKind(err, engine.ErrRunCanceled):
		status = deployment.Canceled
	case errors.IsAnyKind(err, engine.ErrRunFailed, engine.ErrRunCommandNotExecuted):
		status = deployment.Failed
	default:
		panic(errors.E(errors.ErrInternal, "unexpected run status"))
	}

	doCloudSyncDeployment(e, run, state, status)
}

func doCloudSyncDeployment(e *engine.Engine, run engine.StackCloudRun, state *CloudRunState, status deployment.Status) {
	orgUUID := e.CloudState().Org.UUID
	logger := log.With().
		Str("organization", string(orgUUID)).
		Str("stack", run.Stack.RelPath()).
		Stringer("status", status).
		Logger()

	stackID, ok := state.StackCloudID(run.Stack.ID)
	if !ok {
		logger.Error().Msg("unable to update deployment status due to invalid API response")
		return
	}

	var details *cloud.ChangesetDetails

	if run.Task.CloudPlanFile != "" {
		var err error
		details, err = getTerraformChangeset(e, run)
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
	err := e.CloudClient().UpdateDeploymentStacks(ctx, orgUUID, state.RunUUID, payload)
	if err != nil {
		logger.Err(err).Str("stack_id", run.Stack.ID).Msg("failed to update deployment status for each")
	} else {
		logger.Debug().Msg("deployment status synced successfully")
	}
}
