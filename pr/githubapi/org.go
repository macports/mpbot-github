package githubapi

import "github.com/google/go-github/github"

func (client *githubClient) ListOrgMembers(org string) ([]*github.User, error) {
	var allMembers []*github.User
	opt := &github.ListMembersOptions{ListOptions: github.ListOptions{PerPage: 30}}
	for {
		users, resp, err := client.Organizations.ListMembers(client.ctx, org, opt)
		if err != nil {
			return nil, err
		}
		allMembers = append(allMembers, users...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return allMembers, nil
}
