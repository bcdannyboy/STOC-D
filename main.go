package main

import (
	"log"
	"os"

	stocdslack "github.com/bcdannyboy/stocd/slack"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	appToken := os.Getenv("SLACK_APP_TOKEN")
	botToken := os.Getenv("SLACK_BOT_TOKEN")

	bot := stocdslack.NewSlackBot(appToken, botToken)

	log.Println("Starting SlackBot...")
	err = bot.Start()
	if err != nil {
		log.Fatalf("Error starting SlackBot: %v", err)
	}
}
