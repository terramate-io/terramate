// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package auth

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/terramate-io/terramate/cmd/terramate/cli/cliconfig"
	"github.com/terramate-io/terramate/cmd/terramate/cli/github"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/printer"
)

const defaultGitHubClientID = "08e1f8d6f599c7ec48c5"

// GithubLogin logs in the user using GitHub.
func GithubLogin(printers printer.Printers, tmcBaseURL string, clicfg cliconfig.Config) error {
	token, err := githubAuth()
	if err != nil {
		return err
	}

	postBody := url.Values{
		"access_token": []string{token},
		"providerId":   []string{"github.com"},
	}

	reqPayload := googleSignInPayload{
		PostBody:            postBody.Encode(),
		RequestURI:          tmcBaseURL + "/__/auth/handler",
		ReturnIdpCredential: true,
		ReturnSecureToken:   true,
	}

	cred, err := signInWithIDP(reqPayload, idpkey())
	if err != nil {
		return err
	}

	printers.Stdout.Println(fmt.Sprintf("Logged in as %s", cred.UserDisplayName()))
	printers.Stdout.Println(fmt.Sprintf("Token: %s", cred.IDToken))
	expire, _ := strconv.Atoi(cred.ExpiresIn)
	printers.Stdout.Println(fmt.Sprintf("Expire at: %s", time.Now().Add(time.Second*time.Duration(expire)).Format(time.RFC822Z)))
	return saveCredential(printers, cred, clicfg)
}

func githubAuth() (string, error) {
	oauthCtx, err := github.OAuthDeviceFlowAuthStart(ghClientID())
	if err != nil {
		return "", err
	}

	printer.Stdout.Println(fmt.Sprintf("Please visit: %s", oauthCtx.VerificationURI))
	printer.Stdout.Println(fmt.Sprintf("and enter code: %s", oauthCtx.UserCode))

	for {
		var token string
		token, err = oauthCtx.ProbeAuthState()
		if err == nil {
			return token, nil
		}

		var errInfo *errors.Error
		if !errors.As(err, &errInfo) {
			return "", err // unexpected err
		}

		interval := time.Duration(oauthCtx.Interval) * time.Second

		switch errInfo.Kind {
		case github.ErrDeviceFlowSlowDown:
			interval += 5 * time.Second
			fallthrough
		case github.ErrDeviceFlowAuthPending:
			time.Sleep(interval)
		default:
			return "", err
		}
	}
}

func ghClientID() string {
	idpKey := os.Getenv("TMC_API_GITHUB_CLIENT_ID")
	if idpKey == "" {
		idpKey = defaultGitHubClientID
	}
	return idpKey
}
