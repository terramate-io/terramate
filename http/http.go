// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package http provides HTTP helper functions and error types.
package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"

	stdhttp "net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	"github.com/terramate-io/terramate"
	"github.com/terramate-io/terramate/cloud/api/resources"
	"github.com/terramate-io/terramate/errors"
)

// ErrUnexpectedStatus indicates the server responded with an unexpected status code.
const ErrUnexpectedStatus errors.Kind = "unexpected status code"

// ErrNotFound indicates the requested resource does not exist in the server.
const ErrNotFound errors.Kind = "resource not found (HTTP Status 404)"

// ErrConflict indicates the request contains conflicting data (eg.: duplicated resource)
const ErrConflict errors.Kind = "conflict (HTTP Status 409)"

// ErrUnexpectedResponseBody indicates the server responded with an unexpected body.
const ErrUnexpectedResponseBody errors.Kind = "unexpected API response body"

// Credential is the interface for the credential providers.
// Because each provider has its own way to authenticate, the dependency inversion principle
// is used to abstract the authentication method.
type Credential interface {
	// ApplyCredentials applies the credential to the given request.
	ApplyCredentials(req *stdhttp.Request) error

	// RedactCredentials redacts the credential from the given request.
	// This is used for dumping the request without exposing sensitive information.
	RedactCredentials(req *stdhttp.Request)
}

// Client is the interface for the HTTP client.
type Client interface {
	HTTPClient() *stdhttp.Client
	Credential() Credential
}

var debugAPIRequests bool

func init() {
	if d := os.Getenv("TMC_API_DEBUG"); d == "1" || d == "true" {
		debugAPIRequests = true
	}
}

// Get requests the endpoint components list making a GET request and decode the response into the
// entity T if validates successfully.
func Get[T resources.Resource](ctx context.Context, client Client, u url.URL) (entity T, err error) {
	resource, err := Request[T](ctx, client, "GET", u, nil)
	if err != nil {
		return entity, err
	}
	return resource, nil
}

// Post requests the endpoint components list making a POST request and decode the response into the
// entity T if validates successfully.
func Post[T resources.Resource](ctx context.Context, client Client, payload any, url url.URL) (entity T, err error) {
	resource, err := Request[T](ctx, client, "POST", url, payload)
	if err != nil {
		return entity, err
	}
	return resource, nil
}

// Patch requests the endpoint components list making a PATCH request and decode the response into the
// entity T if validates successfully.
func Patch[T resources.Resource](ctx context.Context, client Client, payload interface{}, url url.URL) (entity T, err error) {
	resource, err := Request[T](ctx, client, "PATCH", url, payload)
	if err != nil {
		return entity, err
	}
	return resource, nil
}

// Put requests the endpoint components list making a PUT request and decode the
// response into the entity T if validated successfully.
func Put[T resources.Resource](ctx context.Context, client Client, payload interface{}, url url.URL) (entity T, err error) {
	resource, err := Request[T](ctx, client, "PUT", url, payload)
	if err != nil {
		return entity, err
	}
	return resource, nil
}

// Delete requests the endpoint url with a DELETE method.
func Delete[T resources.Resource](ctx context.Context, client Client, url url.URL) error {
	_, err := Request[T](ctx, client, "DELETE", url, nil)
	return err
}

// Request makes a request to the Terramate Cloud using client.
// The instantiated type gets decoded and return as the entity T,
// The payload is encoded accordingly to the rules below:
// - If payload is nil, no body is sent and no Content-Type is set.
// - If payload is a []byte or string, it is sent as is and the Content-Type is set to text/plain.
// - If payload is any other type, it is marshaled to JSON and the Content-Type is set to application/json.
func Request[T resources.Resource](ctx context.Context, c Client, method string, url url.URL, payload any) (res T, err error) {
	req, err := newRequest(ctx, c, method, url, payload)
	if err != nil {
		return res, err
	}

	if debugAPIRequests {
		data, _ := dumpRequest(c, req)
		fmt.Printf(">>> %s\n\n", data)
	}

	client := c.HTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return res, err
	}

	if debugAPIRequests {
		data, _ := httputil.DumpResponse(resp, true)
		fmt.Printf("<<< %s\n\n", data)
	}

	defer func() {
		err = errors.L(err, resp.Body.Close()).AsError()
	}()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return res, err
	}

	if resp.StatusCode == stdhttp.StatusNotFound {
		return res, errors.E(ErrNotFound, "%s %s", method, url.String())
	}

	if resp.StatusCode == stdhttp.StatusConflict {
		return res, errors.E(ErrConflict, "%s %s", method, url.String())
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return res, errors.E(ErrUnexpectedStatus, "%s: status: %d, content: %s", url.String(), resp.StatusCode, data)
	}

	if resp.StatusCode == stdhttp.StatusNoContent {
		return res, nil
	}

	ctype, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if ctype != objectContentType {
		return res, errors.E(ErrUnexpectedResponseBody, "client expects the Content-Type: %s but got %s", objectContentType, ctype)
	}

	err = json.Unmarshal(data, &res)
	if err != nil {
		return res, errors.E(ErrUnexpectedResponseBody, err, "status: %d, data: %s", resp.StatusCode, data)
	}
	err = res.Validate()
	if err != nil {
		return res, errors.E(ErrUnexpectedResponseBody, err)
	}
	return res, nil
}

func newRequest(ctx context.Context, c Client, method string, url url.URL, payload any) (*stdhttp.Request, error) {
	body, ctype, err := preparePayload(payload)
	if err != nil {
		return nil, err
	}

	req, err := stdhttp.NewRequestWithContext(ctx, method, url.String(), body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "terramate/v"+terramate.Version())
	req.Header.Set("Content-Type", ctype)

	if cred := c.Credential(); cred != nil {
		err := cred.ApplyCredentials(req)
		if err != nil {
			return nil, err
		}
	}
	return req, nil
}

func preparePayload(payload any) (body io.Reader, ctype string, err error) {
	if payload != nil {
		switch v := payload.(type) {
		case []byte:
			body = bytes.NewBuffer(v)
			ctype = "text/plain"
		case string:
			body = strings.NewReader(v)
			ctype = "text/plain"
		default:
			data, err := json.Marshal(payload)
			if err != nil {
				return nil, "", errors.E("marshaling request payload", err)
			}
			body = bytes.NewBuffer(data)
			ctype = objectContentType
		}
	}
	return body, ctype, nil
}

// dumpRequest returns a string representation of the request with the
// Authentication/Authorization header redacted.
func dumpRequest(client Client, req *stdhttp.Request) ([]byte, error) {
	reqCopy := req.Clone(req.Context())

	var err error
	if req.GetBody != nil {
		reqCopy.Body, err = req.GetBody()
		if err != nil {
			return nil, err
		}
	}

	if cred := client.Credential(); cred != nil {
		cred.RedactCredentials(reqCopy)
	}

	return httputil.DumpRequestOut(reqCopy, true)
}

const objectContentType = "application/json"
