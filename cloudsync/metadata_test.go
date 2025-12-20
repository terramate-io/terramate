package cloudsync

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/terramate-io/terramate/cloud/integrations/bitbucket"
)

func TestFindMatchingBitbucketPR(t *testing.T) {
	t.Parallel()

	testCommit := "aabbccddeeff00112233445566778899aabbccdd"
	testShortCommit := "aabbccddeeff"

	tests := []struct {
		name       string
		prs        []bitbucket.PR
		commit     string
		branch     string
		destBranch string
		want       *bitbucket.PR
	}{
		{
			name: "matches by branch names",
			prs: []bitbucket.PR{
				{
					ID: 1,
					Source: bitbucket.TargetBranch{
						Branch: bitbucket.Branch{Name: "feature"},
					},
					Destination: bitbucket.TargetBranch{
						Branch: bitbucket.Branch{Name: "main"},
					},
				},
			},
			commit:     "whatever",
			branch:     "feature",
			destBranch: "main",
			want: &bitbucket.PR{
				ID: 1,
				Source: bitbucket.TargetBranch{
					Branch: bitbucket.Branch{Name: "feature"},
				},
				Destination: bitbucket.TargetBranch{
					Branch: bitbucket.Branch{Name: "main"},
				},
			},
		},
		{
			name: "matches by merge commit hash",
			prs: []bitbucket.PR{
				{
					ID: 2,
					MergeCommit: bitbucket.Commit{
						ShortHash: testShortCommit,
					},
				},
			},
			commit: testCommit,
			want: &bitbucket.PR{
				ID: 2,
				MergeCommit: bitbucket.Commit{
					ShortHash: testShortCommit,
				},
			},
		},
		{
			name: "matches by source commit hash",
			prs: []bitbucket.PR{
				{
					ID: 3,
					Source: bitbucket.TargetBranch{
						Commit: bitbucket.Commit{
							ShortHash: testShortCommit,
						},
					},
				},
			},
			commit: testCommit,
			want: &bitbucket.PR{
				ID: 3,
				Source: bitbucket.TargetBranch{
					Commit: bitbucket.Commit{
						ShortHash: testShortCommit,
					},
				},
			},
		},
		{
			name: "does not match empty merge commit hash",
			prs: []bitbucket.PR{
				{
					ID: 4,
					MergeCommit: bitbucket.Commit{
						ShortHash: "",
					},
				},
			},
			commit: testCommit,
			want:   nil,
		},
		{
			name: "does not match empty source commit hash",
			prs: []bitbucket.PR{
				{
					ID: 5,
					Source: bitbucket.TargetBranch{
						Commit: bitbucket.Commit{
							ShortHash: "",
						},
					},
				},
			},
			commit: testCommit,
			want:   nil,
		},
		{
			name: "prioritizes branch match over commit match",
			prs: []bitbucket.PR{
				{
					ID: 6,
					Source: bitbucket.TargetBranch{
						Branch: bitbucket.Branch{Name: "feature"},
						Commit: bitbucket.Commit{ShortHash: "other"},
					},
					Destination: bitbucket.TargetBranch{
						Branch: bitbucket.Branch{Name: "main"},
					},
				},
			},
			commit:     testCommit, // Matches nothing
			branch:     "feature",
			destBranch: "main",
			want: &bitbucket.PR{
				ID: 6,
				Source: bitbucket.TargetBranch{
					Branch: bitbucket.Branch{Name: "feature"},
					Commit: bitbucket.Commit{ShortHash: "other"},
				},
				Destination: bitbucket.TargetBranch{
					Branch: bitbucket.Branch{Name: "main"},
				},
			},
		},
		{
			name: "matches when commit is substring of PR hash (reverse prefix)",
			prs: []bitbucket.PR{
				{
					ID: 7,
					Source: bitbucket.TargetBranch{
						Commit: bitbucket.Commit{
							ShortHash: testCommit, // PR has long hash
						},
					},
				},
			},
			commit: testShortCommit, // Input is short hash
			want: &bitbucket.PR{
				ID: 7,
				Source: bitbucket.TargetBranch{
					Commit: bitbucket.Commit{
						ShortHash: testCommit,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := findMatchingBitbucketPR(tt.prs, tt.commit, tt.branch, tt.destBranch)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("want != got:\n%v", diff)
			}
		})
	}
}
