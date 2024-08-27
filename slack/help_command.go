package stocdslock

import (
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
)

type HelpHandler struct{}

func NewHelpHandler() *HelpHandler {
	return &HelpHandler{}
}

func (h *HelpHandler) HandleCommand(evt *socketmode.Event, client *socketmode.Client) error {
	data := evt.Data.(slack.SlashCommand)
	helpText := "Available commands:\n" +
		"/help - Show this help message\n" +
		"/fcs <symbol> <indicator> <minDTE> <maxDTE> <minRoR> <RFR> - Find credit spreads"

	_, _, err := client.PostMessage(data.ChannelID,
		slack.MsgOptionText(helpText, false))
	return err
}
