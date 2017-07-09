package main

import (
	"flag"
	"github.com/macports/mpbot-github/pr/webhook"
	"log"
	"os"
)

// Entry point of the PR bot
func main() {
	webhookAddr := flag.String("l", "localhost:8081", "listen address for webhook events")
	flag.Parse()
	hubSecret := []byte(os.Getenv("HUB_WEBHOOK_SECRET"))
	if len(hubSecret) == 0 {
		log.Fatal("HUB_WEBHOOK_SECRET not found")
	}

	webhook.NewReceiver(*webhookAddr, hubSecret).Start()
}
