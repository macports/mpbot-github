package githubapi

import (
	"context"
	"regexp"

	"github.com/google/go-github/github"
)

func (client *githubClient) GetPullRequest(owner, repo string, number int) (*github.PullRequest, error) {
	pr, _, err := client.PullRequests.Get(context.Background(), owner, repo, number)
	return pr, err
}

func (client *githubClient) ListChangedPortsAndFiles(owner, repo string, number int) (ports []string, commitFiles []*github.CommitFile, err error) {
	var allFiles []*github.CommitFile
	opt := &github.ListOptions{PerPage: 30}
	for {
		files, resp, err := client.PullRequests.ListFiles(
			context.Background(),
			owner,
			repo,
			number,
			opt,
		)
		if err != nil {
			return nil, nil, err
		}
		allFiles = append(allFiles, files...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	portfileRegexp := regexp.MustCompile(`[^\._/][^/]*/([^/]+)/Portfile`)
	for _, file := range allFiles {
		if match := portfileRegexp.FindStringSubmatch(*file.Filename); match != nil {
			ports = append(ports, match[1])
			commitFiles = append(commitFiles, file)
		}
	}
	return
}

func (client *githubClient) CreateComment(owner, repo string, number int, body *string) error {
	_, _, err := client.Issues.CreateComment(
		client.ctx,
		owner,
		repo,
		number,
		&github.IssueComment{Body: body},
	)
	return err
}

func (client *githubClient) ReplaceLabels(owner, repo string, number int, labels []string) error {
	_, _, err := client.Issues.ReplaceLabelsForIssue(
		client.ctx,
		owner,
		repo,
		number,
		labels,
	)
	return err
}

func (client *githubClient) ListLabels(owner, repo string, number int) ([]string, error) {
	labels, _, err := client.Issues.ListLabelsByIssue(
		client.ctx,
		owner,
		repo,
		number,
		&github.ListOptions{PerPage: 100},
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
