// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cliauth

import (
	"fmt"
	"net/http"

	"github.com/pkg/browser"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/ui/tui/cliconfig"
)

const (
	ssoProviderID = "oidc.workos"
	ssoOauthScope = "openid"
)

// SSOLogin logs in the user using SingleSignOn.
func SSOLogin(printers printer.Printers, verbosity int, orgid string, clicfg cliconfig.Config) error {
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

	if verbosity > 0 {
		printers.Stdout.Println(fmt.Sprintf("trying to open URL in the browser: %s", consentData.AuthURI))
	}

	err = browser.OpenURL(consentData.AuthURI)
	if err != nil {
		printers.Stderr.Println("failed to open URL in the browser")
		printers.Stderr.Println(fmt.Sprintf("Please visit the url: %s", consentData.AuthURI))
	} else {
		printers.Stdout.Println("Please continue the authentication process in the browser.")
	}

	select {
	case cred := <-h.credentialChan:
		return saveCredential(printers, verbosity, "SSO", cred, clicfg)
	case err := <-h.errChan:
		return err.err
	}
}
