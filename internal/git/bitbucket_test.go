package git_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/subosito/gotenv"
	"github.com/vossenwout/crev/internal/git"
)

func Test_ParsePRURL(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		wantBaseURL string
		wantProject string
		wantRepo    string
		wantPRID    int
		wantErr     bool
	}{
		{
			name:        "valid URL with overview",
			url:         "https://aslbitbucket.asldt.in/projects/BH/repos/crobat/pull-requests/50/overview",
			wantBaseURL: "https://aslbitbucket.asldt.in",
			wantProject: "BH",
			wantRepo:    "crobat",
			wantPRID:    50,
			wantErr:     false,
		},
		{
			name:        "valid URL without overview",
			url:         "https://aslbitbucket.asldt.in/projects/BH/repos/crobat/pull-requests/50",
			wantBaseURL: "https://aslbitbucket.asldt.in",
			wantProject: "BH",
			wantRepo:    "crobat",
			wantPRID:    50,
			wantErr:     false,
		},
		{
			name:    "invalid URL",
			url:     "https://example.com/invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prInfo, err := git.ParsePRURL(tt.url)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantBaseURL, prInfo.BaseURL)
			assert.Equal(t, tt.wantProject, prInfo.ProjectKey)
			assert.Equal(t, tt.wantRepo, prInfo.RepoSlug)
			assert.Equal(t, tt.wantPRID, prInfo.PRID)
		})
	}
}

func Test_GetPullRequestFromURL(t *testing.T) {
	err := gotenv.Load("../../.env")
	assert.NoError(t, err)

	ctx := context.Background()
	username := os.Getenv("BITBUCKET_USERNAME")
	password := os.Getenv("BITBUCKET_PASSWORD")

	assert.NotEmpty(t, username)
	assert.NotEmpty(t, password)

	// Test with PR URL
	prURL := "https://aslbitbucket.asldt.in/projects/BH/repos/crobat/pull-requests/50/overview"

	git, err := git.NewGitBitbucketFromURL(prURL, username, password)
	assert.NoError(t, err)

	// Test GetPullRequestDetails to get branch names
	prDetails, err := git.GetPullRequestDetails(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, prDetails)

	t.Logf("PR Title: %s", prDetails.Title)
	t.Logf("Source Branch: %s", prDetails.FromRef.DisplayID)
	t.Logf("Destination Branch: %s", prDetails.ToRef.DisplayID)

	assert.NotEmpty(t, prDetails.FromRef.DisplayID, "Source branch should not be empty")
	assert.NotEmpty(t, prDetails.ToRef.DisplayID, "Destination branch should not be empty")

	// Test GetCloneURL
	cloneURL := git.GetCloneURL()
	t.Logf("Clone URL: %s", cloneURL)
	assert.Contains(t, cloneURL, "aslbitbucket.asldt.in")
	assert.Contains(t, cloneURL, "/scm/BH/crobat.git")
}

func Test_CloneRepository(t *testing.T) {
	err := gotenv.Load("../../.env")
	assert.NoError(t, err)

	ctx := context.Background()
	username := os.Getenv("BITBUCKET_USERNAME")
	password := os.Getenv("BITBUCKET_PASSWORD")

	assert.NotEmpty(t, username)
	assert.NotEmpty(t, password)

	prURL := "https://aslbitbucket.asldt.in/projects/BHR/repos/rsrc-mst-security/pull-requests/30/overview"

	git, err := git.NewGitBitbucketFromURL(prURL, username, password)
	assert.NoError(t, err)

	// Create a temporary directory for cloning
	tempDir, err := os.MkdirTemp("", "bitbucket-clone-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir) // Cleanup after test

	t.Logf("Cloning to: %s", tempDir)

	// Clone the repository
	result, err := git.CloneRepository(ctx, tempDir)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	t.Logf("Clone Result:")
	t.Logf("  Path: %s", result.Path)
	t.Logf("  Source Branch: %s", result.SourceBranch)
	t.Logf("  Dest Branch: %s", result.DestBranch)
	t.Logf("  Clone URL: %s", result.CloneURL)

	assert.Equal(t, tempDir, result.Path)
	assert.NotEmpty(t, result.SourceBranch)
	assert.NotEmpty(t, result.DestBranch)

	// Verify that some files exist in the cloned directory
	entries, err := os.ReadDir(tempDir)
	assert.NoError(t, err)
	assert.NotEmpty(t, entries, "Cloned directory should not be empty")

	t.Logf("Cloned %d files/directories", len(entries))
}

// Test_ListOpenPullRequests requires BITBUCKET_USERNAME and BITBUCKET_PASSWORD in .env.
func Test_ListOpenPullRequests(t *testing.T) {
	err := gotenv.Load("../../.env")
	assert.NoError(t, err)

	ctx := context.Background()
	username := os.Getenv("BITBUCKET_USERNAME")
	password := os.Getenv("BITBUCKET_PASSWORD")

	assert.NotEmpty(t, username)
	assert.NotEmpty(t, password)

	prURL := "https://aslbitbucket.asldt.in/projects/BH/repos/crobat/pull-requests/63/overview"

	g, err := git.NewGitBitbucketFromURL(prURL, username, password)
	assert.NoError(t, err)

	prs, err := g.ListOpenPullRequests(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, prs)

	for _, pr := range prs {
		assert.Equal(t, "OPEN", pr.State, "PR %d should be open", pr.ID)
		assert.NotZero(t, pr.ID)
		assert.NotEmpty(t, pr.Title)
	}

	t.Logf("ListOpenPullRequests returned %d open PR(s)", len(prs))
}
