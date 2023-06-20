// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"context"
	stdfmt "fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/hashicorp/go-uuid"
	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cmd/terramate/cli/cliconfig"
	"github.com/terramate-io/terramate/cmd/terramate/cli/out"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
)

// ErrOnboardingIncomplete indicates the onboarding process is incomplete.
const ErrOnboardingIncomplete errors.Kind = "cloud commands cannot be used until onboarding is complete"

type cloudConfig struct {
	client *cloud.Client
	output out.O

	credential credential

	run struct {
		uuid string
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

func (c *cli) checkSyncDeployment() {
	if !c.parsedArgs.Run.CloudSyncDeployment {
		return
	}
	err := c.setupSyncDeployment()
	if err != nil {
		if errors.IsKind(err, ErrOnboardingIncomplete) {
			c.cred().Info()
		}
		fatal(err)
	}

	if orgs := c.cred().organizations(); len(orgs) != 1 {
		fatal(
			errors.E("requires 1 organization associated with the credential but %d found: %s",
				len(orgs),
				orgs),
		)
	}

	if runid := os.Getenv("GITHUB_RUN_ID"); runid != "" {
		c.cloud.run.uuid = runid
	} else {
		c.cloud.run.uuid, err = uuid.GenerateUUID()
		if err != nil {
			fatal(err, "generating run uuid")
		}
	}
}

func (c *cli) createCloudDeployment(stacks config.List[*config.SortableStack], command []string) {
	logger := log.With().Logger()

	if !c.parsedArgs.Run.CloudSyncDeployment {
		return
	}

	logger.Trace().Msg("Checking if selected stacks have id")

	for _, st := range stacks {
		if st.ID == "" {
			fatal(errors.E("The --cloud-sync-deployment flag requires that selected stacks contain an ID field"))
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	repoURL, err := c.prj.git.wrapper.URL(c.prj.gitcfg().DefaultRemote)
	if err == nil {
		u, err := url.Parse(repoURL)
		if err == nil {
			repoURL = path.Join(u.Host, u.Path)
		} else {
			logger.Warn().Err(err).Msgf("failed to parse repository URL: %s", repoURL)
		}
	} else {
		logger.Warn().Err(err).Msg("failed to retrieve repository URL")
	}

	var payload cloud.DeploymentStacksPayloadRequest
	for _, s := range stacks {
		payload.Stacks = append(payload.Stacks, cloud.DeploymentStackRequest{
			MetaID:          s.ID,
			MetaName:        s.Name,
			MetaDescription: s.Description,
			MetaTags:        s.Tags,
			Repository:      repoURL,
			Path:            c.wd(),
			Command:         strings.Join(command, " "),
		})
	}
	res, err := c.cloud.client.CreateDeploymentStacks(ctx, "0000-1111-2222-3333", c.cloud.run.uuid, payload)
	if err != nil {
		fatal(err)
	}

	for _, r := range res {
		stdfmt.Printf("response: %+v\n", r)
	}
}
func (c *cli) setupSyncDeployment() error {
	cred, err := c.loadCredential()
	if err != nil {
		return err
	}

	c.cloud = cloudConfig{
		client: &cloud.Client{
			BaseURL:    cloudBaseURL,
			HTTPClient: &http.Client{},
			Credential: cred,
		},
		output:     c.output,
		credential: cred,
	}

	return cred.Validate(c.cloud)
}

func (c *cli) cloudInfo() {
	err := c.setupSyncDeployment()
	if err != nil {
		fatal(err)
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
