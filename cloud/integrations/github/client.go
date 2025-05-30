// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package github implements a client SDK for the Github API.
package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/terramate-io/terramate/errors"
)

// GitHub API errors.
const (
	// ErrNotFound indicates the resource does not exists.
	ErrNotFound errors.Kind = "resource not found (HTTP Status: 404)"
	// ErrUnprocessableEntity indicates the entity cannot be processed for any reason.
	ErrUnprocessableEntity errors.Kind = "entity cannot be processed (HTTP Status: 422)"

	ErrDeviceFlowAuthPending    errors.Kind = "the authorization request is still pending"
	ErrDeviceFlowSlowDown       errors.Kind = "too many requests, please slowdown"
	ErrDeviceFlowAuthUnexpected errors.Kind = "unexpected device flow error"
	ErrDeviceFlowAuthExpired    errors.Kind = "this 'device_code' has expired"
	ErrDeviceFlowAccessDenied   errors.Kind = "user cancelled the authorization flow"
	ErrDeviceFlowIncorrectCode  errors.Kind = "the device code provided is not valid"
)

const (
	// Domain is the default GitHub domain.
	Domain = "github.com"
	// APIDomain is the default GitHub API domain.
	APIDomain = "api." + Domain
	// APIBaseURL is the default base url for the GitHub API.
	APIBaseURL = "https://" + APIDomain

	// DefaultTimeout is the default timeout for GitHub API requests.
	DefaultTimeout = 60 * time.Second
)

type (
	// OIDCVars is the variables used for issuing new OIDC tokens.
	OIDCVars struct {
		ReqURL   string
		ReqToken string
	}

	// OAuthDeviceFlowContext holds the context information for an ongoing
	// device authentication flow.
	OAuthDeviceFlowContext struct {
		clientID        string
		UserCode        string `json:"user_code"`
		DeviceCode      string `json:"device_code"`
		VerificationURI string `json:"verification_uri"`
		Interval        int    `json:"interval"`
		ExpiresIn       int    `json:"expires_in"`
	}
)

// OIDCToken requests a new OIDC token.
func OIDCToken(ctx context.Context, cfg OIDCVars) (token string, err error) {
	req, err := http.NewRequestWithContext(ctx, "GET", cfg.ReqURL, nil)
	if err != nil {
		return "", errors.E(err, "creating Github OIDC request")
	}

	req.Header.Set("Authorization", "Bearer "+cfg.ReqToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", errors.E(err, "requesting GET %s", req.URL)
	}

	defer func() {
		err = errors.L(err, resp.Body.Close()).AsError()
	}()

	if resp.StatusCode == http.StatusNotFound {
		return "", errors.E(ErrNotFound, "retrieving %s", req.URL)
	}

	if resp.StatusCode == http.StatusUnprocessableEntity {
		return "", errors.E(ErrUnprocessableEntity, "retrieving %s", req.URL)
	}

	if resp.StatusCode != http.StatusOK {
		return "", errors.E("unexpected status code: %s while getting %s", resp.Status, req.URL)
	}

	type response struct {
		Value string `json:"value"`
	}

	var tokresp response
	err = json.NewDecoder(resp.Body).Decode(&tokresp)
	if err != nil {
		return "", errors.E(err, "unmarshaling Github OIDC JSON response")
	}
	return tokresp.Value, nil
}

// OAuthDeviceFlowAuthStart starts a GitHub device authentication flow.
// After the flow is started, you need to probe for its state using [ProbeAuthState].
func OAuthDeviceFlowAuthStart(clientID string) (oauthCtx OAuthDeviceFlowContext, err error) {
	const deviceCodeURL = "https://github.com/login/device/code"
	params := url.Values{
		"client_id": []string{clientID},
		"scope":     []string{"user:email"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", deviceCodeURL, strings.NewReader(params.Encode()))
	if err != nil {
		return OAuthDeviceFlowContext{}, errors.E(err, "failed to create request for GitHub device code")
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return OAuthDeviceFlowContext{}, errors.E(err, "failed to fetch the GitHub device code")
	}
	defer func() {
		err = errors.L(err, resp.Body.Close()).AsError()
	}()

	err = json.NewDecoder(resp.Body).Decode(&oauthCtx)
	if err != nil {
		return OAuthDeviceFlowContext{}, errors.E(err, "failed to unmarshal response from GitHub OAuth URL: %s", deviceCodeURL)
	}

	oauthCtx.clientID = clientID
	return oauthCtx, nil
}

// ProbeAuthState checks the current authentication flow state.
// This method must be called repeatedly, respecting the oauthCtx.Interval,
// while it returns ErrDeviceFlowPending or ErrDeviceFlowSlowDown. Eventually, the user
// will either finish the flow, cancel the process, or the code will expire.
// When the user finishes the process by providing the correct code, this method returns
// the access_token and no error.
func (oauthCtx *OAuthDeviceFlowContext) ProbeAuthState() (string, error) {
	const uri = "https://github.com/login/oauth/access_token"
	params := url.Values{
		"client_id":   []string{oauthCtx.clientID},
		"device_code": []string{oauthCtx.DeviceCode},
		"grant_type":  []string{"urn:ietf:params:oauth:grant-type:device_code"},
	}
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", uri, strings.NewReader(params.Encode()))
	if err != nil {
		return "", err
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Accept", "application/json")

	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	type authResponse struct {
		AccessToken      string `json:"access_token"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}

	var payloadResp authResponse
	err = json.NewDecoder(resp.Body).Decode(&payloadResp)
	if err != nil {
		return "", err
	}

	switch payloadResp.Error {
	case "":
		// success
		return payloadResp.AccessToken, nil
	case "authorization_pending":
		return "", errors.E(ErrDeviceFlowAuthPending)
	case "expired_token":
		return "", errors.E(ErrDeviceFlowAuthExpired)
	case "slow_down":
		return "", errors.E(ErrDeviceFlowSlowDown)
	case "access_denied":
		return "", errors.E(ErrDeviceFlowAccessDenied)
	case "incorrect_device_code":
		return "", errors.E(ErrDeviceFlowIncorrectCode)

		// unrecoverable errors and internal bugs.
	case "device_flow_disabled":
		panic(errors.E(errors.ErrInternal, "device flow has not been enabled in the app settings"))
	case "incorrect_client_credentials":
		panic(errors.E(errors.ErrInternal, "invalid client_id %s", oauthCtx.clientID))
	case "unsupported_grant_type":
		panic(errors.E(errors.ErrInternal, "invalid grant type"))
	default:
		return "", errors.E(ErrDeviceFlowAuthUnexpected, payloadResp.ErrorDescription)
	}
}
