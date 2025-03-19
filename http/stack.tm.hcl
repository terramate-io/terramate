// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package http // import \"github.com/terramate-io/terramate/http\""
  description = "package http // import \"github.com/terramate-io/terramate/http\"\n\nPackage http provides HTTP helper functions and error types.\n\nconst ErrConflict errors.Kind = \"conflict (HTTP Status 409)\"\nconst ErrNotFound errors.Kind = \"resource not found (HTTP Status 404)\"\nconst ErrUnexpectedResponseBody errors.Kind = \"unexpected API response body\"\nconst ErrUnexpectedStatus errors.Kind = \"unexpected status code\"\nfunc Delete[T resources.Resource](ctx context.Context, client Client, url url.URL) error\nfunc Get[T resources.Resource](ctx context.Context, client Client, u url.URL) (entity T, err error)\nfunc Patch[T resources.Resource](ctx context.Context, client Client, payload interface{}, url url.URL) (entity T, err error)\nfunc Post[T resources.Resource](ctx context.Context, client Client, payload any, url url.URL) (entity T, err error)\nfunc Put[T resources.Resource](ctx context.Context, client Client, payload interface{}, url url.URL) (entity T, err error)\nfunc Request[T resources.Resource](ctx context.Context, c Client, method string, url url.URL, payload any) (res T, err error)\ntype Client interface{ ... }\ntype Credential interface{ ... }"
  tags        = ["golang", "http"]
  id          = "56b330b6-8669-4214-9785-495c9d5d7723"
}
