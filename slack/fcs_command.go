package stocdslack

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bcdannyboy/stocd/models"
	"github.com/bcdannyboy/stocd/positions"
	"github.com/bcdannyboy/stocd/probability"
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

var calibrationCache sync.Map // Cache to store calibrated models for each symbol

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

	client.PostMessage(channelID, slack.MsgOptionText("Fetching quotes...", false), slack.MsgOptionTS(timestamp))
	quotes, err := tradier.GET_QUOTES(symbol, time.Now().AddDate(-10, 0, 0).Format("2006-01-02"), time.Now().Format("2006-01-02"), "daily", tradierKey)
	if err != nil {
		client.PostMessage(channelID, slack.MsgOptionText(fmt.Sprintf("Error fetching quotes: %v", err), false), slack.MsgOptionTS(timestamp))
		return
	}

	client.PostMessage(channelID, slack.MsgOptionText("Fetching options chain...", false), slack.MsgOptionTS(timestamp))
	optionsChain, err := tradier.GET_OPTIONS_CHAIN(symbol, tradierKey, int(minDTE), int(maxDTE))
	if err != nil {
		client.PostMessage(channelID, slack.MsgOptionText(fmt.Sprintf("Error fetching options chain: %v", err), false), slack.MsgOptionTS(timestamp))
		return
	}

	lastPrice := quotes.History.Day[len(quotes.History.Day)-1].Close

	// Calibrate models for the current stock before any analysis is done
	client.PostMessage(channelID, slack.MsgOptionText("Calibrating models...", false), slack.MsgOptionTS(timestamp))
	calibrationChan := make(chan string, 100000)

	// Check if the symbol is already calibrated
	globalModelsInterface, exists := calibrationCache.Load(symbol)
	var globalModels probability.GlobalModels

	if exists {
		client.PostMessage(channelID, slack.MsgOptionText("Using cached calibration for symbol "+symbol, false), slack.MsgOptionTS(timestamp))
		globalModels = globalModelsInterface.(probability.GlobalModels)
	} else {
		globalModels = calibrateGlobalModels(quotes, optionsChain, lastPrice, rfr, client, channelID, timestamp, calibrationChan)
		calibrationCache.Store(symbol, globalModels) // Store the calibrated models in the cache
	}

	go func() {
		// Handle calibration messages
		for msg := range calibrationChan {
			client.PostMessage(channelID, slack.MsgOptionText(msg, false), slack.MsgOptionTS(timestamp))
		}
		close(calibrationChan) // Ensure the channel is closed after calibration messages are processed
	}()

	client.PostMessage(channelID, slack.MsgOptionText("Running analysis...", false), slack.MsgOptionTS(timestamp))
	progressChan := make(chan int)
	resultChan := make(chan []models.SpreadWithProbabilities)

	go func() {
		var spreads []models.SpreadWithProbabilities
		if indicator > 0 {
			client.PostMessage(channelID, slack.MsgOptionText("Identifying Bull Put Spreads...", false), slack.MsgOptionTS(timestamp))
			spreads = positions.IdentifyBullPutSpreads(optionsChain, lastPrice, rfr, *quotes, minRoR, time.Now(), progressChan, &client.Client, channelID, calibrationChan, globalModels)
		} else {
			client.PostMessage(channelID, slack.MsgOptionText("Identifying Bear Call Spreads...", false), slack.MsgOptionTS(timestamp))
			spreads = positions.IdentifyBearCallSpreads(optionsChain, lastPrice, rfr, *quotes, minRoR, time.Now(), progressChan, &client.Client, channelID, calibrationChan, globalModels)
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
				resultMsg.WriteString(fmt.Sprintf("  Short Leg: %s, Long Leg: %s\n", spread.Spread.ShortLeg.Option.Symbol, spread.Spread.LongLeg.Option.Symbol))
				resultMsg.WriteString(fmt.Sprintf("  Spread Credit: %.2f, ROR: %.2f%%\n", spread.Spread.SpreadCredit, spread.Spread.ROR*100))
				resultMsg.WriteString(fmt.Sprintf("  Spread BSM Price: %.2f\n", spread.Spread.SpreadBSMPrice))
				resultMsg.WriteString(fmt.Sprintf("  Average Spread Price: %.2f\n", (spread.Spread.ShortLeg.BSMResult.Price+spread.Spread.LongLeg.BSMResult.Price)/2))
				resultMsg.WriteString(fmt.Sprintf("  Probability of Profit: %.2f%%\n", spread.Probability.AverageProbability*100))
				resultMsg.WriteString(fmt.Sprintf("  Composite Score: %.2f\n", spread.CompositeScore))
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

func calibrateGlobalModels(quotes *tradier.QuoteHistory, chain map[string]*tradier.OptionChain, underlyingPrice, riskFreeRate float64, client *socketmode.Client, channelID, timestamp string, calibrationChan chan<- string) probability.GlobalModels {
	var globalModels probability.GlobalModels

	sendCalibrationMessage := func(message string) {
		calibrationChan <- message
		fmt.Println("Calibration message:", message) // Print to console for debugging
		_, _, err := client.PostMessage(channelID, slack.MsgOptionText(message, false), slack.MsgOptionTS(timestamp))
		if err != nil {
			fmt.Printf("Error sending calibration message: %v\n", err)
		}
	}

	sendCalibrationMessage("Starting model calibration...")
	sendCalibrationMessage(fmt.Sprintf("Risk-Free Rate: %.4f", riskFreeRate))

	marketPrices := extractHistoricalPrices(*quotes)
	strikes := extractAllStrikes(chain)
	s0 := marketPrices[len(marketPrices)-1]
	t := 1.0 // Use 1 year as a default time to maturity

	// Calculate average volatilities
	yangZhangVols := models.CalculateYangZhangVolatility(*quotes)
	rogersSatchellVols := models.CalculateRogersSatchellVolatility(*quotes)
	avgYZ := calculateAverageVolatility(yangZhangVols)
	avgRS := calculateAverageVolatility(rogersSatchellVols)
	avgIV := calculateAverageImpliedVolatility(chain)
	avgVol := (avgYZ + avgRS + avgIV) / 3

	volatilityMsg := fmt.Sprintf("Average Volatilities:\nYang-Zhang: %.4f\nRogers-Satchell: %.4f\nImplied: %.4f\nOverall: %.4f", avgYZ, avgRS, avgIV, avgVol)
	sendCalibrationMessage(volatilityMsg)

	// Calibrate Merton model
	sendCalibrationMessage("Calibrating Merton model...")
	historicalJumps := calculateHistoricalJumps(*quotes)
	mertonModel := models.NewMertonJumpDiffusion(riskFreeRate, avgVol, 1.0, 0, avgVol)
	mertonModel.CalibrateJumpSizes(historicalJumps, 1)
	globalModels.Merton = mertonModel
	sendCalibrationMessage("Merton model calibrated.")

	// Calibrate Kou model
	sendCalibrationMessage("Calibrating Kou model...")
	kouModel := models.NewKouJumpDiffusion(riskFreeRate, avgVol, marketPrices, 1.0/252.0)
	globalModels.Kou = kouModel
	sendCalibrationMessage("Kou model calibrated.")

	// Calibrate CGMY model
	sendCalibrationMessage("Calibrating CGMY model...")
	cgmyProcess := models.NewCGMYProcess(0.1, 5.0, 10.0, 0.5) // Initial guess
	cgmyt := 1.0                                              // Use 1 year as a default time to maturity
	isCall := true                                            // Assume we're using call options for calibration
	cgmyProcess.Calibrate(marketPrices, strikes, underlyingPrice, riskFreeRate, cgmyt, isCall)
	globalModels.CGMY = cgmyProcess
	sendCalibrationMessage("CGMY model calibrated.")

	// Calibrate Heston model
	sendCalibrationMessage("Calibrating Heston model...")
	hestonModel := models.NewHestonModel(avgVol*avgVol, 2, avgVol*avgVol, 0.4, -0.5)
	err := hestonModel.Calibrate(marketPrices, strikes, s0, riskFreeRate, t)
	if err != nil {
		errMsg := fmt.Sprintf("Error calibrating Heston model: %v", err)
		sendCalibrationMessage(errMsg)
	} else {
		globalModels.Heston = hestonModel
		sendCalibrationMessage("Heston model calibrated.")
	}

	sendCalibrationMessage("All models calibrated successfully")
	return globalModels
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

func calculateAverageVolatility(volatilities map[string]float64) float64 {
	sum := 0.0
	count := 0
	for _, vol := range volatilities {
		sum += vol
		count++
	}
	if count == 0 {
		return 0
	}
	return sum / float64(count)
}

func calculateAverageImpliedVolatility(chain map[string]*tradier.OptionChain) float64 {
	sum := 0.0
	count := 0
	for _, expiration := range chain {
		for _, option := range expiration.Options.Option {
			if option.Greeks.MidIv > 0 {
				sum += option.Greeks.MidIv
				count++
			}
		}
	}
	if count == 0 {
		return 0
	}
	return sum / float64(count)
}

func extractHistoricalPrices(quotes tradier.QuoteHistory) []float64 {
	prices := make([]float64, len(quotes.History.Day))
	for i, day := range quotes.History.Day {
		prices[i] = day.Close
	}
	return prices
}

func extractAllStrikes(chain map[string]*tradier.OptionChain) []float64 {
	strikeSet := make(map[float64]struct{})
	for _, expiration := range chain {
		for _, option := range expiration.Options.Option {
			strikeSet[option.Strike] = struct{}{}
		}
	}
	strikes := make([]float64, 0, len(strikeSet))
	for strike := range strikeSet {
		strikes = append(strikes, strike)
	}
	sort.Float64s(strikes)
	return strikes
}

func calculateHistoricalJumps(quotes tradier.QuoteHistory) []float64 {
	jumps := []float64{}
	for i := 1; i < len(quotes.History.Day); i++ {
		prevClose := quotes.History.Day[i-1].Close
		currOpen := quotes.History.Day[i].Open
		jump := math.Log(currOpen / prevClose)
		jumps = append(jumps, jump)
	}
	return jumps
}
