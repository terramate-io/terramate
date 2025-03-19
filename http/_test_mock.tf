// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "http" {
  content = <<-EOT
package http // import "github.com/terramate-io/terramate/http"

Package http provides HTTP helper functions and error types.

const ErrConflict errors.Kind = "conflict (HTTP Status 409)"
const ErrNotFound errors.Kind = "resource not found (HTTP Status 404)"
const ErrUnexpectedResponseBody errors.Kind = "unexpected API response body"
const ErrUnexpectedStatus errors.Kind = "unexpected status code"
func Delete[T resources.Resource](ctx context.Context, client Client, url url.URL) error
func Get[T resources.Resource](ctx context.Context, client Client, u url.URL) (entity T, err error)
func Patch[T resources.Resource](ctx context.Context, client Client, payload interface{}, url url.URL) (entity T, err error)
func Post[T resources.Resource](ctx context.Context, client Client, payload any, url url.URL) (entity T, err error)
func Put[T resources.Resource](ctx context.Context, client Client, payload interface{}, url url.URL) (entity T, err error)
func Request[T resources.Resource](ctx context.Context, c Client, method string, url url.URL, payload any) (res T, err error)
type Client interface{ ... }
type Credential interface{ ... }
EOT

  filename = "${path.module}/mock-http.ignore"
}
