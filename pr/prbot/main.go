package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/macports/mpbot-github/pr/cron"
	"github.com/macports/mpbot-github/pr/db"
	"github.com/macports/mpbot-github/pr/githubapi"
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

	prodFlag := false
	if os.Getenv("BOT_ENV") == "production" {
		prodFlag = true
	}

	dbHelper, err := db.NewDBHelper()
	if err != nil {
		if prodFlag {
			log.Fatal(err)
		} else {
			log.Println(err)
		}
	}

	cronManager := cron.Manager{
		DB:     dbHelper,
		Client: githubapi.NewClient(botSecret),
	}
	go cronManager.Start()

	receiver := webhook.NewReceiver(*webhookAddr, hookSecret, botSecret, prodFlag, dbHelper)
	go receiver.Start()

	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	// TODO: SIGTERM cancels PR processing.
sigLoop:
	for sig := range sigChan {
		switch sig {
		case syscall.SIGINT, syscall.SIGTERM:
			receiver.Shutdown()
			break sigLoop
		}
	}
}
