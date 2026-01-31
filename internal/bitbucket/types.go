package bitbucket

// PRInfo contains parsed information from a Bitbucket PR URL
type PRInfo struct {
	BaseURL    string
	ProjectKey string
	RepoSlug   string
	PRID       int
}

// PullRequestDetails contains the response from the PR API
type PullRequestDetails struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	State       string `json:"state"`
	FromRef     struct {
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
