// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cliauth

import (
	"bytes"
	"context"
	stdjson "encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net"
	stdhttp "net/http"
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
	"github.com/terramate-io/terramate/cloud/api/resources"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/errors/verbosity"
	"github.com/terramate-io/terramate/http"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/ui/tui/cliconfig"
	"github.com/terramate-io/terramate/ui/tui/clitest"
	tel "github.com/terramate-io/terramate/ui/tui/telemetry"
)

const (
	// that's a public key.
	defaultAPIKey = "AIzaSyDeCYIgqEhufsnBGtlNu4fv1alvpcs1Nos"

	credfile = "credentials.tmrc.json"

	minPort = 40000
	maxPort = 52023

	defaultGoogleTimeout = 60 * time.Second

	googleProviderID = "google.com"
	googleOauthScope = `{"google.com": "profile"}`
)

type (
	googleCredential struct {
		mu sync.RWMutex

		idpKey       string
		token        string
		refreshToken string
		jwtClaims    jwt.MapClaims
		expireAt     time.Time
		orgs         resources.MemberOrganizations
		user         resources.User

		provider string

		clicfg   cliconfig.Config
		client   *cloud.Client
		printers printer.Printers
	}

	createAuthURIResponse struct {
		Kind       string `json:"kind"`
		AuthURI    string `json:"authUri"`
		ProviderID string `json:"providerId"`
		SessionID  string `json:"sessionId"`
	}

	credentialInfo struct {
		ProviderID        providerID `json:"providerId"`
		Email             string     `json:"email,omitempty"`
		EmailVerified     bool       `json:"emailVerified,omitempty"`
		DisplayName       string     `json:"displayName"`
		ScreenName        string     `json:"screenName"`
		LocalID           string     `json:"localId"`
		IDToken           string     `json:"idToken"`
		Context           string     `json:"context"`
		OauthAccessToken  string     `json:"oauthAccessToken"`
		OauthExpireIn     int        `json:"oauthExpireIn"`
		RefreshToken      string     `json:"refreshToken"`
		ExpiresIn         string     `json:"expiresIn"`
		OauthIDToken      string     `json:"oauthIdToken"`
		RawUserInfo       string     `json:"rawUserInfo"`
		NeedConfirmation  bool       `json:"needConfirmation,omitempty"`
		VerifiedProviders []string   `json:"verifiedProvider,omitempty"`
	}

	cachedCredential struct {
		Provider     string `json:"provider"`
		IDToken      string `json:"id_token"`
		RefreshToken string `json:"refresh_token"`
	}
)

// GoogleLogin logs in the user using Google OAuth.
func GoogleLogin(printers printer.Printers, verbosity int, clicfg cliconfig.Config) error {
	h := &tokenHandler{
		credentialChan: make(chan credentialInfo),
		errChan:        make(chan tokenError),
		idpKey:         idpkey(),
	}

	mux := stdhttp.NewServeMux()
	mux.Handle("/auth", h)

	s := &stdhttp.Server{
		Handler: mux,
	}

	redirectURLChan := make(chan string)
	consentDataChan := make(chan createAuthURIResponse)

	go startServer(s, h, []int{}, redirectURLChan, consentDataChan)

	consentData, err := createAuthURI(googleProviderID, googleOauthScope, <-redirectURLChan, h.idpKey, map[string]any{})
	if err != nil {
		return err
	}

	consentDataChan <- consentData

	err = browser.OpenURL(consentData.AuthURI)
	if err != nil {
		printers.Stdout.Println("failed to open URL in the browser")
		printers.Stdout.Println(fmt.Sprintf("Please visit the url: %s", consentData.AuthURI))
	} else {
		printers.Stdout.Println("Please continue the authentication process in the browser.")
	}

	select {
	case cred := <-h.credentialChan:
		printers.Stdout.Println(fmt.Sprintf("Logged in as %s", cred.UserDisplayName()))
		if verbosity > 0 {
			printers.Stdout.Println(fmt.Sprintf("Token: %s", cred.IDToken))
			expire, _ := strconv.Atoi(cred.ExpiresIn)
			printers.Stdout.Println(fmt.Sprintf("Expire at: %s", time.Now().Add(time.Second*time.Duration(expire)).Format(time.RFC822Z)))
		}
		return saveCredential(printers, verbosity, "Google", cred, clicfg)
	case err := <-h.errChan:
		return err.err
	}
}

func startServer(
	s *stdhttp.Server,
	h *tokenHandler,
	ports []int,
	redirectURLChan chan<- string,
	consentDataChan <-chan createAuthURIResponse,
) {
	var err error
	defer func() {
		if err != nil {
			h.errChan <- tokenError{
				err: err,
			}
		}
	}()

	const maxretry = 5
	if len(ports) == 0 {
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))

		for i := 0; i < maxretry; i++ {
			ports = append(ports, minPort+rng.Intn(maxPort-minPort))
		}
	}

	var ln net.Listener
	for _, port := range ports {
		addr := "127.0.0.1:" + strconv.Itoa(port)
		s.Addr = addr

		ln, err = net.Listen("tcp", addr)
		if err == nil {
			break
		}
	}

	if ln == nil {
		err = errors.E(err, "failed to find an available port, please try again")
		return
	}

	redirectURL := "http://" + s.Addr + "/auth"

	redirectURLChan <- redirectURL
	h.consentData = <-consentDataChan
	h.continueURL = redirectURL
	err = s.Serve(ln)
	if errors.Is(err, stdhttp.ErrServerClosed) {
		err = nil
	}
}

func createAuthURI(providerID, oauthScope, continueURI, idpKey string, customParameter map[string]any) (createAuthURIResponse, error) {
	const endpoint = "https://www.googleapis.com/identitytoolkit/v3/relyingparty/createAuthUri"

	type payload struct {
		ProviderID      string                 `json:"providerId"`
		ContinueURI     string                 `json:"continueUri"`
		CustomParameter map[string]interface{} `json:"customParameter"`
		OauthScope      string                 `json:"oauthScope"`
	}

	payloadData := payload{
		ProviderID:      providerID,
		ContinueURI:     continueURI,
		CustomParameter: customParameter,
		OauthScope:      oauthScope,
	}

	postBody, err := stdjson.Marshal(&payloadData)
	if err != nil {
		return createAuthURIResponse{}, errors.E(err)
	}

	url := endpointURL(endpoint, idpKey)
	req, err := stdhttp.NewRequest("POST", url.String(), bytes.NewBuffer(postBody))
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

	client := &stdhttp.Client{}
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

func signInWithIDP(reqPayload googleSignInPayload, idpKey string) (cred credentialInfo, err error) {
	const endpoint = "https://identitytoolkit.googleapis.com/v1/accounts:signInWithIdp"

	reqData, err := stdjson.Marshal(reqPayload)
	if err != nil {
		return credentialInfo{}, err
	}

	url := endpointURL(endpoint, idpKey)
	ctx, cancel := context.WithTimeout(context.Background(), defaultGoogleTimeout)
	defer cancel()

	req, err := stdhttp.NewRequestWithContext(ctx, "POST", url.String(), bytes.NewBuffer(reqData))
	if err != nil {
		return credentialInfo{}, err
	}

	req.Header.Add("content-type", "application/json")
	req.Header.Add("Accept", "application/json")

	client := &stdhttp.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return credentialInfo{}, errors.E(err, "failed to start authentication process")
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return credentialInfo{}, errors.E(err)
	}

	if resp.StatusCode != 200 {
		return credentialInfo{}, errors.E("%s request returned %d", req.URL, resp.StatusCode)
	}

	var creds credentialInfo
	err = stdjson.Unmarshal(data, &creds)
	if err != nil {
		return credentialInfo{}, err
	}
	if creds.NeedConfirmation {
		return credentialInfo{}, newIDPNeedConfirmationError(creds.VerifiedProviders)
	}

	claims, err := tokenClaims(creds.IDToken)
	if err != nil {
		return credentialInfo{}, err
	}

	if emailVerified, ok := claims["email_verified"].(bool); ok && !emailVerified {
		return credentialInfo{}, newEmailNotVerifiedError(creds.Email)
	}

	return creds, nil
}

func (c credentialInfo) UserDisplayName() string {
	if c.DisplayName != "" {
		return c.DisplayName
	}
	return c.ScreenName
}

type tokenError struct {
	err error
}

type tokenHandler struct {
	sync.Mutex

	complete       bool
	consentData    createAuthURIResponse
	continueURL    string
	errChan        chan tokenError
	credentialChan chan credentialInfo
	idpKey         string
}

type googleSignInPayload struct {
	PostBody            string `json:"postBody,omitempty"`
	RequestURI          string `json:"requestUri"`
	SessionID           string `json:"sessionId,omitempty"`
	ReturnSecureToken   bool   `json:"returnSecureToken"`
	ReturnIdpCredential bool   `json:"returnIdpCredential"`
}

func (h *tokenHandler) ServeHTTP(w stdhttp.ResponseWriter, r *stdhttp.Request) {
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

	if errStr := r.URL.Query().Get("error_description"); errStr != "" {
		h.handleErr(w)

		errDecoded, _ := url.QueryUnescape(errStr)
		h.errChan <- tokenError{
			err: errors.E(errDecoded),
		}
		return
	}

	reqPayload := googleSignInPayload{
		RequestURI:          gotURL.String(),
		SessionID:           h.consentData.SessionID,
		ReturnSecureToken:   true,
		ReturnIdpCredential: true,
	}

	creds, err := signInWithIDP(reqPayload, idpkey())
	if err != nil {
		h.handleErr(w)
		h.errChan <- tokenError{
			err: err,
		}
		return
	}

	h.handleOK(w, creds)
}

func (h *tokenHandler) handleOK(w stdhttp.ResponseWriter, cred credentialInfo) {
	h.credentialChan <- cred

	w.Header().Add("Location", "https://cloud.terramate.io/cli/signed-in")
	w.WriteHeader(stdhttp.StatusSeeOther)
}

func (h *tokenHandler) handleErr(w stdhttp.ResponseWriter) {
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
	w.WriteHeader(stdhttp.StatusInternalServerError)
	_, _ = w.Write([]byte(errMessage))
}

func saveCredential(printers printer.Printers, verbosity int, providerID string, cred credentialInfo, clicfg cliconfig.Config) error {
	cachePayload := cachedCredential{
		Provider:     providerID,
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

	if verbosity > 0 {
		printers.Stdout.Println(fmt.Sprintf("credentials cached at %s", credfile))
	}
	return nil
}

func loadCredential(clicfg cliconfig.Config) (cachedCredential, bool, error) {
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
	//printers.Stdout.Println(fmt.Sprintf("credentials loaded from %s", credFile))
	return cred, true, nil
}

func endpointURL(endpoint string, idpKey string) *url.URL {
	u, err := url.Parse(endpoint)
	if err != nil {
		printer.Stderr.FatalWithDetails("failed to parse endpoint URL for createAuthURI", err)
	}

	q := u.Query()
	q.Add("key", idpKey)
	u.RawQuery = q.Encode()
	return u
}

func newGoogleCredential(printers printer.Printers, clicfg cliconfig.Config, client *cloud.Client) *googleCredential {
	return &googleCredential{
		clicfg:   clicfg,
		idpKey:   idpkey(),
		client:   client,
		printers: printers,
	}
}

func (g *googleCredential) Load() (bool, error) {
	credinfo, found, err := loadCredential(g.clicfg)
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
	g.client.SetCredential(g)
	g.provider = credinfo.Provider
	return true, g.fetchDetails()
}

func (g *googleCredential) Name() string {
	return g.provider
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
		g.printers.Stdout.Println("refreshing token...")

		defer func() {
			if err == nil {
				g.printers.Stdout.Println("token successfully refreshed.")
				g.printers.Stdout.Println(fmt.Sprintf("next token refresh in: %s", time.Until(g.ExpireAt())))
			}
		}()
	}

	const oidcTimeout = 60 // seconds
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

	req, err := stdhttp.NewRequestWithContext(ctx, "POST", endpoint.String(), bytes.NewBuffer(payloadData))
	if err != nil {
		return err
	}

	client := &stdhttp.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer func() {
		err = resp.Body.Close()
		if err != nil {
			g.printers.Stderr.Println(fmt.Sprintf("failed to close response body: %v", err))
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

	return saveCredential(g.printers, 0, g.provider, credentialInfo{
		ProviderID:   providerID(g.provider),
		IDToken:      g.token,
		RefreshToken: g.refreshToken,
	}, g.clicfg)
}

func (g *googleCredential) HasExpiration() bool {
	return true
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
			value: g.user.Email,
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

func (g *googleCredential) ApplyCredentials(req *stdhttp.Request) error {
	return applyJWTBasedCredentials(req, g)
}

func (g *googleCredential) RedactCredentials(req *stdhttp.Request) {
	redactJWTBasedCredentials(req)
}

func (g *googleCredential) fetchDetails() error {
	var (
		err  error
		user resources.User
		orgs resources.MemberOrganizations
	)

	func() {
		ctx, cancel := context.WithTimeout(context.Background(), defaultCloudTimeout)
		defer cancel()
		orgs, err = g.client.MemberOrganizations(ctx)
	}()

	if err != nil {
		return err
	}

	func() {
		ctx, cancel := context.WithTimeout(context.Background(), defaultCloudTimeout)
		defer cancel()
		user, err = g.client.Users(ctx)
	}()

	if err != nil {
		if errors.IsKind(err, http.ErrNotFound) {
			return errors.E(clitest.ErrCloudOnboardingIncomplete)
		}
		return err
	}
	g.orgs = orgs
	g.user = user

	tel.DefaultRecord.Set(tel.AuthUser(g.user.UUID))

	return nil
}

// Info display the credential details.
func (g *googleCredential) Info(selectedOrgName string) {
	printer.Stdout.Println(fmt.Sprintf("provider: %s", g.Name()))
	printer.Stdout.Println("status: signed in")
	if g.user.DisplayName != "" {
		printer.Stdout.Println(fmt.Sprintf("user: %s", g.user.DisplayName))
	}

	for _, kv := range g.DisplayClaims() {
		printer.Stdout.Println(fmt.Sprintf("%s: %s", kv.key, kv.value))
	}

	activeOrgs := g.orgs.ActiveOrgs()
	if len(activeOrgs) > 0 {
		printer.Stdout.Println(fmt.Sprintf("active organizations: %s", activeOrgs))
	}
	if invitedOrgs := g.orgs.InvitedOrgs(); len(invitedOrgs) > 0 {
		printer.Stdout.Println(fmt.Sprintf("pending invitations: %d", len(invitedOrgs)))
	}
	if ssoInvitedOrgs := g.orgs.SSOInvitedOrgs(); len(ssoInvitedOrgs) > 0 {
		printer.Stdout.Println(fmt.Sprintf("pending SSO invitations: %d", len(ssoInvitedOrgs)))
	}

	if len(activeOrgs) == 0 {
		printer.Stderr.Warnf("You are not part of an organization. Please join an organization or visit %s to create a new one.", cloud.HTMLURL(g.client.Region()))
	}

	if selectedOrgName == "" {
		printer.Stderr.ErrorWithDetails(
			"Missing cloud configuration",
			errors.E("Please set TM_CLOUD_ORGANIZATION environment variable or "+
				"terramate.config.cloud.organization configuration attribute to a specific organization",
			),
		)
		return
	}

	org, found := g.orgs.LookupByName(selectedOrgName)
	if found {
		if org.Status != "active" {
			printer.Stderr.Warn("selected organization (%s) is not active")
		} else {
			printer.Stdout.Println(fmt.Sprintf("selected organization: %s", org))
		}
	} else {
		printer.Stderr.Error(errors.E("selected organization %q not found in the list of active organizations", selectedOrgName))
	}

	if g.user.DisplayName == "" {
		printer.Stderr.Warnf("On-boarding is incomplete. Please visit %s to complete on-boarding.", cloud.HTMLURL(g.client.Region()))
	}
}

// Organizations returns the list of organizations associated with the credential.
func (g *googleCredential) Organizations() resources.MemberOrganizations {
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
	return nil
}

// newIDPNeedConfirmationError creates an error indicating the user has multiple providers set up and
// linking them is needed.
func newIDPNeedConfirmationError(verifiedProviders []string) *errors.DetailedError {
	err := errors.D("The account was already set up with another email provider.")

	if len(verifiedProviders) > 0 {
		err = err.WithDetailf(verbosity.V1, "Please login using one of the methods below:")
		for _, providerDomain := range verifiedProviders {
			switch providerDomain {
			case "google.com":
				err = err.WithDetailf(verbosity.V1, "- Run 'terramate cloud login --google' to login with your Google account")
			case "github.com":
				err = err.WithDetailf(verbosity.V1, "- Run 'terramate cloud login --github' to login with your GitHub account")
			}
			err = err.WithDetailf(verbosity.V1, "Alternatively, visit https://cloud.terramate.io and authenticate with the Social login to link the accounts.")
		}
	} else {
		err = err.WithDetailf(verbosity.V1, "Visit https://cloud.terramate.io and authenticate to link the accounts.")
	}

	return err.WithCode(ErrIDPNeedConfirmation)
}

// newEmailNotVerifiedError creates an error indicating that user's email need to be verified.
func newEmailNotVerifiedError(email string) *errors.DetailedError {
	return errors.D("Email %s is not verified.", email).
		WithDetailf(verbosity.V1, "Please login to https://cloud.terramate.io to verify your email and continue the sign up process.").
		WithCode(ErrEmailNotVerified)
}

func idpkey() string {
	idpKey := os.Getenv("TMC_API_IDP_KEY")
	if idpKey == "" {
		idpKey = defaultAPIKey
	}
	return idpKey
}
