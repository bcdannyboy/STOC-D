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

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	tradier_key := os.Getenv("TRADIER_KEY")

	symbols := []string{"SPY"}
	indicators := map[string]int{
		"SPY": 1,
	}

	minDTE := 5
	maxDTE := 45
	rfr := 0.0379
	minRoR := 0.175

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
		return
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
		vol := float64(allSpreads[i].Spread.ShortLeg.Option.Volume + allSpreads[i].Spread.LongLeg.Option.Volume)

		// Normalize probability (higher is better)
		normProb := (prob - minProb) / (maxProb - minProb)

		// Normalize VaR (lower is better)
		normVaR := 1 - ((var95 - minVaR) / (maxVaR - minVaR))

		// Normalize ES (lower is better)
		normES := 1 - ((es - minES) / (maxES - minES))

		// Normalize liquidity (lower is better, as lower spread means higher liquidity)
		normLiquidity := 1 - ((liquidity - minLiquidity) / (maxLiquidity - minLiquidity))

		// Calculate weighted score
		weightedScore := (normLiquidity * weightLiquidity) +
			(normProb * weightProbability) +
			(normVaR * weightVaR) +
			(normES * weightES)

		allSpreads[i].CompositeScore = weightedScore * vol
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
		return
	}

	f := "jspreads.json"
	err = ioutil.WriteFile(f, jspreads, 0644)
	if err != nil {
		fmt.Printf("Error writing to file %s: %s\n", f, err.Error())
		return
	}

	fmt.Printf("Successfully wrote %d spreads to %s\n", len(allSpreads), f)
}
