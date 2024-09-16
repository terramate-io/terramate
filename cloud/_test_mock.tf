// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "cloud" {
  content = <<-EOT
package cloud // import "github.com/terramate-io/terramate/cloud"

Package cloud implements a client SDK for communication with the cloud API.

Package cloud implements the SDK for communicating with the Terramate Cloud.

const WellKnownCLIPath = "/.well-known/cli.json" ...
const PreviewsPath = "/v1/previews" ...
const BaseURL = "https://" + Host
const DefaultLogBatchSize = 256
const DefaultLogSyncInterval = 1 * time.Second
const ErrNotFound errors.Kind = "resource not found (HTTP Status 404)"
const ErrUnexpectedResponseBody errors.Kind = "unexpected API response body"
const ErrUnexpectedStatus errors.Kind = "unexpected status code"
const Host = "api.terramate.io"
func Get[T Resource](ctx context.Context, client *Client, u url.URL) (entity T, err error)
func Patch[T Resource](ctx context.Context, client *Client, payload interface{}, url url.URL) (entity T, err error)
func Post[T Resource](ctx context.Context, client *Client, payload interface{}, url url.URL) (entity T, err error)
func Put[T Resource](ctx context.Context, client *Client, payload interface{}, url url.URL) (entity T, err error)
func Request[T Resource](ctx context.Context, c *Client, method string, url url.URL, postBody io.Reader) (entity T, err error)
type Author struct{ ... }
type ChangesetDetails struct{ ... }
type Client struct{ ... }
type CommandLog struct{ ... }
type CommandLogs []*CommandLog
type CreatePreviewOpts struct{ ... }
type CreatePreviewPayloadRequest struct{ ... }
type CreatePreviewResponse struct{ ... }
type CreatedPreview struct{ ... }
type Credential interface{ ... }
type DeploymentMetadata struct{ ... }
type DeploymentStackRequest struct{ ... }
type DeploymentStackRequests []DeploymentStackRequest
type DeploymentStackResponse struct{ ... }
type DeploymentStacksPayloadRequest struct{ ... }
type DeploymentStacksResponse []DeploymentStackResponse
type Drift struct{ ... }
type DriftStackPayloadRequest struct{ ... }
type DriftStackPayloadRequests []DriftStackPayloadRequest
type Drifts []Drift
type DriftsStackPayloadResponse struct{ ... }
type EmptyResponse string
type GitMetadata struct{ ... }
type GithubMetadata struct{ ... }
type GitlabMetadata struct{ ... }
type Label struct{ ... }
type LogChannel int
    const StdoutLogChannel LogChannel ...
type LogSyncer struct{ ... }
    func NewLogSyncer(syncfn Syncer) *LogSyncer
    func NewLogSyncerWith(syncfn Syncer, batchSize int, syncInterval time.Duration) *LogSyncer
type MemberOrganization struct{ ... }
type MemberOrganizations []MemberOrganization
type PaginatedResult struct{ ... }
type PreviewStack struct{ ... }
type PreviewStacks []PreviewStack
type Resource interface{ ... }
type ResponsePreviewStack struct{ ... }
type ResponsePreviewStacks []ResponsePreviewStack
type ReviewRequest struct{ ... }
type ReviewRequestResponse struct{ ... }
type ReviewRequestResponsePayload struct{ ... }
type ReviewRequestResponses []ReviewRequestResponse
type Reviewer Author
type Reviewers []Reviewer
type RunContext struct{ ... }
type Stack struct{ ... }
type StackObject struct{ ... }
type StacksResponse struct{ ... }
type StatusFilters struct{ ... }
    func NoStatusFilters() StatusFilters
type Syncer func(l CommandLogs)
type UUID string
type UpdateDeploymentStack struct{ ... }
type UpdateDeploymentStacks struct{ ... }
type UpdateStackPreviewOpts struct{ ... }
type UpdateStackPreviewPayloadRequest struct{ ... }
type User struct{ ... }
type WellKnown struct{ ... }
EOT

  filename = "${path.module}/mock-cloud.ignore"
}
