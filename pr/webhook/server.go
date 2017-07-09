package webhook

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"io/ioutil"
	"net/http"
	"strings"
)

type Receiver struct {
	listenAddr string
	hubSecret  []byte
}

func NewReceiver(listenAddr string, hubSecret []byte) *Receiver {
	return &Receiver{
		listenAddr: listenAddr,
		hubSecret:  hubSecret,
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

		w.WriteHeader(http.StatusNoContent)
	})

	http.ListenAndServe(receiver.listenAddr, mux)
}

// checkMAC reports whether messageMAC is a valid HMAC tag for message.
func (receiver *Receiver) checkMAC(message, messageMAC []byte) bool {
	mac := hmac.New(sha1.New, receiver.hubSecret)
	mac.Write(message)
	expectedMAC := mac.Sum(nil)
	return hmac.Equal(messageMAC, expectedMAC)
}
