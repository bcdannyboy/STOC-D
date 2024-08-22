package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/bcdannyboy/stocd/models"
	"github.com/bcdannyboy/stocd/positions"
	"github.com/bcdannyboy/stocd/tradier"
	"github.com/joho/godotenv"
	"github.com/xhhuango/json"
)

const (
	weightLiquidity   = 0.5
	weightProbability = 0.3
	weightVaR         = 0.1
	weightES          = 0.1
)

func STOCD(indicators map[string]float64, minDTE, maxDTE, rfr, minRoR float64) string {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	tradier_key := os.Getenv("TRADIER_KEY")

	symbols := make([]string, len(indicators))
	for symbol, _ := range indicators {
		symbols = append(symbols, symbol)
	}

	today := time.Now().Format("2006-01-02")
	tenyrsago := time.Now().AddDate(-10, 0, 0).Format("2006-01-02")
	var allSpreads []models.SpreadWithProbabilities

	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, symbol := range symbols {
		wg.Add(1)
		go func(symbol string) {
			defer wg.Done()

			quotes, err := tradier.GET_QUOTES(symbol, tenyrsago, today, "daily", tradier_key)
			if err != nil {
				fmt.Printf("Error fetching quotes for %s: %s\n", symbol, err.Error())
				return
			}

			optionsChains, err := tradier.GET_OPTIONS_CHAIN(symbol, tradier_key, minDTE, maxDTE)
			if err != nil {
				fmt.Printf("Error fetching options chain for %s: %s\n", symbol, err.Error())
				return
			}

			last_price := quotes.History.Day[len(quotes.History.Day)-1].Close

			fmt.Printf("Last price for %s: %.2f\n", symbol, last_price)
			fmt.Printf("Risk-free rate: %.4f\n", rfr)
			fmt.Printf("Minimum Return on Risk: %.2f\n", minRoR)

			var spreads []models.SpreadWithProbabilities
			indicator := indicators[symbol]
			if indicator > 0 {
				fmt.Printf("Identifying Bull Put Spreads for %s\n", symbol)
				BullPuts := positions.IdentifyBullPutSpreads(optionsChains, last_price, rfr, *quotes, minRoR, time.Now())
				spreads = BullPuts
			} else {
				fmt.Printf("Identifying Bear Call Spreads for %s\n", symbol)
				BearCalls := positions.IdentifyBearCallSpreads(optionsChains, last_price, rfr, *quotes, minRoR, time.Now())
				spreads = BearCalls
			}

			mu.Lock()
			allSpreads = append(allSpreads, spreads...)
			mu.Unlock()
		}(symbol)
	}

	wg.Wait()

	fmt.Printf("Number of identified spreads: %d\n", len(allSpreads))
	if len(allSpreads) == 0 {
		fmt.Println("No spreads identified. Check minRoR and other parameters.")
		return "No spreads identified. Check minRoR and other parameters."
	}

	// Find the min and max values across all spreads
	var minProb, maxProb, minVaR, maxVaR, minES, maxES, minLiquidity, maxLiquidity float64
	maxLiquidity = math.Inf(-1) // Initialize to negative infinity
	minLiquidity = math.Inf(1)  // Initialize to positive infinity
	for i := range allSpreads {
		prob := allSpreads[i].Probability.AverageProbability
		var95 := math.Abs(allSpreads[i].VaR95)
		es := math.Abs(allSpreads[i].ExpectedShortfall)
		liquidity := allSpreads[i].Liquidity

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

	for i := range allSpreads {
		prob := allSpreads[i].Probability.AverageProbability
		var95 := math.Abs(allSpreads[i].VaR95)
		es := math.Abs(allSpreads[i].ExpectedShortfall)
		liquidity := allSpreads[i].Liquidity
		vol := float64(allSpreads[i].Spread.ShortLeg.Option.Volume + allSpreads[i].Spread.LongLeg.Option.Volume)

		// Normalize values (avoid division by zero)
		normProb := normalizeValue(prob, minProb, maxProb)
		normVaR := 1 - normalizeValue(var95, minVaR, maxVaR)                       // Invert so lower is better
		normES := 1 - normalizeValue(es, minES, maxES)                             // Invert so lower is better
		normLiquidity := 1 - normalizeValue(liquidity, minLiquidity, maxLiquidity) // Invert so lower is better

		// Calculate weighted score
		weightedScore := (normLiquidity * weightLiquidity) +
			(normProb * weightProbability) +
			(normVaR * weightVaR) +
			(normES * weightES)

		allSpreads[i].CompositeScore = weightedScore * (1 + math.Log1p(vol)) // Use log to dampen the effect of volume
	}

	sort.Slice(allSpreads, func(i, j int) bool {
		return allSpreads[i].CompositeScore > allSpreads[j].CompositeScore
	})

	if len(allSpreads) > 10 {
		allSpreads = allSpreads[:10]
	}

	jspreads, err := json.Marshal(allSpreads)
	if err != nil {
		fmt.Printf("Error marshalling spreads: %s\n", err.Error())
		return "Error marshalling spreads"
	}

	f := "jspreads.json"
	err = ioutil.WriteFile(f, jspreads, 0644)
	if err != nil {
		fmt.Printf("Error writing to file %s: %s\n", f, err.Error())
		return "Error writing to file"
	}

	fmt.Printf("Successfully wrote %d spreads to %s\n", len(allSpreads), f)
	fmt.Printf("--------------------\n")

	spreadStrings := make([]string, len(allSpreads))
	for i, spread := range allSpreads {
		LongLeg := spread.Spread.LongLeg.Option.Description
		ShortLeg := spread.Spread.ShortLeg.Option.Description
		RoR := spread.Spread.ROR * 100 // Convert to percentage
		CompositeScore := spread.CompositeScore
		ExpectedShortfall := spread.ExpectedShortfall * 100               // Convert to percentage
		averageProbability := spread.Probability.AverageProbability * 100 // Convert to percentage
		BSMPrice := spread.Spread.SpreadBSMPrice
		MarketPrice := spread.Spread.SpreadCredit
		AveragePrice := (BSMPrice + MarketPrice) / 2
		Vol := spread.Spread.ShortLeg.Option.Volume + spread.Spread.LongLeg.Option.Volume
		Liquidity := spread.Liquidity
		Var95 := spread.VaR95 * 100 // Convert to percentage

		spreadStr := fmt.Sprintf("Spread #%d (%.2f)\n", i+1, CompositeScore)
		spreadStr += fmt.Sprintf("Average Probability: %.2f%%\n", averageProbability)
		spreadStr += fmt.Sprintf("Bid-Ask Spread: %.2f\n", Liquidity)
		spreadStr += fmt.Sprintf("Volume: %d\n", Vol)
		spreadStr += fmt.Sprintf("Expected Shortfall (ES): %.2f%%\n", ExpectedShortfall)
		spreadStr += fmt.Sprintf("Value at Risk (VaR 95): %.2f%%\n", Var95)
		spreadStr += fmt.Sprintf("Long Leg: %s\n", LongLeg)
		spreadStr += fmt.Sprintf("Short Leg: %s\n", ShortLeg)
		spreadStr += fmt.Sprintf("Return on Risk (RoR): %.2f%%\n", RoR)
		spreadStr += fmt.Sprintf("Average Price: %.2f\n", AveragePrice)
		spreadStr += fmt.Sprintf("BSM Price: %.2f\n", BSMPrice)
		spreadStr += fmt.Sprintf("Market Price: %.2f\n", MarketPrice)

		spreadStrings[i] = spreadStr
	}

	finalStr := fmt.Sprintf("Top %d spreads\n", len(allSpreads))
	finalStr += "--------------------\n"

	for i, spreadStr := range spreadStrings {
		finalStr += spreadStr + "\n"
		if i < len(spreadStrings)-1 {
			finalStr += "--------------------\n"
		}
	}

	fancyspreads := "fancyspreads.txt"
	err = ioutil.WriteFile(fancyspreads, []byte(finalStr), 0644)
	if err != nil {
		fmt.Printf("Error writing to file %s: %s\n", fancyspreads, err.Error())
		return "Error writing to file"
	}

	fmt.Printf("Successfully wrote top %d spreads to %s\n", len(allSpreads), fancyspreads)

	return finalStr
}

func main() {

}
