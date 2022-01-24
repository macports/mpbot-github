package webhook

import (
	"encoding/json"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/go-github/v28/github"
)

var cveRegexp = regexp.MustCompile(`CVE-\d{4}-\d+`)

func (receiver *Receiver) handlePullRequest(body []byte) {
	defer func() {
		if r := recover(); r != nil {
			log.Println(r)
		}

		if !receiver.testing {
			receiver.wg.Done()
		}
	}()

	event := &github.PullRequestEvent{}
	err := json.Unmarshal(body, event)
	if err != nil {
		log.Println(err)
		return
	}

	receiver.processPullRequest(event)
}

func (receiver *Receiver) processPullRequest(event *github.PullRequestEvent) {
	number := *event.Number
	owner := *event.Repo.Owner.Login
	repo := *event.Repo.Name

	log.Println("PR #" + strconv.Itoa(number) + " " + *event.Action)

	ports, files, err := receiver.githubClient.ListChangedPortsAndFiles(owner, repo, number)
	if err != nil {
		log.Println(err)
		return
	}

	handles := make(map[string][]string)
	// If unrecognized port was added
	isSubmission := false
	isAllSubmission := true
	// If all ports changed are openmaintainer or nomaintainer
	isOpenmaintainer := true
	// If all ports changed have no maintainers
	isNomaintainer := true
	// If PR sender is maintainer of all ports changed (exclude minor changes and nomaintainer)
	isMaintainer := true
	// If PR sender is maintainer of one of the ports changed
	isOneMaintainer := false
	for i, port := range ports {
		portMaintainer, err := receiver.dbHelper.GetPortMaintainer(port)
		if err != nil {
			// TODO: warn about submission of duplicate ports in different category
			if err.Error() == "port not found" && !strings.Contains(*files[i].Filename, "/files/") && *files[i].Status == "added" {
				isSubmission = true
				continue
			}
			log.Println("Error getting maintainer for port " + port + ": " + err.Error())
			continue
		}
		isAllSubmission = false
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
				} else {
					handles[maintainer.GithubHandle] = append(handles[maintainer.GithubHandle], port)
				}
			}
		}
		// No maintainer label if not maintainer of one of the ports
		// exclude minor changes like increase revision of dependents
		if *files[i].Changes > 2 && !isPortMaintainer {
			isMaintainer = false
		}
	}
	isMaintainer = isOneMaintainer && isMaintainer
	if isAllSubmission {
		isNomaintainer = false
		isOpenmaintainer = false
	}

	maintainers := make([]string, len(handles))
	{
		i := 0
		for k := range handles {
			maintainers[i] = k
			i++
		}
	}

	switch *event.Action {
	case "opened":
		receiver.dbHelper.NewPR(number, maintainers)
		// Notify maintainers
		mentionSymbol := "@_"
		if receiver.production {
			mentionSymbol = "@"
		}
		if len(handles) > 0 && !strings.Contains(*event.PullRequest.Body, "[skip notification]") {
			// GitHub will only notify the first 50 handles in a single comment
			const mentionLimit = 50

			// i counts from 0 to len(handles)
			// Use (i % mentionLimit) to limit each comment to containing
			// at most (mentionLimit) handles
			i := 0

			totalBatches := (len(handles) + mentionLimit - 1) / mentionLimit

			var body string

			for handle, ports := range handles {
				// Increment counter for each handle
				i++

				// Initialize body if first handle for this batch
				if (i % mentionLimit) == 1 {
					currentBatch := (i + mentionLimit - 1) / mentionLimit
					body = "Notifying maintainers (batch " + strconv.Itoa(currentBatch) +
						" of " + strconv.Itoa(totalBatches) + "):\n"
				}

				body += mentionSymbol + handle + " for port " + strings.Join(ports, ", ") + ".\n"
				err = receiver.githubClient.AddAssignees(owner, repo, number, []string{handle})
				if err != nil {
					log.Println(err)
				}

				// Create comment if last handle for this batch
				// (either once body contains (mentionLimit) handles,
				// or when there are no handles left)
				if ((i % mentionLimit) == 0) || (i == len(handles)) {
					err = receiver.githubClient.CreateComment(owner, repo, number, &body)
					if err != nil {
						log.Println(err)
					}
				}
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
		} else if !isAllSubmission && !isMaintainer {
			// TODO: store in DB
			maintainerLabels = append(maintainerLabels, "maintainer: requires approval")
		}

		if !isNomaintainer && !isAllSubmission && !isMaintainer {
			receiver.dbHelper.SetPRPendingReview(number, true)
		}

		// Collect existing labels (PR sender could add labels when creating a PR)
		for _, label := range labels {
			if strings.HasPrefix(label, "maintainer") {
				continue
			}
			if strings.HasPrefix(label, "type: ") {
				typeLabels = append(typeLabels, label)
			} else {
				newLabels = append(newLabels, label)
			}
		}

		// Determine type labels
		// TODO: read PR body to determine type
		if isSubmission {
			typeLabels = appendIfUnique(typeLabels, "type: submission")
		}
		if strings.Contains(strings.ToLower(*event.PullRequest.Title), ": update") || strings.HasPrefix(strings.ToLower(*event.PullRequest.Title), "update") {
			typeLabels = appendIfUnique(typeLabels, "type: update")
		}
		if cveRegexp.FindString(*event.PullRequest.Title) != "" || cveRegexp.FindString(*event.PullRequest.Body) != "" {
			typeLabels = appendIfUnique(typeLabels, "type: security fix")
		}
		typesFromBody := []string{"bugfix", "enhancement", "security fix", "update"}
		for _, t := range typesFromBody {
			if strings.Contains(*event.PullRequest.Body, "[x] "+t) {
				typeLabels = appendIfUnique(typeLabels, "type: "+t)
			}
		}

		if len(ports) > 0 {
			newLabels = append(newLabels, maintainerLabels...)
		}
		newLabels = append(newLabels, typeLabels...)

		receiver.membersLock.RLock()
		members := receiver.members
		receiver.membersLock.RUnlock()
		if members != nil {
			_, exist := (*members)[*event.Sender.Login]
			if exist {
				newLabels = appendIfUnique(newLabels, "by: member")
			}
		}

		err = receiver.githubClient.ReplaceLabels(owner, repo, number, newLabels)
		if err != nil {
			log.Println(err)
		}

		receiver.dbHelper.SetPRProcessed(number, true)
		//	fallthrough
		//case "synchronize":
	}
	if !receiver.testing {
		log.Println("PR #" + strconv.Itoa(number) + " processed")
	}
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
