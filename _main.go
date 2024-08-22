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
	minDTE            = 5
	maxDTE            = 45
	rfr               = 0.0379
	minRoR            = 0.175
)

type StockScore struct {
	Symbol    string
	Score     float64
	Indicator float64
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	tradierKey := os.Getenv("TRADIER_KEY")

	// List of stocks to analyze
	symbols := []string{"PLTR", "AAPL", "MSFT", "GOOGL", "AMZN", "FB", "NVDA", "TSLA", "JPM", "V"}

	// Calculate scores for all stocks
	stockScores := calculateStockScores(symbols, tradierKey)

	// Sort stocks by score and take top 3
	sort.Slice(stockScores, func(i, j int) bool {
		return stockScores[i].Score > stockScores[j].Score
	})
	topStocks := stockScores[:3]

	// Create indicators map
	indicators := make(map[string]int)
	for _, stock := range topStocks {
		indicators[stock.Symbol] = int(math.Round(stock.Indicator))
	}

	fmt.Println("Top stocks:")
	for _, stock := range topStocks {
		fmt.Printf("%s: %.2f\n", stock.Symbol, stock.Score)
	}

	// Pass top stocks and indicators to STOCD
	STOCD(topStocks, indicators, tradierKey)
}

func calculateStockScores(symbols []string, tradierKey string) []StockScore {
	var stockScores []StockScore
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, symbol := range symbols {
		wg.Add(1)
		go func(sym string) {
			defer wg.Done()
			score := calculateScore(sym, tradierKey)
			mu.Lock()
			stockScores = append(stockScores, score)
			mu.Unlock()
		}(symbol)
	}
	wg.Wait()

	return stockScores
}

func calculateScore(symbol, tradierKey string) StockScore {
	chain, err := tradier.GET_OPTIONS_CHAIN(symbol, tradierKey, minDTE, maxDTE)
	if err != nil {
		log.Printf("Error fetching options chain for %s: %s", symbol, err)
		return StockScore{Symbol: symbol}
	}

	volume := calculateTotalVolume(chain)
	liquidity := calculateAverageLiquidity(chain)
	maxPain := calculateMaxPain(chain)

	// Normalize scores
	volumeScore := normalizeValue(volume, 0, 10000000)
	liquidityScore := 1 - normalizeValue(liquidity, 0, 0.1) // Lower is better for liquidity
	maxPainScore := normalizeValue(maxPain, 0, 100)

	// Calculate final score
	score := (volumeScore*0.4 + liquidityScore*0.4 + maxPainScore*0.2)

	// Calculate put/call ratio and liquidity bias
	putCallRatio := calculatePutCallRatio(chain)
	liquidityBias := calculateLiquidityBias(chain)

	// Translate to -1 to 1 score
	indicator := (putCallRatio-1)*-1 + liquidityBias
	indicator = math.Max(-1, math.Min(1, indicator))

	return StockScore{
		Symbol:    symbol,
		Score:     score,
		Indicator: indicator,
	}
}

func calculateTotalVolume(chain map[string]*tradier.OptionChain) float64 {
	totalVolume := 0
	for _, expiration := range chain {
		for _, option := range expiration.Options.Option {
			totalVolume += option.Volume
		}
	}
	return float64(totalVolume)
}

func calculateAverageLiquidity(chain map[string]*tradier.OptionChain) float64 {
	totalLiquidity := 0.0
	count := 0
	for _, expiration := range chain {
		for _, option := range expiration.Options.Option {
			if option.Ask != option.Bid {
				liquidity := (option.Ask - option.Bid) / ((option.Ask + option.Bid) / 2)
				totalLiquidity += liquidity
				count++
			}
		}
	}
	if count == 0 {
		return 0
	}
	return totalLiquidity / float64(count)
}

func calculateMaxPain(chain map[string]*tradier.OptionChain) float64 {
	strikeSum := 0.0
	count := 0
	for _, expiration := range chain {
		for _, option := range expiration.Options.Option {
			strikeSum += option.Strike
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return strikeSum / float64(count)
}

func calculatePutCallRatio(chain map[string]*tradier.OptionChain) float64 {
	putVolume := 0
	callVolume := 0
	for _, expiration := range chain {
		for _, option := range expiration.Options.Option {
			if option.OptionType == "put" {
				putVolume += option.Volume
			} else {
				callVolume += option.Volume
			}
		}
	}
	if callVolume == 0 {
		return 1 // Default to neutral if no call volume
	}
	return float64(putVolume) / float64(callVolume)
}

func calculateLiquidityBias(chain map[string]*tradier.OptionChain) float64 {
	shortTermVol := 0
	longTermVol := 0
	for _, expiration := range chain {
		expirationDate, _ := time.Parse("2006-01-02", expiration.ExpirationDate)
		daysToExpiration := int(expirationDate.Sub(time.Now()).Hours() / 24)
		for _, option := range expiration.Options.Option {
			if daysToExpiration <= 30 {
				shortTermVol += option.Volume
			} else {
				longTermVol += option.Volume
			}
		}
	}
	if longTermVol == 0 {
		return 0
	}
	return float64(shortTermVol-longTermVol) / float64(longTermVol)
}

func normalizeValue(value, min, max float64) float64 {
	if max == min {
		return 0.5 // Return middle value if min and max are the same
	}
	return (value - min) / (max - min)
}

func STOCD(topStocks []StockScore, indicators map[string]int, tradierKey string) {
	today := time.Now().Format("2006-01-02")
	tenyrsago := time.Now().AddDate(-10, 0, 0).Format("2006-01-02")
	var allSpreads []models.SpreadWithProbabilities

	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, stock := range topStocks {
		wg.Add(1)
		go func(stock StockScore) {
			defer wg.Done()

			quotes, err := tradier.GET_QUOTES(stock.Symbol, tenyrsago, today, "daily", tradierKey)
			if err != nil {
				fmt.Printf("Error fetching quotes for %s: %s\n", stock.Symbol, err.Error())
				return
			}

			optionsChains, err := tradier.GET_OPTIONS_CHAIN(stock.Symbol, tradierKey, minDTE, maxDTE)
			if err != nil {
				fmt.Printf("Error fetching options chain for %s: %s\n", stock.Symbol, err.Error())
				return
			}

			lastPrice := quotes.History.Day[len(quotes.History.Day)-1].Close

			fmt.Printf("Last price for %s: %.2f\n", stock.Symbol, lastPrice)
			fmt.Printf("Risk-free rate: %.4f\n", rfr)
			fmt.Printf("Minimum Return on Risk: %.2f\n", minRoR)

			var spreads []models.SpreadWithProbabilities
			indicator := indicators[stock.Symbol]
			if indicator > 0 {
				fmt.Printf("Identifying Bull Put Spreads for %s\n", stock.Symbol)
				BullPuts := positions.IdentifyBullPutSpreads(optionsChains, lastPrice, rfr, *quotes, minRoR, time.Now())
				spreads = BullPuts
			} else {
				fmt.Printf("Identifying Bear Call Spreads for %s\n", stock.Symbol)
				BearCalls := positions.IdentifyBearCallSpreads(optionsChains, lastPrice, rfr, *quotes, minRoR, time.Now())
				spreads = BearCalls
			}

			mu.Lock()
			allSpreads = append(allSpreads, spreads...)
			mu.Unlock()
		}(stock)
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

		minProb = math.Min(minProb, prob)
		maxProb = math.Max(maxProb, prob)
		minVaR = math.Min(minVaR, var95)
		maxVaR = math.Max(maxVaR, var95)
		minES = math.Min(minES, es)
		maxES = math.Max(maxES, es)
		minLiquidity = math.Min(minLiquidity, liquidity)
		maxLiquidity = math.Max(maxLiquidity, liquidity)
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
