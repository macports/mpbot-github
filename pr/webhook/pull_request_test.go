package webhook

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/google/go-github/github"
	"github.com/macports/mpbot-github/pr/db"
)

var errNotFound = errors.New("404")

func TestCVERegexp(t *testing.T) {
	assert.Equal(t, "CVE-2017-0001", cveRegexp.FindString("Fixes CVE-2017-0001."))
	assert.Equal(t, "", cveRegexp.FindString("CVE-pending."))
}

type PullRequestEventTest struct {
	number  int
	sender  string
	title   string
	body    string
	comment string
	labels  []string
}

func TestHandlePullRequest(t *testing.T) {
	stubClient := stubGitHubClient{}
	receiver := &Receiver{
		githubClient: &stubClient,
		dbHelper:     &stubDBHelper{},
		testing:      true,
	}
	var event github.PullRequestEvent
	json.Unmarshal([]byte(`{
  "action": "opened",
  "number": 1,
  "pull_request": {
    "number": 1,
    "state": "open",
    "title": "",
    "user": {
      "login": "l2dy"
    },
    "body": ""
  },
  "repository": {
    "name": "macports-ports",
    "owner": {
      "login": "macports"
    }
  },
  "sender": {
    "login": "l2dy"
  }
}`), &event)
	prTests := []*PullRequestEventTest{
		{number: 1, sender: "l2dy", title: "z: update to 1.1", labels: []string{"maintainer: none", "type: update"}},
		{number: 1, sender: "l2dy", title: "z: update to 1.1", body: "[x] enhancement", labels: []string{"maintainer: none", "type: update", "type: enhancement"}},
		{number: 1, sender: "l2dy", title: "z: update to 1.1", body: "Fixes CVE-0000-0.", labels: []string{"maintainer: none", "type: update", "type: security fix"}},
		{number: 2, sender: "l2dy", title: "upx-devel: new port", labels: []string{"type: submission"}},
		{number: 3, sender: "l2dy", title: "upx: update to 1.1", labels: []string{"maintainer", "maintainer: open", "type: update"}},
		{number: 3, sender: "jverne", title: "upx: update to 1.1", comment: "Notifying maintainers:\n@_l2dy for port upx.\n\nBy a harmless bot.", labels: []string{"maintainer: open", "type: update"}},
		{number: 3, sender: "jverne", title: "upx: update to 1.1", body: "<!-- [skip notification] -->", labels: []string{"maintainer: open", "type: update"}},
	}
	for _, prt := range prTests {
		stubClient.newComment = ""
		stubClient.newLabels = nil
		event.Number = &prt.number
		event.Sender.Login = &prt.sender
		event.PullRequest.Title = &prt.title
		event.PullRequest.Body = &prt.body
		eventBody, err := json.Marshal(event)
		if err != nil {
			t.Error(err)
		}
		receiver.handlePullRequest(eventBody)
		assert.Equal(t, prt.comment, stubClient.newComment)
		assert.Subset(t, stubClient.newLabels, prt.labels)
		assert.Subset(t, prt.labels, stubClient.newLabels)
	}
}

type stubGitHubClient struct {
	newComment string
	newLabels  []string
}

func (stub *stubGitHubClient) ListChangedPortsAndFiles(owner, repo string, number int) (ports []string, commitFiles []*github.CommitFile, err error) {
	if owner != "macports" || repo != "macports-ports" {
		return nil, nil, errNotFound
	}
	switch number {
	case 1:
		return []string{"z"},
			[]*github.CommitFile{
				{
					Filename: ptrOfStr("sysutils/z/Portfile"),
					Status:   ptrOfStr("modified"),
					Changes:  ptrOfInt(6),
				},
			}, nil
	case 2:
		return []string{"upx-devel"},
			[]*github.CommitFile{
				{
					Filename: ptrOfStr("archivers/upx-devel/Portfile"),
					Status:   ptrOfStr("added"),
					Changes:  ptrOfInt(43),
				},
			}, nil
	case 3:
		return []string{"upx"},
			[]*github.CommitFile{
				{
					Filename: ptrOfStr("archivers/upx/Portfile"),
					Status:   ptrOfStr("modified"),
					Changes:  ptrOfInt(6),
				},
			}, nil
	default:
		return nil, nil, errNotFound
	}
}

func (stub *stubGitHubClient) CreateComment(owner, repo string, number int, body *string) error {
	stub.newComment = *body
	return nil
}

func (stub *stubGitHubClient) ReplaceLabels(owner, repo string, number int, labels []string) error {
	stub.newLabels = labels
	return nil
}

func (stub *stubGitHubClient) ListLabels(owner, repo string, number int) ([]string, error) {
	if owner != "macports" || repo != "macports-ports" {
		return nil, errNotFound
	}
	return nil, nil
}

func (stub *stubGitHubClient) ListOrgMembers(org string) ([]*github.User, error) {
	return []*github.User{
		{Login: ptrOfStr("l2dy")},
	}, nil
}

type stubDBHelper struct{}

func (stub *stubDBHelper) GetGitHubHandle(email string) (string, error) {
	if email == "l2dy@macports.org" {
		return "l2dy", nil
	}
	return "", errNotFound
}

func (stub *stubDBHelper) GetPortMaintainer(port string) (*db.PortMaintainer, error) {
	if port == "upx" {
		return &db.PortMaintainer{
			Primary: &db.Maintainer{
				GithubHandle: "l2dy",
				Email:        "l2dy@macports.org",
			},
			NoMaintainer:   false,
			OpenMaintainer: true,
		}, nil
	}
	if port == "z" {
		return &db.PortMaintainer{
			Primary:        nil,
			NoMaintainer:   true,
			OpenMaintainer: false,
		}, nil
	}
	return nil, errors.New("port not found")
}

func ptrOfStr(s string) *string {
	return &s
}

func ptrOfInt(s int) *int {
	return &s
}
