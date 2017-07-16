package main

import (
	"flag"
	"log"
	"os"

	"github.com/macports/mpbot-github/pr/webhook"
)

// Entry point of the PR bot
func main() {
	webhookAddr := flag.String("l", "localhost:8081", "listen address for webhook events")
	flag.Parse()
	hookSecret := []byte(os.Getenv("HUB_WEBHOOK_SECRET"))
	if len(hookSecret) == 0 {
		log.Fatal("HUB_WEBHOOK_SECRET not found")
	}
	botSecret := os.Getenv("HUB_BOT_SECRET")
	if botSecret == "" {
		log.Fatal("HUB_BOT_SECRET not found")
	}

	webhook.NewReceiver(*webhookAddr, hookSecret, botSecret).Start()
}
