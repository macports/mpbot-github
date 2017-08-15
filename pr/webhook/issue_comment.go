package webhook

import (
	"encoding/json"
	"log"
	"strconv"

	"github.com/google/go-github/github"
)

func (receiver *Receiver) handleIssueComment(body []byte) {
	defer func() {
		if r := recover(); r != nil {
			log.Println(r)
		}
	}()

	event := &github.IssueCommentEvent{}
	err := json.Unmarshal(body, event)
	if err != nil {
		log.Println(err)
		return
	}

	number := *event.Issue.Number
	sender := *event.Sender.Login

	// TODO: refactor, share with PullRequestReview
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
