package stocdslack

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bcdannyboy/stocd/models"
	"github.com/bcdannyboy/stocd/positions"
	"github.com/bcdannyboy/stocd/tradier"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
)

const (
	weightLiquidity   = 0.5
	weightProbability = 0.3
	weightVaR         = 0.1
	weightES          = 0.1
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
		slack.MsgOptionText(fmt.Sprintf("Starting credit spread analysis for: %s %f %d %d %f %f", symbol, indicator, int(minDTE), int(maxDTE), minRoR, rfr), false))
	if err != nil {
		return err
	}

	// Run STOCD with progress updates
	go runSTOCDWithProgress(client, data.ChannelID, ts, indicators, minDTE, maxDTE, rfr, minRoR)

	return nil
}

func runSTOCDWithProgress(client *socketmode.Client, channelID, timestamp string, indicators map[string]float64, minDTE, maxDTE, rfr, minRoR float64) {
	tradierKey := os.Getenv("TRADIER_KEY")
	symbol := getFirstKey(indicators)
	indicator := indicators[symbol]

	quotes, err := tradier.GET_QUOTES(symbol, time.Now().AddDate(-10, 0, 0).Format("2006-01-02"), time.Now().Format("2006-01-02"), "daily", tradierKey)
	if err != nil {
		client.PostMessage(channelID, slack.MsgOptionText(fmt.Sprintf("Error fetching quotes: %v", err), false), slack.MsgOptionTS(timestamp))
		return
	}

	optionsChain, err := tradier.GET_OPTIONS_CHAIN(symbol, tradierKey, int(minDTE), int(maxDTE))
	if err != nil {
		client.PostMessage(channelID, slack.MsgOptionText(fmt.Sprintf("Error fetching options chain: %v", err), false), slack.MsgOptionTS(timestamp))
		return
	}

	lastPrice := quotes.History.Day[len(quotes.History.Day)-1].Close

	progressChan := make(chan int)
	resultChan := make(chan []models.SpreadWithProbabilities)

	go func() {
		var spreads []models.SpreadWithProbabilities
		if indicator > 0 {
			client.PostMessage(channelID, slack.MsgOptionText("Identifying Bull Put Spreads...", false), slack.MsgOptionTS(timestamp))
			spreads = positions.IdentifyBullPutSpreads(optionsChain, lastPrice, rfr, *quotes, minRoR, time.Now(), progressChan)
		} else {
			client.PostMessage(channelID, slack.MsgOptionText("Identifying Bear Call Spreads...", false), slack.MsgOptionTS(timestamp))
			spreads = positions.IdentifyBearCallSpreads(optionsChain, lastPrice, rfr, *quotes, minRoR, time.Now(), progressChan)
		}
		resultChan <- spreads
	}()

	for {
		select {
		case progress := <-progressChan:
			if progress%10 == 0 { // Update every 10%
				client.PostMessage(channelID,
					slack.MsgOptionText(fmt.Sprintf("Analysis %d%% complete...", progress), false),
					slack.MsgOptionTS(timestamp))
			}
		case spreads := <-resultChan:
			// Calculate composite scores
			calculateCompositeScores(spreads)

			// Sort spreads by composite score
			sort.Slice(spreads, func(i, j int) bool {
				return spreads[i].CompositeScore > spreads[j].CompositeScore
			})

			// Prepare the result message
			var resultMsg strings.Builder
			resultMsg.WriteString(fmt.Sprintf("Analysis complete. Found %d spreads meeting criteria.\n\n", len(spreads)))

			for i, spread := range spreads[:min(10, len(spreads))] {
				resultMsg.WriteString(fmt.Sprintf("Spread %d:\n", i+1))
				resultMsg.WriteString(fmt.Sprintf("  Composite Score: %.2f\n", spread.CompositeScore))
				resultMsg.WriteString(fmt.Sprintf("  Short Leg: %s, Long Leg: %s\n", spread.Spread.ShortLeg.Option.Symbol, spread.Spread.LongLeg.Option.Symbol))
				resultMsg.WriteString(fmt.Sprintf("  Probability of Profit: %.2f%%\n", spread.Probability.AverageProbability*100))
				resultMsg.WriteString(fmt.Sprintf("  Spread Credit: %.2f, ROR: %.2f%%\n", spread.Spread.SpreadCredit, spread.Spread.ROR*100))
				resultMsg.WriteString(fmt.Sprintf("  Expected Shortfall: %.2f%%\n", spread.ExpectedShortfall*100))
				resultMsg.WriteString(fmt.Sprintf("  VaR (95%%): %.2f%%\n", spread.VaR95*100))
				resultMsg.WriteString(fmt.Sprintf("  Liquidity: %.2f\n", spread.Liquidity))
				resultMsg.WriteString(fmt.Sprintf("  Volume: %d\n\n", spread.Spread.ShortLeg.Option.Volume+spread.Spread.LongLeg.Option.Volume))
			}

			// Send the final result
			client.PostMessage(channelID, slack.MsgOptionText(resultMsg.String(), false), slack.MsgOptionTS(timestamp))
			return
		}
	}
}

func calculateCompositeScores(spreads []models.SpreadWithProbabilities) {
	var minProb, maxProb, minVaR, maxVaR, minES, maxES, minLiquidity, maxLiquidity float64
	maxLiquidity = math.Inf(-1) // Initialize to negative infinity
	minLiquidity = math.Inf(1)  // Initialize to positive infinity

	// Find min and max values
	for _, spread := range spreads {
		prob := spread.Probability.AverageProbability
		var95 := math.Abs(spread.VaR95)
		es := math.Abs(spread.ExpectedShortfall)
		liquidity := spread.Liquidity

		minProb = math.Min(minProb, prob)
		maxProb = math.Max(maxProb, prob)
		minVaR = math.Min(minVaR, var95)
		maxVaR = math.Max(maxVaR, var95)
		minES = math.Min(minES, es)
		maxES = math.Max(maxES, es)
		minLiquidity = math.Min(minLiquidity, liquidity)
		maxLiquidity = math.Max(maxLiquidity, liquidity)
	}

	normalizeValue := func(value, min, max float64) float64 {
		if min == max {
			return 0.5 // Return middle value if min and max are the same
		}
		return (value - min) / (max - min)
	}

	// Calculate composite scores
	for i := range spreads {
		prob := spreads[i].Probability.AverageProbability
		var95 := math.Abs(spreads[i].VaR95)
		es := math.Abs(spreads[i].ExpectedShortfall)
		liquidity := spreads[i].Liquidity
		vol := float64(spreads[i].Spread.ShortLeg.Option.Volume + spreads[i].Spread.LongLeg.Option.Volume)

		// Normalize values
		normProb := normalizeValue(prob, minProb, maxProb)
		normVaR := 1 - normalizeValue(var95, minVaR, maxVaR)                       // Invert so lower is better
		normES := 1 - normalizeValue(es, minES, maxES)                             // Invert so lower is better
		normLiquidity := 1 - normalizeValue(liquidity, minLiquidity, maxLiquidity) // Invert so lower is better

		// Calculate weighted score
		weightedScore := (normLiquidity * weightLiquidity) +
			(normProb * weightProbability) +
			(normVaR * weightVaR) +
			(normES * weightES)

		spreads[i].CompositeScore = weightedScore * (1 + math.Log1p(vol)) // Use log to dampen the effect of volume
	}
}

func getFirstKey(m map[string]float64) string {
	for k := range m {
		return k
	}
	return ""
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
