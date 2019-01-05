package webhook

import (
	"context"
	"crypto"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	retryablehttp "github.com/hashicorp/go-retryablehttp"
	"github.com/macports/mpbot-github/pr/db"
	"github.com/macports/mpbot-github/pr/githubapi"
)

type Receiver struct {
	server           *http.Server
	hookSecret       []byte
	production       bool
	testing          bool
	httpClient       *retryablehttp.Client
	githubClient     githubapi.Client
	dbHelper         db.DBHelper
	wg               sync.WaitGroup
	members          *map[string]bool
	membersLock      sync.RWMutex
	travisPubKey     *rsa.PublicKey
	travisPubKeyLock sync.RWMutex
}

func NewReceiver(listenAddr string, hookSecret []byte, botSecret string, production bool, dbHelper db.DBHelper) *Receiver {
	return &Receiver{
		server:       &http.Server{Addr: listenAddr},
		hookSecret:   hookSecret,
		production:   production,
		httpClient:   retryablehttp.NewClient(),
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

		receiver.wg.Add(1)

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			receiver.wg.Done()
			return
		}

		if !receiver.checkMAC(body, sig) {
			w.WriteHeader(http.StatusBadRequest)
			receiver.wg.Done()
			return
		}

		eventType := r.Header.Get("X-GitHub-Event")
		switch eventType {
		case "":
			w.WriteHeader(http.StatusBadRequest)
			receiver.wg.Done()
			return
		case "pull_request":
			go receiver.handlePullRequest(body)
		case "pull_request_review", "issue_comment":
			go receiver.handleOtherPullRequestEvents(eventType, body)
		default:
			w.WriteHeader(http.StatusNoContent)
			receiver.wg.Done()
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("/travis", func(w http.ResponseWriter, r *http.Request) {
		sigStr := r.Header.Get("Signature")

		sig, err := base64.StdEncoding.DecodeString(sigStr)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		receiver.wg.Add(1)

		body := []byte(r.FormValue("payload"))

		if len(body) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			receiver.wg.Done()
			return
		}

		hashed := sha1.Sum(body)
		receiver.travisPubKeyLock.RLock()
		err = rsa.VerifyPKCS1v15(receiver.travisPubKey, crypto.SHA1, hashed[:], sig)
		receiver.travisPubKeyLock.RUnlock()
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusBadRequest)
			receiver.wg.Done()
			return
		}

		var payload TravisWebhookPayload

		err = json.Unmarshal(body, &payload)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusBadRequest)
			receiver.wg.Done()
			return
		}

		go receiver.handleTravisWebhook(payload)

		w.WriteHeader(http.StatusNoContent)
	})

	go receiver.updateMembers()
	receiver.updateTravisPubKey()

	receiver.server.Handler = mux
	receiver.server.ListenAndServe()
}

func (receiver *Receiver) Shutdown() {
	receiver.server.Shutdown(context.Background())
	receiver.wg.Wait()
}

func (receiver *Receiver) updateTravisPubKey() {
	const travisPubKeyPEM = "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAvtjdLkS+FP+0fPC09j25\ny/PiuYDDivIT86COVedvlElk99BBYTrqNaJybxjXbIZ1Q6xFNhOY+iTcBr4E1zJu\ntizF3Xi0V9tOuP/M8Wn4Y/1lCWbQKlWrNQuqNBmhovF4K3mDCYswVbpgTmp+JQYu\nBm9QMdieZMNry5s6aiMA9aSjDlNyedvSENYo18F+NYg1J0C0JiPYTxheCb4optr1\n5xNzFKhAkuGs4XTOA5C7Q06GCKtDNf44s/CVE30KODUxBi0MCKaxiXw/yy55zxX2\n/YdGphIyQiA5iO1986ZmZCLLW8udz9uhW5jUr3Jlp9LbmphAC61bVSf4ou2YsJaN\n0QIDAQAB\n-----END PUBLIC KEY-----"

	p, _ := pem.Decode([]byte(travisPubKeyPEM))
	if p == nil || p.Type != "PUBLIC KEY" {
		log.Println("travis: invalid public key")
		return
	}

	travisPubKey, err := x509.ParsePKIXPublicKey(p.Bytes)
	if err != nil {
		return
	}

	if pubKey, ok := travisPubKey.(*rsa.PublicKey); ok {
		receiver.travisPubKeyLock.Lock()
		receiver.travisPubKey = pubKey
		receiver.travisPubKeyLock.Unlock()
	}
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
			log.Println("Updating list of members, got", len(members), "members")
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
