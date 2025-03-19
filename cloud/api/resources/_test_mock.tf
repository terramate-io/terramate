// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "resources" {
  content = <<-EOT
package resources // import "github.com/terramate-io/terramate/cloud/api/resources"

Package resources contains the resource entities used in the Terramate Cloud
API.

type Author struct{ ... }
type BitbucketMetadata struct{ ... }
type ChangesetDetails struct{ ... }
type CommandLog struct{ ... }
type CommandLogs []*CommandLog
type CreatePreviewPayloadRequest struct{ ... }
type CreatePreviewResponse struct{ ... }
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
type SingleSignOnDetailResponse struct{ ... }
type Stack struct{ ... }
type StackObject struct{ ... }
type StacksResponse struct{ ... }
type StatusFilters struct{ ... }
    func NoStatusFilters() StatusFilters
type StoreOutput struct{ ... }
type StoreOutputKey struct{ ... }
type StoreOutputRequest struct{ ... }
type UUID string
type UpdateDeploymentStack struct{ ... }
type UpdateDeploymentStacks struct{ ... }
type UpdateStackPreviewPayloadRequest struct{ ... }
type User struct{ ... }
type WellKnown struct{ ... }
EOT

  filename = "${path.module}/mock-resources.ignore"
}
