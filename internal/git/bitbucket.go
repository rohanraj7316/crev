package git

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)

// PRInfo contains parsed information from a Bitbucket PR URL
type PRInfo struct {
	BaseURL    string
	ProjectKey string
	RepoSlug   string
	PRID       int
}

// PRParticipant represents a participant in a pull request (reviewer/assignee)
type PRParticipant struct {
	User struct {
		Name         string `json:"name"`
		EmailAddress string `json:"emailAddress"`
		Slug         string `json:"slug"`
	} `json:"user"`
	Role     string `json:"role"`     // e.g. "REVIEWER", "AUTHOR"
	Approved bool   `json:"approved"`
}

// PullRequestDetails contains the response from the PR API
type PullRequestDetails struct {
	ID           int              `json:"id"`
	Title        string           `json:"title"`
	Description  string           `json:"description"`
	State        string           `json:"state"`
	Participants []PRParticipant `json:"participants,omitempty"`
	FromRef      struct {
		ID           string `json:"id"`
		DisplayID    string `json:"displayId"`
		LatestCommit string `json:"latestCommit"`
		Repository   struct {
			Slug    string `json:"slug"`
			Project struct {
				Key string `json:"key"`
			} `json:"project"`
		} `json:"repository"`
	} `json:"fromRef"`
	ToRef struct {
		ID           string `json:"id"`
		DisplayID    string `json:"displayId"`
		LatestCommit string `json:"latestCommit"`
		Repository   struct {
			Slug    string `json:"slug"`
			Project struct {
				Key string `json:"key"`
			} `json:"project"`
		} `json:"repository"`
	} `json:"toRef"`
}

// CloneResult contains the result of a clone operation
type CloneResult struct {
	Path         string // Local path where repo was cloned
	SourceBranch string // Source branch name (from PR)
	DestBranch   string // Destination branch name (from PR)
	CloneURL     string // URL used for cloning
}

// ParticipantStatusRequest represents the request body for updating participant status
type ParticipantStatusRequest struct {
	Status string `json:"status"`
}

// CommentRequest represents the request body for posting a PR comment
type CommentRequest struct {
	Text string `json:"text"`
}

// activitiesPage is the Bitbucket Server paged response for PR activities
type activitiesPage struct {
	Values        []activityItem `json:"values"`
	IsLastPage    bool           `json:"isLastPage"`
	NextPageStart int            `json:"nextPageStart"`
}

type activityItem struct {
	Action  string `json:"action"`
	Comment struct {
		ID   int64  `json:"id"`
		Text string `json:"text"`
	} `json:"comment"`
}

type git struct {
	baseURL    string
	username   string
	password   string
	projectKey string
	repoSlug   string
	prID       int
	httpClient *http.Client
}

// ParsePRURL parses a Bitbucket Server PR URL and extracts components
// Example URL: https://aslbitbucket.asldt.in/projects/BH/repos/crobat/pull-requests/50/overview
func ParsePRURL(prURL string) (*PRInfo, error) {
	// Pattern: {baseURL}/projects/{projectKey}/repos/{repoSlug}/pull-requests/{prID}[/...]
	pattern := regexp.MustCompile(`^(https?://[^/]+)/projects/([^/]+)/repos/([^/]+)/pull-requests/(\d+)`)
	matches := pattern.FindStringSubmatch(prURL)

	if len(matches) != 5 {
		return nil, fmt.Errorf("invalid PR URL format: %s", prURL)
	}

	prID, err := strconv.Atoi(matches[4])
	if err != nil {
		return nil, fmt.Errorf("invalid PR ID in URL: %s", matches[4])
	}

	return &PRInfo{
		BaseURL:    matches[1],
		ProjectKey: matches[2],
		RepoSlug:   matches[3],
		PRID:       prID,
	}, nil
}

// NewGitBitbucketFromURL creates a new Git client from a PR URL
func NewGitBitbucketFromURL(prURL, username, password string) (Git, error) {
	prInfo, err := ParsePRURL(prURL)
	if err != nil {
		return nil, err
	}

	client := &git{
		baseURL:    prInfo.BaseURL,
		username:   username,
		password:   password,
		projectKey: prInfo.ProjectKey,
		repoSlug:   prInfo.RepoSlug,
		prID:       prInfo.PRID,
		httpClient: &http.Client{},
	}

	return client, nil
}

// WithPRID returns a new Git client scoped to the given PR ID (same repo, different PR).
func (g *git) WithPRID(prID int) Git {
	return &git{
		baseURL:    g.baseURL,
		username:   g.username,
		password:   g.password,
		projectKey: g.projectKey,
		repoSlug:   g.repoSlug,
		prID:       prID,
		httpClient: g.httpClient,
	}
}

// pullRequestsPage is the Bitbucket Server paged response for list pull-requests
type pullRequestsPage struct {
	Size          int                  `json:"size"`
	Limit         int                  `json:"limit"`
	IsLastPage    bool                 `json:"isLastPage"`
	NextPageStart int                  `json:"nextPageStart"`
	Values        []PullRequestDetails `json:"values"`
}

// ListOpenPullRequests returns all open PRs for the repository that are assigned to the current user
// (i.e. the authenticated user appears as a participant/reviewer on the PR).
func (g *git) ListOpenPullRequests(ctx context.Context) ([]*PullRequestDetails, error) {
	all, err := g.listOpenPullRequestsRaw(ctx)
	if err != nil {
		return nil, err
	}

	userSlug := strings.ReplaceAll(g.username, "@", "_")
	var assigned []*PullRequestDetails
	for _, pr := range all {
		// If list response omitted participants, fetch full details for this PR
		if len(pr.Participants) == 0 {
			detail, err := g.getPullRequestDetailsByID(ctx, pr.ID)
			if err != nil {
				continue // skip on error to avoid failing entire list
			}
			pr = detail
		}
		if isAssignedToUser(pr, userSlug, g.username) {
			assigned = append(assigned, pr)
		}
	}
	return assigned, nil
}

// isAssignedToUser returns true if the PR has the given user as a participant (by slug or name).
func isAssignedToUser(pr *PullRequestDetails, userSlug, username string) bool {
	for _, p := range pr.Participants {
		if p.User.Slug == userSlug || p.User.Name == username || strings.EqualFold(p.User.EmailAddress, username) {
			return true
		}
	}
	return false
}

// listOpenPullRequestsRaw fetches all open PRs for the repository (no assignee filter).
func (g *git) listOpenPullRequestsRaw(ctx context.Context) ([]*PullRequestDetails, error) {
	var all []*PullRequestDetails
	start := 0
	limit := 100

	for {
		apiURL := fmt.Sprintf("%s/rest/api/1.0/projects/%s/repos/%s/pull-requests?state=OPEN&limit=%d&start=%d",
			g.baseURL, g.projectKey, g.repoSlug, limit, start)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.SetBasicAuth(g.username, g.password)
		req.Header.Set("Accept", "application/json")

		resp, err := g.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to execute request: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(body))
		}

		var page pullRequestsPage
		if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		resp.Body.Close()

		for i := range page.Values {
			all = append(all, &page.Values[i])
		}

		if page.IsLastPage {
			break
		}
		start = page.NextPageStart
	}

	return all, nil
}

// getPullRequestDetailsByID fetches full PR details by ID (includes participants).
func (g *git) getPullRequestDetailsByID(ctx context.Context, prID int) (*PullRequestDetails, error) {
	apiURL := fmt.Sprintf("%s/rest/api/1.0/projects/%s/repos/%s/pull-requests/%d",
		g.baseURL, g.projectKey, g.repoSlug, prID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(g.username, g.password)
	req.Header.Set("Accept", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(body))
	}

	var prDetails PullRequestDetails
	if err := json.NewDecoder(resp.Body).Decode(&prDetails); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &prDetails, nil
}

// GetPullRequestDetails fetches PR details including branch information
func (g *git) GetPullRequestDetails(ctx context.Context) (*PullRequestDetails, error) {
	return g.getPullRequestDetailsByID(ctx, g.prID)
}

// GetCloneURL returns the HTTPS clone URL for the repository
// Format: https://bitbucket.server.com/scm/{projectKey}/{repoSlug}.git
func (g *git) GetCloneURL() string {
	return fmt.Sprintf("%s/scm/%s/%s.git", g.baseURL, g.projectKey, g.repoSlug)
}

// CloneRepository clones the repository with both source and destination branches from the PR
func (g *git) CloneRepository(ctx context.Context, destPath string) (*CloneResult, error) {
	// First, get PR details to know which branches to fetch
	prDetails, err := g.GetPullRequestDetails(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get PR details: %w", err)
	}

	sourceBranch := prDetails.FromRef.DisplayID // Branch with PR changes
	destBranch := prDetails.ToRef.DisplayID     // Target branch (e.g., dev, main)
	cloneURL := g.GetCloneURL()

	// Create destination directory if it doesn't exist
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Build clone URL with credentials for authentication
	parsedURL, err := url.Parse(cloneURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse clone URL: %w", err)
	}

	auth := &githttp.BasicAuth{
		Username: g.username,
		Password: g.password,
	}

	// Clone the repository with the destination branch first (usually smaller/faster)
	repo, err := gogit.PlainCloneContext(ctx, destPath, false, &gogit.CloneOptions{
		URL:           parsedURL.String(),
		Auth:          auth,
		Progress:      os.Stdout,
		ReferenceName: plumbing.NewBranchReferenceName(destBranch),
		SingleBranch:  false, // We need multiple branches for git diff
	})
	if err != nil {
		return nil, fmt.Errorf("failed to clone repository: %w", err)
	}

	// Fetch the source branch (PR branch)
	err = repo.Fetch(&gogit.FetchOptions{
		RefSpecs: []config.RefSpec{
			config.RefSpec(fmt.Sprintf("+refs/heads/%s:refs/remotes/origin/%s", sourceBranch, sourceBranch)),
		},
		Auth:     auth,
		Progress: os.Stdout,
	})
	if err != nil && err != gogit.NoErrAlreadyUpToDate {
		return nil, fmt.Errorf("failed to fetch source branch %s: %w", sourceBranch, err)
	}

	// Get worktree
	worktree, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("failed to get worktree: %w", err)
	}

	// Create local branch for source from remote tracking branch
	remoteRef, err := repo.Reference(plumbing.NewRemoteReferenceName("origin", sourceBranch), true)
	if err != nil {
		return nil, fmt.Errorf("failed to get remote reference for source branch: %w", err)
	}

	// Create local source branch pointing to the remote ref
	localSourceRef := plumbing.NewHashReference(plumbing.NewBranchReferenceName(sourceBranch), remoteRef.Hash())
	if err := repo.Storer.SetReference(localSourceRef); err != nil {
		return nil, fmt.Errorf("failed to create local source branch: %w", err)
	}

	// Checkout the source branch (the PR branch with changes)
	err = worktree.Checkout(&gogit.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(sourceBranch),
		Force:  true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to checkout source branch: %w", err)
	}

	return &CloneResult{
		Path:         destPath,
		SourceBranch: sourceBranch,
		DestBranch:   destBranch,
		CloneURL:     cloneURL,
	}, nil
}

// GetPRComments returns all comments on the pull request (from the activities stream).
func (g *git) GetPRComments(ctx context.Context) ([]PRComment, error) {
	var all []PRComment
	start := 0
	limit := 100

	for {
		apiURL := fmt.Sprintf("%s/rest/api/1.0/projects/%s/repos/%s/pull-requests/%d/activities?limit=%d&start=%d",
			g.baseURL, g.projectKey, g.repoSlug, g.prID, limit, start)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.SetBasicAuth(g.username, g.password)
		req.Header.Set("Accept", "application/json")

		resp, err := g.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to execute request: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(body))
		}

		var page activitiesPage
		if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		resp.Body.Close()

		for _, item := range page.Values {
			if item.Action == "COMMENTED" && item.Comment.Text != "" {
				all = append(all, PRComment{ID: item.Comment.ID, Text: item.Comment.Text})
			}
		}

		if page.IsLastPage {
			break
		}
		start = page.NextPageStart
	}

	return all, nil
}

// PostComment posts a comment to the pull request
func (g *git) PostComment(ctx context.Context, comment string) error {
	// Bitbucket Server API endpoint for PR comments
	// POST /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/comments
	apiURL := fmt.Sprintf("%s/rest/api/1.0/projects/%s/repos/%s/pull-requests/%d/comments",
		g.baseURL, g.projectKey, g.repoSlug, g.prID)

	// Create request body
	reqBody := CommentRequest{
		Text: comment,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal comment: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(g.username, g.password)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: %s - %s", resp.Status, string(body))
	}

	return nil
}

// RequestChanges sets the PR status to "Needs Work" (Request Changes)
func (g *git) RequestChanges(ctx context.Context) error {
	// Bitbucket Server API endpoint for updating participant status
	// PUT /rest/api/latest/projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/participants/{userSlug}
	// The userSlug has @ replaced with _ in the username
	userSlug := strings.ReplaceAll(g.username, "@", "_")

	apiURL := fmt.Sprintf("%s/rest/api/latest/projects/%s/repos/%s/pull-requests/%d/participants/%s?avatarSize=48&version=0",
		g.baseURL, g.projectKey, g.repoSlug, g.prID, userSlug)

	// Create request body with NEEDS_WORK status
	reqBody := ParticipantStatusRequest{
		Status: "NEEDS_WORK",
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal status request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(g.username, g.password)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: %s - %s", resp.Status, string(body))
	}

	return nil
}

// Approve sets the PR status to Approved for the current user.
func (g *git) Approve(ctx context.Context) error {
	userSlug := strings.ReplaceAll(g.username, "@", "_")

	apiURL := fmt.Sprintf("%s/rest/api/latest/projects/%s/repos/%s/pull-requests/%d/participants/%s?avatarSize=48&version=0",
		g.baseURL, g.projectKey, g.repoSlug, g.prID, userSlug)

	reqBody := ParticipantStatusRequest{
		Status: "APPROVED",
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal status request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(g.username, g.password)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: %s - %s", resp.Status, string(body))
	}

	return nil
}
