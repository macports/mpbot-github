package webhook

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/macports/mpbot-github/pr/githubapi"
)

type Receiver struct {
	listenAddr   string
	hookSecret   []byte
	githubClient *githubapi.Client
}

func NewReceiver(listenAddr string, hookSecret []byte, botSecret string) *Receiver {
	return &Receiver{
		listenAddr: listenAddr,
		hookSecret: hookSecret,
		// TODO: canonical owner
		githubClient: githubapi.NewClient(botSecret),
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
		}

		w.WriteHeader(http.StatusNoContent)
	})

	http.ListenAndServe(receiver.listenAddr, mux)
}

// checkMAC reports whether messageMAC is a valid HMAC tag for message.
func (receiver *Receiver) checkMAC(message, messageMAC []byte) bool {
	mac := hmac.New(sha1.New, receiver.hookSecret)
	mac.Write(message)
	expectedMAC := mac.Sum(nil)
	return hmac.Equal(messageMAC, expectedMAC)
}
