// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package telemetry

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate"
	"github.com/terramate-io/terramate/errors"
)

// PlatformType is the CI/CD platform.
type PlatformType int

// nolint:revive
const (
	// PlatformLocal represents the local user environment.
	PlatformLocal PlatformType = iota
	PlatformGithub
	PlatformGitlab
	PlatformGenericCI
	PlatformAppVeyor
	PlatformAzureDevops
	PlatformBamboo
	PlatformBitBucket
	PlatformBuddyWorks
	PlatformBuildKite
	PlatformCircleCI
	PlatformCirrus
	PlatformCodeBuild
	PlatformHeroku
	PlatformHudson
	PlatformJenkins
	PlatformMyGet
	PlatformTeamCity
	PlatformTravis
)

// AuthType is the authentication method that was used.
type AuthType int

const (
	// AuthNone represents no authentication.
	AuthNone AuthType = iota
	// AuthIDPGoogle represents Google IDP authentication.
	AuthIDPGoogle
	// AuthIDPGithub represents GitHub IDP authentication.
	AuthIDPGithub
	// AuthOIDCGithub represents GitHub OIDC authentication.
	AuthOIDCGithub
	// AuthOIDCGitlab represents GitLab OIDC authentication.
	AuthOIDCGitlab
	// AuthAPIKey represents API key authentication.
	AuthAPIKey
)

// Message is the analytics data that will be collected.
type Message struct {
	Platform PlatformType `json:"platform,omitempty"`
	Auth     AuthType     `json:"auth,omitempty"`

	Signature string `json:"signature,omitempty"`
	OrgName   string `json:"org_name,omitempty"`
	OrgUUID   string `json:"org_uuid,omitempty"`

	Arch string `json:"arch,omitempty"`
	OS   string `json:"os,omitempty"`

	// Command stores the invoked command.
	Command string `json:"command,omitempty"`

	// Details stores features or flags used by the command.
	Details []string `json:"details,omitempty"`
}

// DetectPlatformFromEnv detects PlatformType based on environment variables.
func DetectPlatformFromEnv() PlatformType {
	if isEnvVarSet("GITHUB_ACTIONS") {
		return PlatformGithub
	} else if isEnvVarSet("GITLAB_CI") {
		return PlatformGitlab
	} else if isEnvVarSet("BITBUCKET_BUILD_NUMBER") {
		return PlatformBitBucket
	} else if isEnvVarSet("TF_BUILD") {
		return PlatformAzureDevops
	} else if isEnvVarSet("APPVEYOR") {
		return PlatformAppVeyor
	} else if isEnvVarSet("bamboo.buildKey") {
		return PlatformBamboo
	} else if isEnvVarSet("BUDDY") {
		return PlatformBuddyWorks
	} else if isEnvVarSet("BUILDKITE") {
		return PlatformBuildKite
	} else if isEnvVarSet("CIRCLECI") {
		return PlatformCircleCI
	} else if isEnvVarSet("CIRRUS_CI") {
		return PlatformCirrus
	} else if isEnvVarSet("CODEBUILD_CI") {
		return PlatformCodeBuild
	} else if isEnvVarSet("HEROKU_TEST_RUN_ID") {
		return PlatformHeroku
	} else if strings.HasPrefix(os.Getenv("BUILD_TAG"), "hudson-") {
		return PlatformHudson
	} else if isEnvVarSet("JENKINS_URL") {
		return PlatformJenkins
	} else if os.Getenv("BuildRunner") == "MyGet" {
		return PlatformMyGet
	} else if isEnvVarSet("TEAMCITY_VERSION") {
		return PlatformTeamCity
	} else if isEnvVarSet("TRAVIS") {
		return PlatformTravis
	} else if isEnvVarSet("CI") {
		return PlatformGenericCI
	}
	return PlatformLocal
}

// DetectAuthTypeFromEnv detects AuthType based on environment variables and credentials.
func DetectAuthTypeFromEnv(credpath string) AuthType {
	if isEnvVarSet("ACTIONS_ID_TOKEN_REQUEST_TOKEN") {
		return AuthOIDCGithub
	} else if isEnvVarSet("TM_GITLAB_ID_TOKEN") {
		return AuthOIDCGitlab
	}
	return getAuthProviderFromCredentials(credpath)
}

// ReadSignature parses a signature file. It works for checkpoint and analytics signatures as both use the same format.
func ReadSignature(p string) string {
	sigBytes, err := os.ReadFile(p)
	if err == nil {
		lines := strings.SplitN(string(sigBytes), "\n", 2)
		if len(lines) > 0 {
			return strings.TrimSpace(lines[0])
		}
	}
	return ""
}

// GenerateOrReadSignature attempts to read the analytics signature.
// If not present, it will create a new one.
func GenerateOrReadSignature(cpsigfile, anasigfile string) (string, bool) {
	logger := log.With().
		Str("action", "GenerateOrReadSignature()").
		Logger()

	// Try reading existing signature.
	anasig := ReadSignature(anasigfile)
	if anasig != "" {
		return anasig, false
	}

	// Create a new one. Bootstrap from checkpoint signature, otherwise create a new one.
	var newsig string
	cpsig := ReadSignature(cpsigfile)
	if cpsig != "" {
		newsig = cpsig
	} else {
		newsig = GenerateSignature()
	}

	if err := os.MkdirAll(filepath.Dir(anasigfile), 0755); err != nil {
		logger.Debug().Err(err).Msg("failed to create directory for signature file")
	}
	if err := os.WriteFile(anasigfile, []byte(newsig+"\n\n"+userMessage+"\n"), 0644); err != nil {
		logger.Debug().Err(err).Msg("failed to save signature file")
	}

	return newsig, true
}

// GenerateSignature generates a new random signature.
func GenerateSignature() string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

func isEnvVarSet(key string) bool {
	val := os.Getenv(key)
	return val != "" && val != "0" && val != "false"
}

func getAuthProviderFromCredentials(credfile string) AuthType {
	_, err := os.Lstat(credfile)
	if err != nil {
		return AuthNone
	}
	contents, err := os.ReadFile(credfile)
	if err != nil {
		return AuthNone
	}

	var providerProbe struct {
		Provider string `json:"provider"`
	}
	err = json.Unmarshal(contents, &providerProbe)
	if err != nil {
		return AuthNone
	}

	switch providerProbe.Provider {
	case "Google":
		return AuthIDPGoogle
	case "GitHub":
		return AuthIDPGithub
	default:
		// Not handling cases like unknown or invalid values.
		return AuthNone
	}
}

// SendMessageParams contains parameters for SendMessage.
type SendMessageParams struct {
	Endpoint *url.URL
	Client   *http.Client
	Version  string
	Timeout  time.Duration
}

// SendMessage sends an analytics message to the backend endpoint asynchronously.
// It returns a channel that will receive the result of the operation when it's done.
func SendMessage(msg *Message, p SendMessageParams) <-chan error {
	if p.Endpoint == nil {
		url := Endpoint()
		p.Endpoint = &url
	}
	if p.Client == nil {
		p.Client = http.DefaultClient
	}
	if p.Version == "" {
		p.Version = terramate.Version()
	}
	if p.Timeout == 0 {
		p.Timeout = time.Millisecond * 100
	}

	rch := make(chan error, 1)
	go func() {
		rch <- doSendMessage(msg, p)
		close(rch)
	}()
	return rch
}

func doSendMessage(msg *Message, p SendMessageParams) error {
	ctx, cancel := context.WithTimeout(context.Background(), p.Timeout)
	defer cancel()

	buf, err := json.Marshal(&msg)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.Endpoint.String(), bytes.NewReader(buf))
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", "terramate/v"+terramate.Version())
	req.Header.Set("Content-Type", "application/json")
	errs := errors.L()
	resp, err := p.Client.Do(req)
	errs.Append(err)
	if err == nil {
		errs.Append(resp.Body.Close())
	}
	return errs.AsError()
}

// userMessage is suffixed to the uid file.
const userMessage = `
This is a randomly generated ID used to aggregate analytics data.
`
