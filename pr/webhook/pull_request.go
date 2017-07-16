package webhook

import (
	"encoding/json"
	"strings"

	"github.com/google/go-github/github"
	"github.com/macports/mpbot-github/pr/db"
	"log"
	"strconv"
)

func (receiver *Receiver) handlePullRequest(body []byte) {
	defer func() {
		if r := recover(); r != nil {
			log.Println(r)
		}
	}()

	event := &github.PullRequestEvent{}
	err := json.Unmarshal(body, event)
	if err != nil {
		// TODO: log
		return
	}
	number := *event.Number
	owner := *event.Repo.Owner.Login
	repo := *event.Repo.Name
	isOpenmaintainer := true
	isNomaintainer := true
	isMaintainer := false
	ports, err := receiver.githubClient.ListChangedPorts(number)
	if err != nil {
		return
	}
	handles := make([]string, 0, 1)
	for _, port := range ports {
		maintainer, err := db.GetMaintainer(port)
		if err != nil {
			continue
		}
		isNomaintainer = isNomaintainer && maintainer.NoMaintainer
		isOpenmaintainer = isOpenmaintainer && (maintainer.OpenMaintainer || maintainer.NoMaintainer)
		if maintainer.NoMaintainer {
			continue
		}
		if maintainer.Primary.GithubHandle != "" {
			handles = append(handles, maintainer.Primary.GithubHandle)
			if maintainer.Primary.GithubHandle == *event.Sender.Login {
				// TODO: should be set only when the sender is maintainer of all modified ports
				isMaintainer = true
			}
		}
	}

	switch *event.Action {
	case "opened":
		// Notify maintainers
		mentionSymbol := "@_"
		if receiver.production {
			mentionSymbol = "@"
		}
		if len(handles) > 0 {
			body := "Notifying maintainers: " + mentionSymbol + strings.Join(handles, " "+mentionSymbol)
			err = receiver.githubClient.CreateComment(owner, repo, number, &body)
			if err != nil {
				log.Println(err)
			}
		}
		fallthrough
	case "synchronize":
		// Modify labels
		labels, err := receiver.githubClient.ListLabels(owner, repo, number)
		newLabels := make([]string, len(labels))
		copy(newLabels, labels)
		if err != nil {
			return
		}
		maintainerLabels := make([]string, 0)
		if isMaintainer {
			maintainerLabels = append(maintainerLabels, "maintainer")
		}
		if isNomaintainer {
			maintainerLabels = append(maintainerLabels, "maintainer: none")
		} else if isOpenmaintainer {
			maintainerLabels = append(maintainerLabels, "maintainer: open")
		}
		for _, label := range labels {
			if !strings.HasPrefix(label, "maintainer") {
				newLabels = append(newLabels, label)
			}
		}
		newLabels = append(newLabels, maintainerLabels...)
		err = receiver.githubClient.ReplaceLabels(owner, repo, number, newLabels)
		if err != nil {
			log.Println(err)
		}
	}
	log.Println("PR #" + strconv.Itoa(number) + " processed")
}
