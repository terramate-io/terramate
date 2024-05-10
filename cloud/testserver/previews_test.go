// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package testserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/cloud/testserver/cloudstore"
)

func TestPostPreviews(t *testing.T) {
	orguuid := "deadbeef-dead-dead-dead-deaddeadbeef"
	const testserverJSONFile = "../../testdata/testserver/cloud.data.json"
	store, err := cloudstore.LoadDatastore(testserverJSONFile)
	assert.NoError(t, err)
	router := Router(store)

	// create a preview
	w := doRequest(t, router, "POST", "/v1/previews/"+orguuid,
		`{
				"stacks": [
					{
					  "preview_status": "pending",
					  "cmd": ["terraform",  "plan"],
					  "repository": "terramate-io/terramate",
					  "name": "teststack",
					  "path": "teststack",
					  "meta_id": "teststack",
					  "default_branch": "main"
					}
				],
				"review_request": {"repository": "terramate-io/terramate", "number": 1, "title": "sometitle"},
				"updated_at": 1709644546,
				"pushed_at": 1709644546,
				"commit_sha": "somecommitsha",
				"technology": "terraform",
				"technology_layer": "default"
			}`,
	)

	assert.EqualInts(t, http.StatusOK, w.Code, w.Body.String())
	assert.EqualStrings(t,
		`{"preview_id":"1","stacks":[{"meta_id":"teststack","stack_preview_id":"1"}]}`,
		string(w.Body.String()),
	)

	// update stack preview status to running
	wStackPreview := doRequest(t, router, "PATCH", "/v1/stack_previews/"+orguuid+"/1", `{ "status": "running" }`)
	assert.EqualInts(t, http.StatusNoContent, wStackPreview.Code)

	// update stack preview status to changed (with changeset)
	wStackPreviewChanged := doRequest(t, router, "PATCH", "/v1/stack_previews/"+orguuid+"/1",
		`{
			  "status": "changed",
			  "changeset_details": {
			    "provisioner": "terraform",
				"changeset_ascii": "test changeset",
				"changeset_json": "{\"test\": \"changeset\"}"
			  }
	}`)
	assert.EqualInts(t, http.StatusNoContent, wStackPreviewChanged.Code)

	// create stack preview logs
	wLogs := doRequest(t, router, "POST", "/v1/stack_previews/"+orguuid+"/1/logs",
		`[
			   {
			     "log_line": 1,
				 "timestamp": "2024-01-01T00:00:00Z",
				 "channel": "stdout",
				 "message": "test message 1"
			   },
			   {
			     "log_line": 2,
				 "timestamp": "2024-01-01T00:00:01Z",
				 "channel": "stdout",
				 "message": "test message 2"
			   }
			]`,
	)

	assert.EqualInts(t, http.StatusNoContent, wLogs.Code, wLogs.Body.String())

	// get preview and assert contents
	wGetPreview := doRequest(t, router, "GET", "/v1/previews/"+orguuid+"/1", "")

	var preview cloudstore.Preview
	if err := json.Unmarshal(wGetPreview.Body.Bytes(), &preview); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assert.EqualInts(t, http.StatusOK, wGetPreview.Code, wGetPreview.Body.String())
	assert.EqualStrings(t, "1", preview.PreviewID)
	assert.EqualStrings(t, "terraform", preview.Technology)
	assert.EqualStrings(t, "default", preview.TechnologyLayer)
	assert.EqualInts(t, 1709644546, int(preview.UpdatedAt))
	assert.EqualInts(t, 1709644546, int(preview.PushedAt))
	assert.EqualStrings(t, "somecommitsha", preview.CommitSHA)

	assert.IsTrue(t, len(preview.StackPreviews) == 1)
	assert.EqualStrings(t, "1", preview.StackPreviews[0].ID)
	assert.EqualStrings(t, "terraform", preview.StackPreviews[0].ChangesetDetails.Provisioner)
	assert.EqualStrings(t, "test changeset", preview.StackPreviews[0].ChangesetDetails.ChangesetASCII)
	assert.EqualStrings(t, `{"test": "changeset"}`, preview.StackPreviews[0].ChangesetDetails.ChangesetJSON)

	assert.EqualInts(t, 2, len(preview.StackPreviews[0].Logs))
	assert.EqualInts(t, 1, int(preview.StackPreviews[0].Logs[0].Line))
	assert.EqualStrings(t, "test message 1", preview.StackPreviews[0].Logs[0].Message)
	assert.EqualInts(t, 2, int(preview.StackPreviews[0].Logs[1].Line))
	assert.EqualStrings(t, "test message 2", preview.StackPreviews[0].Logs[1].Message)

	// t.Logf(string(respGetPreview))
}

func doRequest(t *testing.T, router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req, err := http.NewRequest(method, path, strings.NewReader(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-agent", "terramate/0.0.0-testserver")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}
