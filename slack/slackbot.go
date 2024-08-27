package stocdslack

import (
	"log"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
)

type SlackBot struct {
	client       *slack.Client
	socketClient *socketmode.Client
	eventHandler *Handler
}

func NewSlackBot(appToken, botToken string) *SlackBot {
	client := slack.New(
		botToken,
		slack.OptionAppLevelToken(appToken),
	)

	socketClient := socketmode.New(
		client,
		socketmode.OptionDebug(true),
		socketmode.OptionLog(log.New(log.Writer(), "socketmode: ", log.Lshortfile|log.LstdFlags)),
	)

	return &SlackBot{
		client:       client,
		socketClient: socketClient,
		eventHandler: NewHandler(),
	}
}

func (sb *SlackBot) Start() error {
	go func() {
		for evt := range sb.socketClient.Events {
			switch evt.Type {
			case socketmode.EventTypeSlashCommand:
				sb.eventHandler.Handle(&evt, sb.socketClient)
			}
		}
	}()

	return sb.socketClient.Run()
}
