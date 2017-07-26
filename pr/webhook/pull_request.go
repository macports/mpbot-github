package webhook

import (
	"encoding/json"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/go-github/github"
	"github.com/macports/mpbot-github/pr/db"
)

var cveRegexp = regexp.MustCompile(`CVE-\d{4}-\d+`)

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

	ports, files, err := receiver.githubClient.ListChangedPortsAndFiles(owner, repo, number)
	if err != nil {
		log.Println(err)
		return
	}

	handles := make(map[string][]string)
	// If unrecognized port was added
	isSubmission := false
	// If all ports changed are openmaintainer or nomaintainer
	isOpenmaintainer := true
	// If all ports changed have no maintainers
	isNomaintainer := true
	// If PR sender is maintainer of all ports changed (exclude minor changes and nomaintainer)
	isMaintainer := true
	// If PR sender is maintainer of one of the ports changed
	isOneMaintainer := false
	for i, port := range ports {
		portMaintainer, err := db.GetPortMaintainer(port)
		if err != nil {
			// TODO: handle submission of duplicate ports
			if err.Error() == "port not found" && *files[i].Status == "added" {
				isSubmission = true
				// Could be adding a -devel port, so keep the "maintainer" label
				isNomaintainer = false
				isOpenmaintainer = false
				continue
			}
			log.Println("Error getting maintainer for port " + port + ": " + err.Error())
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
				if maintainer.GithubHandle == *event.Sender.Login {
					isPortMaintainer = true
					isOneMaintainer = true
				} else if len(ports) < 2 || *files[i].Changes > 2 {
					handles[maintainer.GithubHandle] = append(handles[maintainer.GithubHandle], port)
				}
			}
		}
		// Exclude minor changes like increase revision of dependents
		if *files[i].Changes > 2 && !isPortMaintainer {
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
		if len(handles) > 0 && !strings.Contains(*event.PullRequest.Body, "[skip notification]") {
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

		// Modify labels
		labels, err := receiver.githubClient.ListLabels(owner, repo, number)
		if err != nil {
			log.Println(err)
			return
		}
		newLabels := make([]string, 0, len(labels))
		maintainerLabels := make([]string, 0)
		typeLabels := make([]string, 0)

		if isMaintainer {
			maintainerLabels = append(maintainerLabels, "maintainer")
		}
		if isNomaintainer {
			maintainerLabels = append(maintainerLabels, "maintainer: none")
		} else if isOpenmaintainer {
			maintainerLabels = append(maintainerLabels, "maintainer: open")
		} else if !isSubmission && !isMaintainer {
			// TODO: store in DB
			maintainerLabels = append(maintainerLabels, "maintainer: requires approval")
		}

		// Collect existing labels (PR sender could add labels when creating a PR)
		for _, label := range labels {
			if !strings.HasPrefix(label, "maintainer") {
				if strings.HasPrefix(label, "type: ") {
					typeLabels = append(typeLabels, label)
				} else {
					newLabels = append(newLabels, label)
				}
			}
		}

		// Determine type labels
		// TODO: read PR body to determine type
		if isSubmission {
			typeLabels = appendIfUnique(typeLabels, "type: submission")
		}
		if strings.Contains(strings.ToLower(*event.PullRequest.Title), ": update to") {
			typeLabels = appendIfUnique(typeLabels, "type: update")
		}
		if cveRegexp.FindString(*event.PullRequest.Body) != "" {
			typeLabels = appendIfUnique(typeLabels, "type: security fix")
		}
		typesFromBody := []string{"bugfix", "enhancement", "security fix"}
		for _, t := range typesFromBody {
			if strings.Contains(*event.PullRequest.Body, "[x] "+t) {
				typeLabels = appendIfUnique(typeLabels, "type: "+t)
			}
		}

		if len(ports) > 0 {
			newLabels = append(newLabels, maintainerLabels...)
		}
		newLabels = append(newLabels, typeLabels...)

		err = receiver.githubClient.ReplaceLabels(owner, repo, number, newLabels)
		if err != nil {
			log.Println(err)
		}
		//	fallthrough
		//case "synchronize":
	}
	log.Println("PR #" + strconv.Itoa(number) + " processed")
}

// TODO: use map to dedup
func appendIfUnique(slice []string, elem string) []string {
	for _, e := range slice {
		if e == elem {
			return slice
		}
	}
	return append(slice, elem)
}
