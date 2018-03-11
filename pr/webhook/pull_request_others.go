package webhook

import (
	"encoding/json"
	"log"
	"strconv"

	"github.com/google/go-github/github"
)

func (receiver *Receiver) handleOtherPullRequestEvents(eventType string, body []byte) {
	defer func() {
		if r := recover(); r != nil {
			log.Println(r)
		}

		if !receiver.testing {
			receiver.wg.Done()
		}
	}()

	var number int
	var sender string

	switch eventType {
	case "pull_request_review":
		event := &github.PullRequestReviewEvent{}
		err := json.Unmarshal(body, event)
		if err != nil {
			log.Println(err)
			return
		}

		number = *event.PullRequest.Number
		sender = *event.Sender.Login
	case "issue_comment":
		event := &github.IssueCommentEvent{}
		err := json.Unmarshal(body, event)
		if err != nil {
			log.Println(err)
			return
		}

		number = *event.Issue.Number
		sender = *event.Sender.Login
	default:
		return
	}

	pr, err := receiver.dbHelper.GetPR(number)
	if err != nil {
		log.Println(err)
		return
	}
	if !pr.Processed {
		return
	}
	if !pr.PendingReview {
		return
	}
	isOneMaintainer := false
	for _, maintainer := range pr.Maintainers {
		if maintainer == sender {
			isOneMaintainer = true
		}
	}
	if isOneMaintainer {
		log.Println("Maintainer responded in PR #" + strconv.Itoa(pr.Number))
		receiver.dbHelper.SetPRPendingReview(number, false)
	}
}
