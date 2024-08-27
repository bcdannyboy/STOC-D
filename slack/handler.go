package stocdslock

import (
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
)

type Handler struct {
	helpHandler *HelpHandler
	fcsHandler  *FCSHandler
}

func NewHandler() *Handler {
	return &Handler{
		helpHandler: NewHelpHandler(),
		fcsHandler:  NewFCSHandler(),
	}
}

func (h *Handler) Handle(evt *socketmode.Event, client *socketmode.Client) error {
	data := evt.Data.(slack.SlashCommand)
	switch data.Command {
	case "/help":
		err := h.helpHandler.HandleCommand(evt, client)
		if err != nil {
			return err
		}
	case "/fcs":
		err := h.fcsHandler.HandleCommand(evt, client)
		if err != nil {
			return err
		}
	}

	client.Ack(*evt.Request)
	return nil
}
