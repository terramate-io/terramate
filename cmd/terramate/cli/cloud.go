// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"net/http"

	"github.com/terramate-io/terramate/errors"
)

type cloud struct {
	baseAPI    string
	client     *http.Client
	credential credential
}

type credential interface {
	Name() string
	Load() (bool, error)
	Token() (string, error)
	Refresh() error
	IsExpired() bool
	String() string
}

func credentialPrecedence() []credential {
	return []credential{
		newGithubOIDC(),
	}
}

func (c *cli) cloudInfo() {
	cred, err := c.loadCredential()
	if err != nil {
		fatal(err)
	}

	_, token := cred.Token()
	c.output.MsgStdOut("token: %s", token)
}

func (c *cli) loadCredential() (credential, error) {
	probes := credentialPrecedence()
	var cred credential
	var found bool
	for _, probe := range probes {
		var err error
		found, err = probe.Load()
		if err != nil {
			return nil, err
		}
		if found {
			cred = probe
			break
		}
	}
	if !found {
		return nil, errors.E("no credential found")
	}

	return cred, nil
}
