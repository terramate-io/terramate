// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package auth

import (
	"net/http"

	"github.com/pkg/browser"
	"github.com/terramate-io/terramate/cmd/terramate/cli/cliconfig"
	"github.com/terramate-io/terramate/cmd/terramate/cli/out"
)

const (
	ssoProviderID = "oidc.workos"
	ssoOauthScope = "openid"
)

// SSOLogin logs in the user using SingleSignOn.
func SSOLogin(output out.O, orgid string, clicfg cliconfig.Config) error {
	h := &tokenHandler{
		credentialChan: make(chan credentialInfo),
		errChan:        make(chan tokenError),
		idpKey:         idpkey(),
	}

	mux := http.NewServeMux()
	mux.Handle("/auth", h)

	s := &http.Server{
		Handler: mux,
	}

	redirectURLChan := make(chan string)
	consentDataChan := make(chan createAuthURIResponse)

	var ssoPorts = []int{52023}

	go startServer(s, h, ssoPorts, redirectURLChan, consentDataChan)

	consentData, err := createAuthURI(
		ssoProviderID,
		ssoOauthScope,
		<-redirectURLChan,
		h.idpKey,
		map[string]any{
			"organization": orgid,
		},
	)
	if err != nil {
		return err
	}

	consentDataChan <- consentData

	output.MsgStdOutV("trying to open URL in the browser: %s", consentData.AuthURI)

	err = browser.OpenURL(consentData.AuthURI)
	if err != nil {
		output.MsgStdErr("failed to open URL in the browser")
		output.MsgStdOut("Please visit the url: %s", consentData.AuthURI)
	} else {
		output.MsgStdOut("Please continue the authentication process in the browser.")
	}

	select {
	case cred := <-h.credentialChan:
		return saveCredential(output, "SSO", cred, clicfg)
	case err := <-h.errChan:
		return err.err
	}
}
