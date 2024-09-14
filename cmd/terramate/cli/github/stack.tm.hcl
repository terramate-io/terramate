// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package github // import \"github.com/terramate-io/terramate/cmd/terramate/cli/github\""
  description = "package github // import \"github.com/terramate-io/terramate/cmd/terramate/cli/github\"\n\nPackage github implements a client SDK for the Github API.\n\nconst ErrNotFound errors.Kind = \"resource not found (HTTP Status: 404)\" ...\nconst Domain = \"github.com\" ...\nconst ErrGithubEventPathEnvNotSet errors.Kind = `environment variable \"GITHUB_EVENT_PATH\" not set` ...\nfunc GetEventPR() (*github.PullRequest, error)\nfunc OIDCToken(ctx context.Context, cfg OIDCVars) (token string, err error)\ntype OAuthDeviceFlowContext struct{ ... }\n    func OAuthDeviceFlowAuthStart(clientID string) (oauthCtx OAuthDeviceFlowContext, err error)\ntype OIDCVars struct{ ... }"
  tags        = ["cli", "cmd", "github", "golang", "terramate"]
  id          = "5e0c44d6-4b57-4469-acc6-d3a893908e22"
}
