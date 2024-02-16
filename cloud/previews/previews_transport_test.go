// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package previews_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/terramate-io/terramate/cloud"
)

type previewsTransport struct {
	// receivedReqs is a list of all requests that were processed by the transport.
	receivedReqs []*http.Request
}

func (f *previewsTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	f.receivedReqs = append(f.receivedReqs, req)

	var reqParsed cloud.CreatePreviewPayloadRequest
	if err := json.NewDecoder(req.Body).Decode(&reqParsed); err != nil {
		return &http.Response{StatusCode: http.StatusInternalServerError}, nil
	}

	type previewResp struct {
		PreviewID string `json:"preview_id"`
		Stacks    []struct {
			MetaID         string `json:"meta_id"`
			StackPreviewID string `json:"stack_preview_id"`
		} `json:"stacks"`
	}

	resp := previewResp{PreviewID: "1"}
	for i, s := range reqParsed.Stacks {
		resp.Stacks = append(resp.Stacks, cloud.ResponsePreviewStack{
			MetaID:         s.MetaID,
			StackPreviewID: strconv.Itoa(i),
		})
	}

	respBytes, err := json.Marshal(resp)
	if err != nil {
		return &http.Response{StatusCode: http.StatusInternalServerError}, nil
	}

	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(respBytes)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}, nil
}

func (f *previewsTransport) httpEndpointsCalled() []string {
	endpoints := make([]string, len(f.receivedReqs))
	for i, r := range f.receivedReqs {
		endpoints[i] = fmt.Sprintf("%s %s", r.Method, r.URL.Path)
	}
	return endpoints
}
