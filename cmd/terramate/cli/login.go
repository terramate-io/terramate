// Copyright 2023 Mineiros GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/mineiros-io/terramate/cmd/terramate/cli/cliconfig"
	"github.com/mineiros-io/terramate/cmd/terramate/cli/out"
	"github.com/mineiros-io/terramate/errors"
	"github.com/pkg/browser"
	"github.com/rs/zerolog/log"
)

const (
	// that's a public key.
	apiKey = "AIzaSyDeCYIgqEhufsnBGtlNu4fv1alvpcs1Nos"

	// do not use localhost here.
	// see recommendation here: https://datatracker.ietf.org/doc/html/rfc8252#section-7.3
	serverAddr = "localhost:8080"
	serverURL  = "http://" + serverAddr + "/auth"

	credfile = "credentials.tmrc.json"
)

type (
	createAuthURIResponse struct {
		Kind       string `json:"kind"`
		AuthURI    string `json:"authUri"`
		ProviderID string `json:"providerId"`
		SessionID  string `json:"sessionId"`
	}

	credentialInfo struct {
		ProviderID       string `json:"providerId"`
		Email            string `json:"email"`
		DisplayName      string `json:"displayName"`
		LocalID          string `json:"localId"`
		IDToken          string `json:"idToken"`
		Context          string `json:"context"`
		OauthAccessToken string `json:"oauthAccessToken"`
		OauthExpireIn    int    `json:"oauthExpireIn"`
		RefreshToken     string `json:"refreshToken"`
		ExpiresIn        string `json:"expiresIn"`
		OauthIDToken     string `json:"oauthIdToken"`
		RawUserInfo      string `json:"rawUserInfo"`
	}

	cachedCredential struct {
		DisplayName             string    `json:"display_name"`
		CachedAt                time.Time `json:"cached_at"`
		IDToken                 string    `json:"id_token"`
		IDTokenExpiresInSeconds int       `json:"id_token_expires_in_seconds"`
		RefreshToken            string    `json:"refresh_token"`
	}
)

func login(output out.O, clicfg cliconfig.Config) error {
	consentData, err := createAuthURI()
	if err != nil {
		return err
	}

	output.MsgStdOutV("trying to open URL in the browser: %s", consentData.AuthURI)

	err = browser.OpenURL(consentData.AuthURI)
	if err != nil {
		output.MsgStdErr("failed to open URL in the browser")
		output.MsgStdOut("Please visit the url: %s", consentData.AuthURI)
	} else {
		output.MsgStdOut("Please continue the authentication process in the browser.")
	}

	h := newHandler(consentData)

	mux := http.NewServeMux()
	mux.Handle("/auth", h)

	s := http.Server{
		Addr:    serverAddr,
		Handler: mux,
	}

	go func() {
		err := s.ListenAndServe()
		if err != nil {
			h.errChan <- err
			return
		}
	}()

	select {
	case cred := <-h.credentialChan:
		output.MsgStdOut("Logged in as %s", cred.DisplayName)
		output.MsgStdOutV("Token: %s", cred.IDToken)
		expire, _ := strconv.Atoi(cred.ExpiresIn)
		output.MsgStdOutV("Expire at: %s", time.Now().Add(time.Second*time.Duration(expire)).Format(time.RFC822Z))
		return cacheToken(output, cred, clicfg)
	case err := <-h.errChan:
		return err
	}
}

func createAuthURI() (createAuthURIResponse, error) {
	const endpoint = "https://www.googleapis.com/identitytoolkit/v3/relyingparty/createAuthUri"
	const authScope = `{"google.com": "profile"}`

	type payload struct {
		ProviderID      string                 `json:"providerId"`
		ContinueURI     string                 `json:"continueUri"`
		CustomParameter map[string]interface{} `json:"customParameter"`
		OauthScope      string                 `json:"oauthScope"`
	}

	payloadData := payload{
		ProviderID:      "google.com",
		ContinueURI:     serverURL,
		CustomParameter: map[string]interface{}{},
		OauthScope:      authScope,
	}

	postBody, err := json.Marshal(&payloadData)
	if err != nil {
		return createAuthURIResponse{}, errors.E(err)
	}

	url := endpointURL(endpoint)
	req, err := http.NewRequest("POST", url.String(), bytes.NewBuffer(postBody))
	if err != nil {
		return createAuthURIResponse{}, errors.E(err, "failed to create authentication url")
	}

	req.Header.Add("content-type", "application/json")

	logger := log.With().
		Str("action", "createAuthURI()").
		Logger()

	logger.Debug().
		Str("url", req.URL.String()).
		Msg("sending request")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return createAuthURIResponse{}, errors.E(err, "failed to start authentication process")
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return createAuthURIResponse{}, errors.E(err)
	}

	logger.Trace().
		Str("response-body", string(data)).
		Int("response-status-code", resp.StatusCode).
		Msg("response")

	if resp.StatusCode != 200 {
		return createAuthURIResponse{}, errors.E("%s request returned %d", req.URL, resp.StatusCode)
	}

	var respURL createAuthURIResponse
	err = json.Unmarshal(data, &respURL)
	if err != nil {
		return createAuthURIResponse{}, errors.E(err, "failed to unmarshal response")
	}

	return respURL, nil
}

type tokenHandler struct {
	sync.Mutex

	complete       bool
	consentData    createAuthURIResponse
	errChan        chan error
	credentialChan chan credentialInfo
}

func newHandler(consent createAuthURIResponse) *tokenHandler {
	return &tokenHandler{
		consentData:    consent,
		credentialChan: make(chan credentialInfo),
		errChan:        make(chan error),
	}
}

func (h *tokenHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.Lock()
	defer func() {
		h.complete = true
		h.Unlock()
	}()

	// handles only 1 request

	if h.complete {
		return
	}

	gotURL := "http://" + serverAddr + r.URL.String()

	const signInEndpoint = "https://identitytoolkit.googleapis.com/v1/accounts:signInWithIdp"

	type payload struct {
		RequestURI          string `json:"requestUri"`
		SessionID           string `json:"sessionId"`
		ReturnSecureToken   bool   `json:"returnSecureToken"`
		ReturnIdpCredential bool   `json:"returnIdpCredential"`
	}

	postBody := payload{
		RequestURI:          gotURL,
		SessionID:           h.consentData.SessionID,
		ReturnSecureToken:   true,
		ReturnIdpCredential: true,
	}

	data, err := json.Marshal(&postBody)
	if err != nil {
		h.handleErr(w, errors.E(err))
		return
	}

	logger := log.With().
		Str("action", "tokenHandler.ServeHTTP").
		Logger()

	logger.Trace().
		Str("endpoint", signInEndpoint).
		Str("post-body", string(data)).
		Msg("prepared request body")

	url := endpointURL(signInEndpoint)
	req, err := http.NewRequest("POST", url.String(), bytes.NewBuffer(data))
	if err != nil {
		h.handleErr(w, errors.E(err, "failed to create authentication url"))
		return
	}

	req.Header.Add("content-type", "application/json")

	logger.Debug().
		Str("url", req.URL.String()).
		Msg("sending request")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		h.handleErr(w, errors.E(err, "failed to start authentication process"))
		return
	}

	data, err = io.ReadAll(resp.Body)
	if err != nil {
		h.errChan <- errors.E(err)
		return
	}

	logger.Trace().
		Str("response-body", string(data)).
		Int("response-status-code", resp.StatusCode).
		Msg("response")

	if resp.StatusCode != 200 {
		h.handleErr(w, errors.E("%s request returned %d", req.URL, resp.StatusCode))
		return
	}

	var creds credentialInfo
	err = json.Unmarshal(data, &creds)
	if err != nil {
		h.handleErr(w, errors.E(err))
		return
	}

	h.handleOK(w, creds)
}

func (h *tokenHandler) handleOK(w http.ResponseWriter, cred credentialInfo) {
	const messageFormat = `
	<html>
		<head>
			<title>Terramate Cloud Login Succeed</title>
		</head>
		<body>
			<h2>Successfully authenticated as %s</h2>
			<p>You can close this page now.</p>
		</body>
	</html>
	`
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(fmt.Sprintf(messageFormat, html.EscapeString(cred.DisplayName))))
	h.credentialChan <- cred
}

func (h *tokenHandler) handleErr(w http.ResponseWriter, err error) {
	const errMessage = `
	<html>
		<head>
			<title>Terramate Cloud Login Failed</title>
		</head>
		<body>
			<h2>Terramate Cloud Login Failed</h2>
			<p>Please, go back to Terminal and try again</p>
		</body>
	</html>
	`
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = w.Write([]byte(errMessage))
	h.errChan <- err
}

func cacheToken(output out.O, cred credentialInfo, clicfg cliconfig.Config) error {
	cachedAt := time.Now()
	cacheFile := filepath.Join(clicfg.HomeTerramateDir, credfile)

	expiresIn, err := strconv.Atoi(cred.ExpiresIn)
	if err != nil {
		return errors.E("authentication returned an invalid expiration time: %v", err)
	}

	cachePayload := cachedCredential{
		DisplayName:             cred.DisplayName,
		IDToken:                 cred.IDToken,
		IDTokenExpiresInSeconds: expiresIn,
		RefreshToken:            cred.RefreshToken,
		CachedAt:                cachedAt,
	}

	data, err := json.Marshal(&cachePayload)
	if err != nil {
		return errors.E(err, "failed to JSON marshal the credentials")
	}

	err = os.WriteFile(cacheFile, data, 0600)
	if err != nil {
		return errors.E(err, "failed to cache credentials")
	}

	output.MsgStdOutV("credentials cached at %s", cacheFile)
	return nil
}

func endpointURL(endpoint string) *url.URL {
	u, err := url.Parse(endpoint)
	if err != nil {
		fatal(err, "failed to parse endpoint URL for createAuthURI")
	}

	q := u.Query()
	q.Add("key", apiKey)
	u.RawQuery = q.Encode()
	return u
}
