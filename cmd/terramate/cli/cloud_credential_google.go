// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"bytes"
	"context"
	stdjson "encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/pkg/browser"
	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cmd/terramate/cli/cliconfig"
	"github.com/terramate-io/terramate/cmd/terramate/cli/clitest"
	"github.com/terramate-io/terramate/cmd/terramate/cli/out"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/printer"
)

const (
	// that's a public key.
	defaultAPIKey = "AIzaSyDeCYIgqEhufsnBGtlNu4fv1alvpcs1Nos"

	credfile = "credentials.tmrc.json"

	minPort = 40000
	maxPort = 52023
)

type (
	googleCredential struct {
		mu sync.RWMutex

		idpKey       string
		token        string
		refreshToken string
		jwtClaims    jwt.MapClaims
		expireAt     time.Time
		email        string
		orgs         cloud.MemberOrganizations
		user         cloud.User

		output out.O
		clicfg cliconfig.Config
		client *cloud.Client
	}

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
		IDToken      string `json:"id_token"`
		RefreshToken string `json:"refresh_token"`
	}
)

func googleLogin(output out.O, idpKey string, clicfg cliconfig.Config) error {
	h := &tokenHandler{
		credentialChan: make(chan credentialInfo),
		errChan:        make(chan error),
		idpKey:         idpKey,
	}

	mux := http.NewServeMux()
	mux.Handle("/auth", h)

	s := &http.Server{
		Handler: mux,
	}

	redirectURLChan := make(chan string)
	consentDataChan := make(chan createAuthURIResponse)

	go startServer(s, h, redirectURLChan, consentDataChan)

	consentData, err := createAuthURI(<-redirectURLChan, idpKey)
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
		output.MsgStdOut("Logged in as %s", cred.DisplayName)
		output.MsgStdOutV("Token: %s", cred.IDToken)
		expire, _ := strconv.Atoi(cred.ExpiresIn)
		output.MsgStdOutV("Expire at: %s", time.Now().Add(time.Second*time.Duration(expire)).Format(time.RFC822Z))
		return saveCredential(output, cred, clicfg)
	case err := <-h.errChan:
		return err
	}
}

func startServer(
	s *http.Server,
	h *tokenHandler,
	redirectURLChan chan<- string,
	consentDataChan <-chan createAuthURIResponse,
) {
	var err error
	defer func() {
		if err != nil {
			h.errChan <- err
		}
	}()

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	var ln net.Listener
	const maxretry = 5
	var retry int
	for retry = 0; retry < maxretry; retry++ {
		addr := "127.0.0.1:" + strconv.Itoa(minPort+rng.Intn(maxPort-minPort))
		s.Addr = addr

		ln, err = net.Listen("tcp", addr)
		if err == nil {
			break
		}
	}

	if retry == maxretry {
		err = errors.E(err, "failed to find an available port, please try again")
		return
	}

	redirectURL := "http://" + s.Addr + "/auth"

	redirectURLChan <- redirectURL
	h.consentData = <-consentDataChan
	h.continueURL = redirectURL
	err = s.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		err = nil
	}
}

func createAuthURI(continueURI string, idpKey string) (createAuthURIResponse, error) {
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
		ContinueURI:     continueURI,
		CustomParameter: map[string]interface{}{},
		OauthScope:      authScope,
	}

	postBody, err := stdjson.Marshal(&payloadData)
	if err != nil {
		return createAuthURIResponse{}, errors.E(err)
	}

	url := endpointURL(endpoint, idpKey)
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
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return createAuthURIResponse{}, errors.E(err)
	}

	if resp.StatusCode != 200 {
		return createAuthURIResponse{}, errors.E("%s request returned %d", req.URL, resp.StatusCode)
	}

	var respURL createAuthURIResponse
	err = stdjson.Unmarshal(data, &respURL)
	if err != nil {
		return createAuthURIResponse{}, errors.E(err, "failed to unmarshal response")
	}

	return respURL, nil
}

type tokenHandler struct {
	sync.Mutex

	complete       bool
	consentData    createAuthURIResponse
	continueURL    string
	errChan        chan error
	credentialChan chan credentialInfo
	idpKey         string
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

	gotURL, _ := url.Parse(h.continueURL)
	gotURL.RawQuery = r.URL.Query().Encode()

	const signInEndpoint = "https://identitytoolkit.googleapis.com/v1/accounts:signInWithIdp"

	type payload struct {
		RequestURI          string `json:"requestUri"`
		SessionID           string `json:"sessionId"`
		ReturnSecureToken   bool   `json:"returnSecureToken"`
		ReturnIdpCredential bool   `json:"returnIdpCredential"`
	}

	postBody := payload{
		RequestURI:          gotURL.String(),
		SessionID:           h.consentData.SessionID,
		ReturnSecureToken:   true,
		ReturnIdpCredential: true,
	}

	data, err := stdjson.Marshal(&postBody)
	if err != nil {
		h.handleErr(w, errors.E(err))
		return
	}

	logger := log.With().
		Str("action", "tokenHandler.ServeHTTP").
		Logger()

	url := endpointURL(signInEndpoint, h.idpKey)
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
	defer func() { _ = resp.Body.Close() }()

	data, err = io.ReadAll(resp.Body)
	if err != nil {
		h.errChan <- errors.E(err)
		return
	}

	if resp.StatusCode != 200 {
		h.handleErr(w, errors.E("%s request returned %d", req.URL, resp.StatusCode))
		return
	}

	var creds credentialInfo
	err = stdjson.Unmarshal(data, &creds)
	if err != nil {
		h.handleErr(w, errors.E(err))
		return
	}

	h.handleOK(w, creds)
}

func (h *tokenHandler) handleOK(w http.ResponseWriter, cred credentialInfo) {
	h.credentialChan <- cred

	w.Header().Add("Location", "https://cloud.terramate.io/cli/signed-in")
	w.WriteHeader(http.StatusSeeOther)
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

func saveCredential(output out.O, cred credentialInfo, clicfg cliconfig.Config) error {
	cachePayload := cachedCredential{
		IDToken:      cred.IDToken,
		RefreshToken: cred.RefreshToken,
	}

	data, err := stdjson.Marshal(&cachePayload)
	if err != nil {
		return errors.E(err, "failed to JSON marshal the credentials")
	}

	credfile := filepath.Join(clicfg.UserTerramateDir, credfile)
	err = os.WriteFile(credfile, data, 0600)
	if err != nil {
		return errors.E(err, "failed to cache credentials")
	}

	output.MsgStdOutV("credentials cached at %s", credfile)
	return nil
}

func loadCredential(output out.O, clicfg cliconfig.Config) (cachedCredential, bool, error) {
	credFile := filepath.Join(clicfg.UserTerramateDir, credfile)
	_, err := os.Lstat(credFile)
	if err != nil {
		return cachedCredential{}, false, nil
	}
	contents, err := os.ReadFile(credFile)
	if err != nil {
		return cachedCredential{}, true, err
	}
	var cred cachedCredential
	err = stdjson.Unmarshal(contents, &cred)
	if err != nil {
		return cachedCredential{}, true, err
	}
	output.MsgStdOutV("credentials loaded from %s", credFile)
	return cred, true, nil
}

func endpointURL(endpoint string, idpKey string) *url.URL {
	u, err := url.Parse(endpoint)
	if err != nil {
		fatal("failed to parse endpoint URL for createAuthURI", err)
	}

	q := u.Query()
	q.Add("key", idpKey)
	u.RawQuery = q.Encode()
	return u
}

func newGoogleCredential(
	output out.O,
	idpKey string,
	clicfg cliconfig.Config,
	client *cloud.Client,
) *googleCredential {
	return &googleCredential{
		output: output,
		clicfg: clicfg,
		idpKey: idpKey,
		client: client,
	}
}

func (g *googleCredential) Load() (bool, error) {
	credinfo, found, err := loadCredential(g.output, g.clicfg)
	if err != nil {
		return false, err
	}

	if !found {
		return false, nil
	}

	err = g.update(credinfo.IDToken, credinfo.RefreshToken)
	if err != nil {
		return true, err
	}
	g.client.Credential = g
	return true, g.fetchDetails()
}

func (g *googleCredential) Name() string {
	return "Google Social Provider"
}

func (g *googleCredential) IsExpired() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return time.Now().After(g.expireAt)
}

func (g *googleCredential) ExpireAt() time.Time {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.expireAt
}

func (g *googleCredential) Refresh() (err error) {
	if g.token != "" {
		g.output.MsgStdOutV("refreshing token...")

		defer func() {
			if err == nil {
				g.output.MsgStdOutV("token successfully refreshed.")
				g.output.MsgStdOutV("next token refresh in: %s", time.Until(g.ExpireAt()))
			}
		}()
	}

	const oidcTimeout = 3 // seconds
	const refreshTokenURL = "https://securetoken.googleapis.com/v1/token"

	type RequestBody struct {
		GrantType    string `json:"grant_type"`
		RefreshToken string `json:"refresh_token"`
	}

	ctx, cancel := context.WithTimeout(context.Background(), oidcTimeout*time.Second)
	defer cancel()

	endpoint := endpointURL(refreshTokenURL, g.idpKey)
	reqPayload := RequestBody{
		GrantType:    "refresh_token",
		RefreshToken: g.refreshToken,
	}

	payloadData, err := stdjson.Marshal(reqPayload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint.String(), bytes.NewBuffer(payloadData))
	if err != nil {
		return err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer func() {
		err = resp.Body.Close()
		if err != nil {
			g.output.MsgStdErrV("failed to close response body: %v", err)
		}
	}()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	type response struct {
		IDToken      string `json:"id_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    string `json:"expires_in"`
		TokenType    string `json:"token_type"`
		UserID       string `json:"user_id"`
		ProjectID    string `json:"project_id"`
	}

	var tokresp response
	err = stdjson.Unmarshal(data, &tokresp)
	if err != nil {
		return err
	}

	err = g.update(tokresp.IDToken, g.refreshToken)
	if err != nil {
		return err
	}

	return saveCredential(g.output, credentialInfo{
		IDToken:      g.token,
		RefreshToken: g.refreshToken,
	}, g.clicfg)
}

func (g *googleCredential) Claims() jwt.MapClaims {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.jwtClaims
}

func (g *googleCredential) DisplayClaims() []keyValue {
	return []keyValue{
		{
			key:   "email",
			value: g.email,
		},
	}
}

func (g *googleCredential) Token() (string, error) {
	if g.IsExpired() {
		err := g.Refresh()
		if err != nil {
			return "", err
		}
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.token, nil
}

func (g *googleCredential) fetchDetails() error {
	var (
		err  error
		user cloud.User
		orgs cloud.MemberOrganizations
	)

	func() {
		ctx, cancel := context.WithTimeout(context.Background(), defaultGoogleTimeout)
		defer cancel()
		orgs, err = g.client.MemberOrganizations(ctx)
	}()

	if err != nil {
		return err
	}

	func() {
		ctx, cancel := context.WithTimeout(context.Background(), defaultGoogleTimeout)
		defer cancel()
		user, err = g.client.Users(ctx)
	}()

	if err != nil {
		if errors.IsKind(err, cloud.ErrNotFound) {
			return errors.E(clitest.ErrCloudOnboardingIncomplete)
		}
		return err
	}
	g.orgs = orgs
	g.user = user
	return nil
}

// info display the credential details.
func (g *googleCredential) info(selectedOrgName string) {
	printer.Stdout.Println("status: signed in")
	printer.Stdout.Println(fmt.Sprintf("provider: %s", g.Name()))

	if g.user.DisplayName != "" {
		printer.Stdout.Println(fmt.Sprintf("user: %s", g.user.DisplayName))
	}

	for _, kv := range g.DisplayClaims() {
		printer.Stdout.Println(fmt.Sprintf("%s: %s", kv.key, kv.value))
	}

	if len(g.orgs) > 0 {
		printer.Stdout.Println(fmt.Sprintf("organizations: %s", g.orgs))
	}

	if selectedOrgName == "" && len(g.orgs) > 1 {
		printer.Stderr.Warn("User is member of multiple organizations but none was selected")
	}

	if g.user.DisplayName == "" {
		printer.Stderr.Warn("On-boarding is incomplete. Please visit cloud.terramate.io to complete on-boarding.")
	}

	if len(g.orgs) == 0 {
		printer.Stderr.Warn("You are not part of an organization. Please visit cloud.terramate.io to create an organization.")
	}
}

// organizations returns the list of organizations associated with the credential.
func (g *googleCredential) organizations() cloud.MemberOrganizations {
	return g.orgs
}

func (g *googleCredential) update(idToken, refreshToken string) (err error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.token = idToken
	g.refreshToken = refreshToken // the server can return a new refresh_token
	g.jwtClaims, err = tokenClaims(g.token)
	if err != nil {
		return err
	}
	exp, ok := g.jwtClaims["exp"].(float64)
	if !ok {
		return errors.E("cached JWT token has no expiration field")
	}
	sec, dec := math.Modf(exp)
	g.expireAt = time.Unix(int64(sec), int64(dec*(1e9)))

	email, ok := g.jwtClaims["email"].(string)
	if !ok {
		return errors.E(`Google JWT token has no "email" field`)
	}
	g.email = email
	return nil
}
