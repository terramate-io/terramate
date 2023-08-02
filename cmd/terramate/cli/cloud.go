// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/golang-jwt/jwt"
	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cmd/terramate/cli/cliconfig"
	"github.com/terramate-io/terramate/cmd/terramate/cli/github"
	"github.com/terramate-io/terramate/cmd/terramate/cli/out"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
)

// ErrOnboardingIncomplete indicates the onboarding process is incomplete.
const ErrOnboardingIncomplete errors.Kind = "cloud commands cannot be used until onboarding is complete"

const (
	defaultCloudTimeout  = 5 * time.Second
	defaultGithubTimeout = defaultCloudTimeout
)

// DisablingCloudMessage is the message displayed in the warning when disabling
// the cloud features. It's exported because it's checked in tests.
const DisablingCloudMessage = "disabling the cloud features"

type cloudConfig struct {
	disabled bool
	client   *cloud.Client
	output   out.O

	credential credential

	run struct {
		runUUID string
		orgUUID string

		meta2id map[string]int
	}
}

type credential interface {
	Name() string
	Load() (bool, error)
	Token() (string, error)
	Refresh() error
	IsExpired() bool
	ExpireAt() time.Time
	Validate(cloudcfg cloudConfig) error
	organizations() cloud.MemberOrganizations
	Info()
}

type keyValue struct {
	key   string
	value string
}

func credentialPrecedence(output out.O, clicfg cliconfig.Config) []credential {
	return []credential{
		newGithubOIDC(output),
		newGoogleCredential(output, clicfg),
	}
}

func (c *cli) cloudEnabled() bool {
	return !c.cloud.disabled
}

func (c *cli) checkSyncDeployment() {
	if !c.parsedArgs.Run.CloudSyncDeployment {
		return
	}
	err := c.setupCloudConfig()
	if err != nil {
		if errors.IsKind(err, ErrOnboardingIncomplete) {
			c.cred().Info()
		}
		fatal(err)
	}

	if c.cloud.disabled {
		return
	}

	// at this point we know user is onboarded, ie has at least 1 organization.
	orgs := c.cred().organizations()

	useOrgName := os.Getenv("TM_CLOUD_ORGANIZATION")
	if useOrgName != "" {
		var useOrgUUID string
		for _, org := range orgs {
			if org.Name == useOrgName {
				if org.Status != "active" && org.Status != "trusted" {
					fatal(errors.E("You are not yet an active member of organization %s. Please accept the invitation first.", useOrgName))
				}

				useOrgUUID = org.UUID
				break
			}
		}

		if useOrgUUID == "" {
			fatal(errors.E("You are not a member of organization %q or the organization does not exist. Available organizations: %s",
				useOrgName,
				orgs,
			))
		}

		c.cloud.run.orgUUID = useOrgUUID
	} else if len(orgs) != 1 {
		fatal(
			errors.E("Please set TM_CLOUD_ORGANIZATION environment variable to a specific available organization: %s", orgs),
		)
	} else {
		org := orgs[0]
		if org.Status != "active" && org.Status != "trusted" {
			fatal(errors.E("You are not yet an active member of organization %s. Please accept the invitation first.", org.Name))
		}
		c.cloud.run.orgUUID = org.UUID
	}

	c.cloud.run.meta2id = make(map[string]int)

	c.cloud.run.runUUID, err = generateRunID()
	if err != nil {
		fatal(err, "generating run uuid")
	}
}

func (c *cli) setupCloudConfig() error {
	cred, err := c.loadCredential()
	if err != nil {
		return err
	}

	c.cloud = cloudConfig{
		client: &cloud.Client{
			BaseURL:    cloudBaseURL,
			HTTPClient: &c.httpClient,
			Credential: cred,
		},
		output:     c.output,
		credential: cred,
	}

	err = cred.Validate(c.cloud)
	if err != nil {
		log.Warn().Err(errors.E(err, "failed to check if credentials work")).
			Msg(DisablingCloudMessage)

		c.cloud.disabled = true
	}
	return nil
}

func (c *cli) createCloudDeployment(stacks config.List[*config.SortableStack], command []string) {
	logger := log.With().
		Logger()

	if !c.cloudEnabled() || !c.parsedArgs.Run.CloudSyncDeployment {
		return
	}

	logger.Trace().Msg("Checking if selected stacks have id")

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
		fatal(errors.E("The --cloud-sync-deployment flag requires that selected stacks contain an ID field"))
	}

	logger = logger.With().
		Str("organization", c.cloud.run.orgUUID).
		Logger()

	ctx, cancel := context.WithTimeout(context.Background(), defaultCloudTimeout)
	defer cancel()

	var (
		err                 error
		deploymentCommitSHA string
		deploymentURL       string
		normalizedRepo      string
		reviewRequest       *cloud.DeploymentReviewRequest
	)

	if c.prj.isRepo {
		var rr cloud.DeploymentReviewRequest
		repoURL, err := c.prj.git.wrapper.URL(c.prj.gitcfg().DefaultRemote)
		if err == nil {
			normalizedRepo = cloud.NormalizeGitURI(repoURL)
			rr.Repository = normalizedRepo
		} else {
			logger.Warn().Err(err).Msg("failed to retrieve repository URL")
		}

		deploymentCommitSHA = c.prj.headCommit()
		if len(c.prj.git.repoChecks.UntrackedFiles) > 0 ||
			len(c.prj.git.repoChecks.UncommittedFiles) > 0 {
			deploymentCommitSHA = ""

			logger.Debug().Msg("commit SHA is not being synced because the repository is dirty")
		}

		ghRepo := os.Getenv("GITHUB_REPOSITORY")
		if ghRepo == "" && normalizedRepo != "" && normalizedRepo != "local" {
			r, err := repository.Parse(normalizedRepo)
			if err != nil {
				logger.Warn().
					Str("repository", normalizedRepo).
					Err(err).
					Msg("failed to normalize the repository")
			} else {
				ghRepo = r.Owner + "/" + r.Name
			}
		}

		ghToken := os.Getenv("GITHUB_TOKEN")

		if ghRepo == "" {
			logger.Debug().
				Str("repository", normalizedRepo).
				Msg("repository cannot be normalized: skipping pull request retrievals for commit")

		} else {
			ghClient := github.Client{
				BaseURL:    os.Getenv("GITHUB_API_URL"),
				HTTPClient: &c.httpClient,
				Token:      ghToken,
			}

			ctx, cancel := context.WithTimeout(context.Background(), defaultGithubTimeout)
			defer cancel()

			pulls, err := ghClient.PullsForCommit(ctx, ghRepo, c.prj.headCommit())
			if err == nil {
				for _, pull := range pulls {
					logger.Debug().
						Str("associated-pull-url", pull.HTMLURL).
						Msg("found pull request")
				}

				if len(pulls) > 0 {
					pull := pulls[0]
					rr.Platform = "github"
					rr.URL = pull.HTMLURL
					rr.Number = pull.Number
					rr.Title = pull.Title
					rr.Description = pull.Body
					rr.CommitSHA = c.prj.headCommit()

					logger.Debug().
						Str("pull-url", rr.URL).
						Msg("using pull request url")

				} else {
					logger.Debug().
						Str("head-commit", c.prj.headCommit()).
						Msg("no pull request associated with HEAD commit")
				}
			} else {
				logger.Error().
					Str("normalized-repo", normalizedRepo).
					Str("gh-repo", ghRepo).
					Str("head-commit", c.prj.headCommit()).
					Err(err).
					Msg("failed to retrieve pull requests associated with HEAD")
			}
		}

		ghRunID := os.Getenv("GITHUB_RUN_ID")
		if ghRunID != "" && ghRepo != "" {
			deploymentURL = fmt.Sprintf(
				"https://github.com/%s/actions/runs/%s",
				ghRepo,
				ghRunID,
			)

			logger.Debug().
				Str("deployment_url", deploymentURL).
				Msg("detected deployment url")
		}

		if rr.URL != "" {
			reviewRequest = &rr
		}
	}

	payload := cloud.DeploymentStacksPayloadRequest{
		ReviewRequest: reviewRequest,
	}

	for _, s := range stacks {
		tags := s.Tags
		if tags == nil {
			tags = []string{}
		}
		payload.Stacks = append(payload.Stacks, cloud.DeploymentStackRequest{
			MetaID:            strings.ToLower(s.ID),
			MetaName:          s.Name,
			MetaDescription:   s.Description,
			MetaTags:          tags,
			Repository:        normalizedRepo,
			Path:              s.Dir().String(),
			CommitSHA:         deploymentCommitSHA,
			DeploymentCommand: strings.Join(command, " "),
			DeploymentURL:     deploymentURL,
		})
	}
	res, err := c.cloud.client.CreateDeploymentStacks(ctx, c.cloud.run.orgUUID, c.cloud.run.runUUID, payload)
	if err != nil {
		log.Warn().
			Err(errors.E(err, "failed to create cloud deployment")).
			Msg(DisablingCloudMessage)

		c.cloud.disabled = true
		return
	}

	if len(res) != len(stacks) {
		logger.Warn().Err(errors.E(
			"the backend respond with an invalid number of stacks in the deployment: %d instead of %d",
			len(res), len(stacks)),
		).Msg(DisablingCloudMessage)

		c.cloud.disabled = true
		return
	}

	for _, r := range res {
		logger.Debug().Msgf("deployment created: %+v\n", r)
		if r.StackMetaID == "" {
			fatal(errors.E("backend returned empty meta_id"))
		}
		c.cloud.run.meta2id[r.StackMetaID] = r.StackID
	}
}

func (c *cli) syncCloudDeployment(s *config.Stack, status cloud.Status) {
	logger := log.With().
		Str("organization", c.cloud.run.orgUUID).
		Str("stack", s.RelPath()).
		Stringer("status", status).
		Logger()

	stackID, ok := c.cloud.run.meta2id[s.ID]
	if !ok {
		logger.Error().Msg("unable to update deployment status due to invalid API response")
		return
	}

	payload := cloud.UpdateDeploymentStacks{
		Stacks: []cloud.UpdateDeploymentStack{
			{
				StackID: stackID,
				Status:  status,
			},
		},
	}

	logger.Debug().Msg("updating deployment status")

	ctx, cancel := context.WithTimeout(context.Background(), defaultCloudTimeout)
	defer cancel()
	err := c.cloud.client.UpdateDeploymentStacks(ctx, c.cloud.run.orgUUID, c.cloud.run.runUUID, payload)
	if err != nil {
		logger.Err(err).Str("stack-id", s.ID).Msg("failed to update deployment status for each")
	}
}

func (c *cli) cloudInfo() {
	err := c.setupCloudConfig()
	if err != nil {
		fatal(err)
	}
	if c.cloud.disabled {
		fatal(errors.E("unable to provide credential info"))
	}
	c.cred().Info()
	// verbose info
	c.cloud.output.MsgStdOutV("next token refresh in: %s", time.Until(c.cred().ExpireAt()))
}

func (c *cli) loadCredential() (credential, error) {
	probes := credentialPrecedence(c.output, c.clicfg)
	var cred credential
	var found bool
	for _, probe := range probes {
		var err error
		found, err = probe.Load()
		if err != nil {
			return nil, err
		}
		if found {
			cred = probe
			break
		}
	}
	if !found {
		return nil, errors.E("no credential found")
	}

	return cred, nil
}

func tokenClaims(token string) (jwt.MapClaims, error) {
	jwtParser := &jwt.Parser{}
	tokParsed, _, err := jwtParser.ParseUnverified(token, jwt.MapClaims{})
	if err != nil {
		return nil, errors.E(err, "parsing jwt token")
	}

	if claims, ok := tokParsed.Claims.(jwt.MapClaims); ok {
		return claims, nil
	}
	return nil, errors.E("invalid jwt token claims")
}
