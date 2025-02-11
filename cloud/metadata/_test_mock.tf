// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "metadata" {
  content = <<-EOT
package metadata // import "github.com/terramate-io/terramate/cloud/metadata"

Package metadata contains data structures for platform metadata that is sent
to TMC, i.e. information about CI/CD environments, pull requests, VCS details.
A large chunk of definitions can also be found in terramate/cloud/types.go.

How the metadata has been handled historically:
  - Initially, it was a flat string->string map with key prefixes for grouping
    and simple values.
  - For PR data, we needed more complex data structures that can hold lists etc,
    so a separate API object review_request was introduced, which did both hold
    new data, but also implement some logic on how to abstract pull requests
    from different structures under a single concept.

In the future, we would like to move away from this and use the following
approach:
  - Use a single API object. We keep using the existing metadata map, but relax
    it to accept string->any.
  - Group related data by storing them under a top-level key in metadata.
    No longer flatten data types into prefixed keys.
  - Do not abstract between different platforms on the CLI level, instead send
    the data as-is, i.e. "github_pull_request": {...}, "gitlab_merge_request":
    {...}, each having different definitions.

type BitbucketActor struct{ ... }
    func NewBitbucketActor(in *bitbucket.Actor) (*BitbucketActor, error)
type BitbucketPullRequest struct{ ... }
    func NewBitbucketPullRequest(in *bitbucket.PR) (*BitbucketPullRequest, error)
type BitbucketUser struct{ ... }
    func NewBitbucketUser(in *bitbucket.User) (*BitbucketUser, error)
type CommitAuthor struct{ ... }
    func NewCommitAuthor(in *github.CommitAuthor) (*CommitAuthor, error)
type GithubCommit struct{ ... }
    func NewGithubCommit(in *github.RepositoryCommit) (*GithubCommit, error)
type GithubPullRequest struct{ ... }
    func NewGithubPullRequest(inPR *github.PullRequest, inReviews []*github.PullRequestReview, ...) (*GithubPullRequest, error)
type GithubPullRequestReview struct{ ... }
    func NewGithubPullRequestReview(in *github.PullRequestReview) (*GithubPullRequestReview, error)
type GithubTeam struct{ ... }
    func NewGithubTeam(in *github.Team) (*GithubTeam, error)
type GithubUser struct{ ... }
    func NewGithubUser(in *github.User) (*GithubUser, error)
type GitlabMergeRequest struct{ ... }
    func NewGitlabMergeRequest(inMR *gitlab.MR, inReviewers []gitlab.MRReviewer, inParticipants []gitlab.User) (*GitlabMergeRequest, error)
type GitlabMergeRequestReviewer struct{ ... }
    func NewGitlabMergeRequestReviewer(in *gitlab.MRReviewer) (*GitlabMergeRequestReviewer, error)
type GitlabUser struct{ ... }
    func NewGitlabUser(in *gitlab.User) (*GitlabUser, error)
EOT

  filename = "${path.module}/mock-metadata.ignore"
}
