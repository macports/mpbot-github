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
		log.Println(err)
		return
	}
	number := *event.Number
	owner := *event.Repo.Owner.Login
	repo := *event.Repo.Name

	ports, changes, err := receiver.githubClient.ListChangedPortsAndLines(number)
	if err != nil {
		log.Println(err)
		return
	}

	handles := make(map[string][]string)
	isOpenmaintainer := true
	isNomaintainer := true
	isMaintainer := true
	isOneMaintainer := false
	for i, port := range ports {
		portMaintainer, err := db.GetPortMaintainer(port)
		if err != nil {
			continue
		}
		isNomaintainer = isNomaintainer && portMaintainer.NoMaintainer
		isOpenmaintainer = isOpenmaintainer && (portMaintainer.OpenMaintainer || portMaintainer.NoMaintainer)
		if portMaintainer.NoMaintainer {
			continue
		}
		allMaintainers := append(portMaintainer.Others, portMaintainer.Primary)
		isPortMaintainer := false
		for _, maintainer := range allMaintainers {
			if maintainer.GithubHandle != "" {
				handles[maintainer.GithubHandle] = append(handles[maintainer.GithubHandle], port)
				if maintainer.GithubHandle == *event.Sender.Login {
					isPortMaintainer = true
					isOneMaintainer = true
				}
			}
		}
		if changes[i] > 7 && !isPortMaintainer {
			isMaintainer = false
		}
	}
	isMaintainer = isOneMaintainer && isMaintainer

	switch *event.Action {
	case "opened":
		// Notify maintainers
		mentionSymbol := "@_"
		if receiver.production {
			mentionSymbol = "@"
		}
		if len(handles) > 0 {
			body := "Notifying maintainers:\n"
			for handle, ports := range handles {
				body += mentionSymbol + handle + " for port " + strings.Join(ports, ", ") + ".\n"
			}
			body += "\nBy a harmless bot."
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
			log.Println(err)
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
