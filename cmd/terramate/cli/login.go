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
	"sync"

	"github.com/mineiros-io/terramate/cmd/terramate/cli/out"
	"github.com/mineiros-io/terramate/errors"
	"github.com/rs/zerolog/log"
)

const (
	apiKey = "AIzaSyDeCYIgqEhufsnBGtlNu4fv1alvpcs1Nos"

	serverAddr = "localhost:8080"
	serverURL  = "http://" + serverAddr + "/auth"
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
)

func login(output out.O) error {
	consentData, err := createAuthURI()
	if err != nil {
		return err
	}

	output.MsgStdOut("Please visit the url: %s", consentData.AuthURI)

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
		output.MsgStdOut("Logged in as: ; %s", cred.DisplayName)
		return nil
	case err := <-h.errChan:
		return err
	}
}

func createAuthURI() (createAuthURIResponse, error) {
	const endpoint = "https://www.googleapis.com/identitytoolkit/v3/relyingparty/createAuthUri"

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
		OauthScope:      `{"google.com": "profile"}`,
	}

	postBody, err := json.Marshal(&payloadData)
	if err != nil {
		return createAuthURIResponse{}, errors.E(err)
	}
	req, err := http.NewRequest("POST", endpoint+"?key="+apiKey, bytes.NewBuffer(postBody))
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

	req, err := http.NewRequest("POST", signInEndpoint+"?key="+apiKey, bytes.NewBuffer(data))
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
	w.WriteHeader(http.StatusInternalServerError)
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
