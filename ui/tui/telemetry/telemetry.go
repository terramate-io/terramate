// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package telemetry

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate"
	"github.com/terramate-io/terramate/ci"
	"github.com/terramate-io/terramate/errors"
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
	Platform ci.PlatformType `json:"platform,omitempty"`
	// PlatformUser is a platform-specific identifier.
	PlatformUser string `json:"platform_user,omitempty"`

	Auth AuthType `json:"auth,omitempty"`
	// AuthUser is the TMC user UUID.
	AuthUser string `json:"auth_user,omitempty"`

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
	Product  string
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
	if p.Product == "" {
		p.Product = "terramate"
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

	req.Header.Set("User-Agent", fmt.Sprintf("%s/v%s", p.Product, p.Version))
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
