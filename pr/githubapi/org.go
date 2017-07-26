package githubapi

import "github.com/google/go-github/github"

func (client *githubClient) ListOrgMembers(org string) ([]*github.User, error) {
	users, _, err := client.Organizations.ListMembers(client.ctx, org, nil)
	return users, err
}
