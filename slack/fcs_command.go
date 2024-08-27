package stocdslock

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
)

type FCSHandler struct{}

func NewFCSHandler() *FCSHandler {
	return &FCSHandler{}
}

func (h *FCSHandler) HandleCommand(evt *socketmode.Event, client *socketmode.Client) error {
	data := evt.Data.(slack.SlashCommand)
	args := strings.Fields(data.Text)

	if len(args) != 6 {
		_, _, err := client.PostMessage(data.ChannelID,
			slack.MsgOptionText("Invalid number of arguments. Usage: /fcs <symbol> <indicator> <minDTE> <maxDTE> <minRoR> <RFR>", false))
		return err
	}

	symbol := args[0]
	indicator, _ := strconv.ParseFloat(args[1], 64)
	minDTE, _ := strconv.ParseFloat(args[2], 64)
	maxDTE, _ := strconv.ParseFloat(args[3], 64)
	minRoR, _ := strconv.ParseFloat(args[4], 64)
	rfr, _ := strconv.ParseFloat(args[5], 64)

	indicators := map[string]float64{symbol: indicator}

	// Send initial message
	_, ts, err := client.PostMessage(data.ChannelID,
		slack.MsgOptionText("Starting credit spread analysis...", false))
	if err != nil {
		return err
	}

	// Run STOCD with progress updates
	go runSTOCDWithProgress(client, data.ChannelID, ts, indicators, minDTE, maxDTE, rfr, minRoR)

	return nil
}

func runSTOCDWithProgress(client *socketmode.Client, channelID, timestamp string, indicators map[string]float64, minDTE, maxDTE, rfr, minRoR float64) {
	progressChan := make(chan int)
	resultChan := make(chan string)

	go func() {
		result := STOCD(indicators, minDTE, maxDTE, rfr, minRoR, progressChan)
		resultChan <- result
	}()

	for {
		select {
		case progress := <-progressChan:
			if progress == 25 || progress == 50 || progress == 75 {
				client.PostMessage(channelID,
					slack.MsgOptionText(fmt.Sprintf("Analysis %d%% complete...", progress), false),
					slack.MsgOptionTS(timestamp))
			}
		case result := <-resultChan:
			client.PostMessage(channelID,
				slack.MsgOptionText(result, false),
				slack.MsgOptionTS(timestamp))
			return
		}
	}
}

// STOCD function should be implemented in the main package and imported here
// This is just a placeholder to show how it would be called
func STOCD(indicators map[string]float64, minDTE, maxDTE, rfr, minRoR float64, progressChan chan<- int) string {
	// Implementation goes here
	return "STOCD result"
}
