package main

import (
	"log"

	"github.com/macports/mpbot-github/ci"
	"github.com/macports/mpbot-github/ci/logger"
)

// Entry point of the CI bot
func main() {
	session, err := ci.NewSession()
	if err != nil {
		log.Fatal(err)
	}
	err = session.Run()
	// Signal the logger to exit
	logger.GlobalLogger.LogChan <- nil
	logger.GlobalLogger.Wait()
	if err != nil {
		log.Fatal(err)
	}
}
