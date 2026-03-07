package git

import "context"

// PRComment represents a comment on a pull request.
type PRComment struct {
	ID   int64  `json:"id"`
	Text string `json:"text"`
}

type Git interface {
	GetPullRequestDetails(ctx context.Context) (*PullRequestDetails, error)
	ListOpenPullRequests(ctx context.Context) ([]*PullRequestDetails, error)
	CloneRepository(ctx context.Context, destPath string) (*CloneResult, error)
	GetCloneURL() string
	PostComment(ctx context.Context, comment string) error
	RequestChanges(ctx context.Context) error
	Approve(ctx context.Context) error
	GetPRComments(ctx context.Context) ([]PRComment, error)
	WithPRID(prID int) Git
}
