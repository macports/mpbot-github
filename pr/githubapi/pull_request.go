package githubapi

import (
	"context"
	"github.com/google/go-github/github"
	"regexp"
)

func ListChangedPorts(number int) []string {
	client := github.NewClient(nil)
	files, _, err := client.PullRequests.ListFiles(context.Background(), "macports-staging", "macports-ports", number, nil)
	if err != nil {
		return nil
	}
	ports := make([]string, 0, 1)
	portfileRegexp := regexp.MustCompile(`[^\._/][^/]*/([^/]+)/Portfile`)
	for _, file := range files {
		if match := portfileRegexp.FindStringSubmatch(*file.Filename); match != nil {
			ports = append(ports, match[1])
		}
	}
	return ports
}
