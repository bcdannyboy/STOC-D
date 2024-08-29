package stocdslack

import (
	"fmt"
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

	bot := &SlackBot{
		client:       client,
		socketClient: socketClient,
		eventHandler: NewHandler(),
	}

	// Send startup message to all channels
	go bot.notifyAllChannels()

	return bot
}

func (sb *SlackBot) notifyAllChannels() {
	// Fetch all channels using the Conversations API
	params := &slack.GetConversationsParameters{
		ExcludeArchived: true,
		Limit:           1000,
	}

	fmt.Println("Notifying all channels about STOCD bot starting...")

	for {
		channels, nextCursor, err := sb.client.GetConversations(params)
		if err != nil {
			log.Printf("Error fetching channels: %v", err)
			return
		}

		for _, channel := range channels {
			fmt.Printf("Notifying channel %s\n", channel.Name)
			_, _, err := sb.client.PostMessage(channel.ID, slack.MsgOptionText("STOCD bot has started.", false))
			if err != nil {
				log.Printf("Error sending start message to channel %s: %v", channel.Name, err)
			}
		}

		if nextCursor == "" {
			break
		}
		params.Cursor = nextCursor
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
