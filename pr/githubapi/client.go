package githubapi

import (
	"context"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

type Client interface {
	GetPullRequest(owner, repo string, number int) (*github.PullRequest, error)
	ListChangedPortsAndFiles(owner, repo string, number int) (ports []string, commitFiles []*github.CommitFile, err error)
	CreateComment(owner, repo string, number int, body *string) error
	ReplaceLabels(owner, repo string, number int, labels []string) error
	ListLabels(owner, repo string, number int) ([]string, error)
	ListOrgMembers(org string) ([]*github.User, error)
}

type githubClient struct {
	*github.Client
	ctx context.Context
}

func NewClient(botSecret string) Client {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: botSecret},
	)
	tc := oauth2.NewClient(ctx, ts)

	return &githubClient{
		Client: github.NewClient(tc),
		ctx:    ctx,
	}
}
