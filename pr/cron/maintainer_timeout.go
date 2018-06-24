package cron

import (
	"log"
	"strconv"
	"time"

	"github.com/macports/mpbot-github/pr/db"
	"github.com/macports/mpbot-github/pr/githubapi"
)

type Manager struct {
	DB     db.DBHelper
	Client githubapi.Client
}

func (manager *Manager) Start() {
	manager.MaintainerTimeout()
	for {
		select {
		case <-time.After(6 * time.Hour):
			manager.MaintainerTimeout()
		}
	}
}

func (manager *Manager) MaintainerTimeout() {
	//TODO: properly handle nil pointers
	defer func() {
		if r := recover(); r != nil {
			log.Println(r)
		}
	}()

	prs, err := manager.DB.GetTimeoutPRs()
	if err != nil {
		log.Println(err)
		return
	}
prLoop:
	for _, pr := range prs {
		log.Println("maintainer timeout of PR #" + strconv.Itoa(pr.Number) + " detected")
		prStatus, err := manager.Client.GetPullRequest("macports", "macports-ports", pr.Number)
		if err != nil {
			log.Println("Failed to get status of PR #" + strconv.Itoa(pr.Number))
			continue
		}
		if *prStatus.State == "closed" {
			log.Println("PR #" + strconv.Itoa(pr.Number) + " closed, clear pending_review")
			manager.DB.SetPRPendingReview(pr.Number, false)
			continue
		}
		//TODO: de-hardcode
		labels, err := manager.Client.ListLabels("macports", "macports-ports", pr.Number)
		if err != nil {
			continue
		}
		isApprovalRequired := false
		for _, label := range labels {
			if label == "maintainer: requires approval" {
				isApprovalRequired = true
			}
			if label == "maintainer: timeout" {
				manager.DB.SetPRPendingReview(pr.Number, false)
				continue prLoop
			}
		}
		if !isApprovalRequired {
			manager.DB.SetPRPendingReview(pr.Number, false)
		} else {
			labels = append(labels, "maintainer: timeout")
			err = manager.Client.ReplaceLabels("macports", "macports-ports", pr.Number, labels)
			if err == nil {
				manager.DB.SetPRPendingReview(pr.Number, false)
			}
		}
	}
}
