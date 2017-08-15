package webhook

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/macports/mpbot-github/pr/db"
	"github.com/macports/mpbot-github/pr/githubapi"
)

type Receiver struct {
	listenAddr   string
	hookSecret   []byte
	production   bool
	testing      bool
	githubClient githubapi.Client
	dbHelper     db.DBHelper
	members      *map[string]bool
	membersLock  sync.RWMutex
}

func NewReceiver(listenAddr string, hookSecret []byte, botSecret string, production bool, dbHelper db.DBHelper) *Receiver {
	return &Receiver{
		listenAddr:   listenAddr,
		hookSecret:   hookSecret,
		production:   production,
		githubClient: githubapi.NewClient(botSecret),
		dbHelper:     dbHelper,
	}
}

func (receiver *Receiver) Start() {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		sigStr := r.Header.Get("X-Hub-Signature")
		if len(sigStr) != 45 || !strings.HasPrefix(sigStr, "sha1=") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		sig, err := hex.DecodeString(sigStr[5:])
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if !receiver.checkMAC(body, sig) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		switch r.Header.Get("X-GitHub-Event") {
		case "":
			w.WriteHeader(http.StatusBadRequest)
			return
		case "pull_request":
			go receiver.handlePullRequest(body)
		case "pull_request_review":
			go receiver.handlePullRequestReview(body)
		case "issue_comment":
			go receiver.handleIssueComment(body)
		}

		w.WriteHeader(http.StatusNoContent)
	})

	go receiver.updateMembers()

	http.ListenAndServe(receiver.listenAddr, mux)
}

func (receiver *Receiver) updateMembers() {
	for ; ; time.Sleep(24 * time.Hour) {
		users, err := receiver.githubClient.ListOrgMembers("macports")
		if err != nil {
			continue
		}
		members := make(map[string]bool)
		for _, user := range users {
			if user.Login == nil {
				continue
			}
			login := *user.Login
			if login == "" {
				continue
			}
			members[login] = true
		}
		if len(members) > 0 {
			receiver.membersLock.Lock()
			receiver.members = &members
			receiver.membersLock.Unlock()
		}
	}
}

// checkMAC reports whether messageMAC is a valid HMAC tag for message.
func (receiver *Receiver) checkMAC(message, messageMAC []byte) bool {
	mac := hmac.New(sha1.New, receiver.hookSecret)
	mac.Write(message)
	expectedMAC := mac.Sum(nil)
	return hmac.Equal(messageMAC, expectedMAC)
}
