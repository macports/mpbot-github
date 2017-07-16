package githubapi

import (
	"context"
	"regexp"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

type Client struct {
	*github.Client
	ctx         context.Context
	owner, repo string
}

func NewClient(botSecret, owner, repo string) *Client {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: botSecret},
	)
	tc := oauth2.NewClient(ctx, ts)

	return &Client{
		Client: github.NewClient(tc),
		ctx:    ctx,
		owner:  owner,
		repo:   repo,
	}
}

func (client *Client) ListChangedPorts(number int) ([]string, error) {
	files, _, err := client.PullRequests.ListFiles(context.Background(), "macports-staging", "macports-ports", number, nil)
	if err != nil {
		return nil, err
	}
	ports := make([]string, 0, 1)
	portfileRegexp := regexp.MustCompile(`[^\._/][^/]*/([^/]+)/Portfile`)
	for _, file := range files {
		if match := portfileRegexp.FindStringSubmatch(*file.Filename); match != nil {
			ports = append(ports, match[1])
		}
	}
	return ports, nil
}

func (client *Client) CreateComment(number int, body *string) error {
	_, _, err := client.Issues.CreateComment(
		client.ctx,
		client.owner,
		client.repo,
		number,
		&github.IssueComment{Body: body},
	)
	return err
}

func (client *Client) ReplaceLabels(number int, labels []string) error {
	_, _, err := client.Issues.ReplaceLabelsForIssue(
		client.ctx,
		client.owner,
		client.repo,
		number,
		labels,
	)
	return err
}

func (client *Client) ListLabels(number int) ([]string, error) {
	labels, _, err := client.Issues.ListLabelsByIssue(
		client.ctx,
		client.owner,
		client.repo,
		number,
		nil,
	)
	if err != nil {
		return nil, err
	}
	labelNames := make([]string, 0, 1)
	for _, label := range labels {
		labelNames = append(labelNames, *label.Name)
	}
	return labelNames, nil
}
