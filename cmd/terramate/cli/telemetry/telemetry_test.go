// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package telemetry_test

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"

	"github.com/terramate-io/terramate/test/sandbox"

	. "github.com/terramate-io/terramate/cmd/terramate/cli/telemetry"
)

func TestDetectPlatformFromEnv(t *testing.T) {
	allEnvs := []string{"GITHUB_ACTIONS", "GITLAB_CI", "CI"}

	for k, want := range map[string]PlatformType{
		"GITHUB_ACTIONS": PlatformGithub,
		"GITLAB_CI":      PlatformGitlab,
		"CI":             PlatformGenericCI,
	} {
		t.Run(k, func(t *testing.T) {
			for _, e := range allEnvs {
				if e == k {
					t.Setenv(e, "1")
				} else {
					t.Setenv(e, "")
				}
			}
			assert.EqualInts(t, int(want), int(DetectPlatformFromEnv()))
		})
	}
}

func TestDetectAuthTypeFromEnv(t *testing.T) {
	allEnvs := []string{"ACTIONS_ID_TOKEN_REQUEST_TOKEN", "TM_GITLAB_ID_TOKEN"}

	for k, want := range map[string]AuthType{
		"ACTIONS_ID_TOKEN_REQUEST_TOKEN": AuthOIDCGithub,
		"TM_GITLAB_ID_TOKEN":             AuthOIDCGitlab,
	} {
		t.Run(k, func(t *testing.T) {
			for _, e := range allEnvs {
				if e == k {
					t.Setenv(e, "1")
				} else {
					t.Setenv(e, "")
				}
			}
			assert.EqualInts(t, int(want), int(DetectAuthTypeFromEnv("")))
		})
	}

	t.Run("Google", func(t *testing.T) {
		s := sandbox.New(t)
		s.BuildTree([]string{
			`f:userdir/credentials.tmrc.json:{"provider": "Google"}`,
		})
		for _, e := range allEnvs {
			t.Setenv(e, "")
		}
		assert.EqualInts(t, int(AuthIDPGoogle), int(DetectAuthTypeFromEnv(filepath.Join(s.RootDir(), "userdir/credentials.tmrc.json"))))
	})

	t.Run("GitHub", func(t *testing.T) {
		s := sandbox.New(t)
		s.BuildTree([]string{
			`f:userdir/credentials.tmrc.json:{"provider": "GitHub"}`,
		})
		for _, e := range allEnvs {
			t.Setenv(e, "")
		}
		assert.EqualInts(t, int(AuthIDPGithub), int(DetectAuthTypeFromEnv(filepath.Join(s.RootDir(), "userdir/credentials.tmrc.json"))))
	})
}

func TestGenerateOrReadSignature(t *testing.T) {
	t.Run("New random signature", func(t *testing.T) {
		s := sandbox.New(t)

		cpsigfile := filepath.Join(s.RootDir(), "userdir/checkpoint_signature")
		anasigfile := filepath.Join(s.RootDir(), "userdir/analytics_signature")

		got, isNew := GenerateOrReadSignature(cpsigfile, anasigfile)
		assert.IsTrue(t, got != "a1a15394-e622-4a88-9e01-25b3cdc1d28f")
		assert.IsTrue(t, isNew)

		saved := ReadSignature(anasigfile)
		assert.EqualStrings(t, got, saved)
	})

	t.Run("New signature, use checkpoint", func(t *testing.T) {
		s := sandbox.New(t)
		s.BuildTree([]string{
			`f:userdir/checkpoint_signature:a1a15394-e622-4a88-9e01-25b3cdc1d28f
			

This signature is a randomly generated UUID used to de-duplicate
alerts and version information. This signature is random, it is
not based on any personally identifiable information. To create
a new signature, you can simply delete this file at any time.
See the documentation for the software using Checkpoint for more
information on how to disable it.`,
		})

		cpsigfile := filepath.Join(s.RootDir(), "userdir/checkpoint_signature")
		anasigfile := filepath.Join(s.RootDir(), "userdir/analytics_signature")

		got, isNew := GenerateOrReadSignature(cpsigfile, anasigfile)
		assert.EqualStrings(t, "a1a15394-e622-4a88-9e01-25b3cdc1d28f", got)
		assert.IsTrue(t, isNew)

		saved := ReadSignature(anasigfile)
		assert.EqualStrings(t, got, saved)
	})

	t.Run("Use existing signature", func(t *testing.T) {
		s := sandbox.New(t)
		s.BuildTree([]string{
			`f:userdir/checkpoint_signature:a1a15394-e622-4a88-9e01-25b3cdc1d28f


This signature is a randomly generated UUID used to de-duplicate
alerts and version information. This signature is random, it is
not based on any personally identifiable information. To create
a new signature, you can simply delete this file at any time.
See the documentation for the software using Checkpoint for more
information on how to disable it.`,
			`f:userdir/analytics_signature:a1a15394-e622-4a88-9e01-25b3cdc1d28f


This is a randomly generated ID used to aggregate analytics data.`,
		})

		cpsigfile := filepath.Join(s.RootDir(), "userdir/checkpoint_signature")
		anasigfile := filepath.Join(s.RootDir(), "userdir/analytics_signature")

		got, isNew := GenerateOrReadSignature(cpsigfile, anasigfile)
		assert.EqualStrings(t, "a1a15394-e622-4a88-9e01-25b3cdc1d28f", got)
		assert.IsTrue(t, !isNew)

		saved := ReadSignature(anasigfile)
		assert.EqualStrings(t, got, saved)
	})
}

func TestSendMessage(t *testing.T) {
	t.Run("Error", func(t *testing.T) {
		tr := &fakeTransport{timeout: true}
		cl := &http.Client{Transport: tr}

		done := SendMessage(&Message{}, SendMessageParams{
			Client: cl,
		})

		err := <-done
		assert.IsError(t, err, context.DeadlineExceeded)
		assert.EqualInts(t, 0, len(tr.receivedReqs))
	})

	t.Run("OK", func(t *testing.T) {
		tr := &fakeTransport{}
		cl := &http.Client{Transport: tr}

		done := SendMessage(&Message{}, SendMessageParams{
			Client: cl,
		})

		err := <-done
		assert.NoError(t, err)
		assert.EqualInts(t, 1, len(tr.receivedReqs))
	})
}

type fakeTransport struct {
	timeout      bool
	receivedReqs []*http.Request
}

func (f *fakeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if f.timeout {
		return nil, context.DeadlineExceeded
	}
	f.receivedReqs = append(f.receivedReqs, req)
	return &http.Response{}, nil
}
