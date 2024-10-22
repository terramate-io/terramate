// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "testserver" {
  content = <<-EOT
package testserver // import "github.com/terramate-io/terramate/cloud/testserver"

Package testserver provides fake Terramate Cloud endpoints for testing purposes.

func DisableEndpoints(paths ...string) map[string]bool
func EnableAllConfig() map[string]bool
func GetCommit(store *cloudstore.Data, w http.ResponseWriter, _ *http.Request, ...)
func GetDeploymentLogs(store *cloudstore.Data, w http.ResponseWriter, _ *http.Request, ...)
func GetDeploymentLogsEvents(store *cloudstore.Data, w http.ResponseWriter, _ *http.Request, ...)
func GetDeployments(store *cloudstore.Data, w http.ResponseWriter, _ *http.Request, ...)
func GetDrift(store *cloudstore.Data, w http.ResponseWriter, _ *http.Request, ...)
func GetDrifts(store *cloudstore.Data, w http.ResponseWriter, _ *http.Request, ...)
func GetMemberships(store *cloudstore.Data, w http.ResponseWriter, r *http.Request, ...)
func GetPreview(store *cloudstore.Data, w http.ResponseWriter, _ *http.Request, ...)
func GetPullRequest(store *cloudstore.Data, w http.ResponseWriter, _ *http.Request, ...)
func GetStackDrifts(store *cloudstore.Data, w http.ResponseWriter, r *http.Request, ...)
func GetStacks(store *cloudstore.Data, w http.ResponseWriter, r *http.Request, ...)
func GetUsers(store *cloudstore.Data, w http.ResponseWriter, r *http.Request, ...)
func GetWellKnown(store *cloudstore.Data, w http.ResponseWriter, _ *http.Request, ...)
func ListCheckRunsForRef(_ *cloudstore.Data, w http.ResponseWriter, _ *http.Request, ...)
func ListPullRequestsWithCommit(_ *cloudstore.Data, w http.ResponseWriter, _ *http.Request, ...)
func ListReviews(_ *cloudstore.Data, w http.ResponseWriter, _ *http.Request, ...)
func PatchDeployment(store *cloudstore.Data, w http.ResponseWriter, r *http.Request, ...)
func PatchStackPreviews(store *cloudstore.Data, w http.ResponseWriter, r *http.Request, ...)
func PostDeployment(store *cloudstore.Data, w http.ResponseWriter, r *http.Request, ...)
func PostDeploymentLogs(store *cloudstore.Data, w http.ResponseWriter, r *http.Request, ...)
func PostDrift(store *cloudstore.Data, w http.ResponseWriter, r *http.Request, ...)
func PostPreviews(store *cloudstore.Data, w http.ResponseWriter, r *http.Request, ...)
func PostStackPreviewsLogs(store *cloudstore.Data, w http.ResponseWriter, r *http.Request, ...)
func PullRequestIsMerged(_ *cloudstore.Data, w http.ResponseWriter, _ *http.Request, ...)
func PutStack(store *cloudstore.Data, w http.ResponseWriter, r *http.Request, ...)
func Router(store *cloudstore.Data) *httprouter.Router
func RouterAdd(store *cloudstore.Data, router *httprouter.Router, enabled map[string]bool)
func RouterAddCustoms(router *httprouter.Router, store *cloudstore.Data, custom Custom)
func RouterWith(store *cloudstore.Data, enabled map[string]bool) *httprouter.Router
type Custom struct{ ... }
type Handler func(store *cloudstore.Data, w http.ResponseWriter, r *http.Request, ...)
type Route struct{ ... }
EOT

  filename = "${path.module}/mock-testserver.ignore"
}
