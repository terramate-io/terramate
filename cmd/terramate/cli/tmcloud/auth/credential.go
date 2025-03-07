// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package auth

import (
	"path/filepath"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cmd/terramate/cli/cliconfig"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/printer"
)

const (
	defaultCloudTimeout = 60 * time.Second

	// ErrIDPNeedConfirmation is an error indicating the user has multiple providers set up and
	// linking them is needed.
	ErrIDPNeedConfirmation errors.Kind = "the account was already set up with another email provider"

	// ErrEmailNotVerified is an error indicating that user's email need to be verified.
	ErrEmailNotVerified errors.Kind = "email is not verified"

	// ErrLoginRequired is an error indicating that user has to login to the cloud.
	ErrLoginRequired errors.Kind = "cloud login required"
)

// Credential is the interface for the cloud credentials.
type Credential interface {
	Name() string
	Load() (bool, error)
	Token() (string, error)
	HasExpiration() bool
	IsExpired() bool
	ExpireAt() time.Time

	// private interface

	Organizations() cloud.MemberOrganizations
	Info(selectedOrgName string)
}

// ProbingPrecedence returns the probing precedence for the loading of credentials.
func ProbingPrecedence(printers printer.Printers, client *cloud.Client, clicfg cliconfig.Config) []Credential {
	return []Credential{
		newAPIKey(client),
		newGithubOIDC(printers, client),
		newGitlabOIDC(client),
		newGoogleCredential(printers, clicfg, client),
	}
}

// CredentialFile returns the path to the credential file.
func CredentialFile(clicfg cliconfig.Config) string {
	return filepath.Join(clicfg.UserTerramateDir, credfile)
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

type keyValue struct {
	key   string
	value string
}
