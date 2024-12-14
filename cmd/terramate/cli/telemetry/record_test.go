// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package telemetry_test

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate"
	"github.com/terramate-io/terramate/test/sandbox"

	. "github.com/terramate-io/terramate/cmd/terramate/cli/telemetry"
)

func TestRecordLifecycle(t *testing.T) {
	s := sandbox.New(t)

	// Setup credentials and an existing checkpoint signature.
	s.BuildTree([]string{
		`f:userdir/credentials.tmrc.json:{"provider": "Google"}`,
		"f:userdir/checkpoint_signature:a1a15394-e622-4a88-9e01-25b3cdc1d28f\nThis was\ngenerated",
	})

	t.Setenv("GITHUB_ACTIONS", "1")

	credfile := filepath.Join(s.RootDir(), "userdir/credentials.tmrc.json")
	cpsigfile := filepath.Join(s.RootDir(), "userdir/checkpoint_signature")
	anasigfile := filepath.Join(s.RootDir(), "userdir/analytics_signature")

	// Create a record and set data.
	rec := NewRecord()
	rec.Set(
		Command("my-command"),
		OrgName("hello-org"),
		DetectFromEnv(credfile, cpsigfile, anasigfile),
		BoolFlag("flag1", true),
		BoolFlag("flag2", false),
		StringFlag("flag3", "something"),
		StringFlag("flag4", ""),
		StringFlag("flag5", "something", "my-command"),
		StringFlag("flag6", "something", "my-command2"),
	)

	tr := &fakeTransport{}
	cl := &http.Client{Transport: tr}

	rec.Send(SendMessageParams{
		Client: cl,
	})

	// Second send is a no-op.
	rec.Send(SendMessageParams{
		Client: cl,
	})

	assert.NoError(t, rec.WaitForSend())
	assert.EqualInts(t, 1, len(tr.receivedReqs))

	req := tr.receivedReqs[0]

	assert.EqualStrings(t, Endpoint().Host, req.Host)
	assert.EqualStrings(t, "terramate/v"+terramate.Version(), req.Header["User-Agent"][0])

	var gotMsg Message
	err := json.NewDecoder(req.Body).Decode(&gotMsg)
	assert.NoError(t, err)

	assert.EqualInts(t, int(PlatformGithub), int(gotMsg.Platform))

	assert.EqualStrings(t, "a1a15394-e622-4a88-9e01-25b3cdc1d28f", gotMsg.Signature)

	assert.EqualStrings(t, runtime.GOARCH, gotMsg.Arch)
	assert.EqualStrings(t, runtime.GOOS, gotMsg.OS)

	assert.EqualStrings(t, "my-command", gotMsg.Command)
	assert.EqualStrings(t, "hello-org", gotMsg.OrgName)

	if diff := cmp.Diff([]string{"flag1", "flag3", "flag5"}, gotMsg.Details); diff != "" {
		t.Errorf("unexpected flag details: %s", diff)
	}

	storedSig := ReadSignature(anasigfile)
	assert.EqualStrings(t, "a1a15394-e622-4a88-9e01-25b3cdc1d28f", storedSig)
}
