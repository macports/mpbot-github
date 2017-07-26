package githubapi

import (
	"context"
	"regexp"

	"github.com/google/go-github/github"
)

func (client *githubClient) ListChangedPortsAndFiles(owner, repo string, number int) (ports []string, commitFiles []*github.CommitFile, err error) {
	files, _, err := client.PullRequests.ListFiles(
		context.Background(),
		owner,
		repo,
		number,
		nil,
	)
	if err != nil {
		return nil, nil, err
	}
	portfileRegexp := regexp.MustCompile(`[^\._/][^/]*/([^/]+)/Portfile`)
	for _, file := range files {
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
