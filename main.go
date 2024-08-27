package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bcdannyboy/stocd/models"
	"github.com/bcdannyboy/stocd/positions"
	"github.com/bcdannyboy/stocd/tradier"
	"github.com/joho/godotenv"
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"github.com/xhhuango/json"
)

const (
	weightLiquidity   = 0.5
	weightProbability = 0.3
	weightVaR         = 0.1
	weightES          = 0.1
)

func STOCD(indicators map[string]float64, minDTE, maxDTE, rfr, minRoR float64) string {
	tradier_key := os.Getenv("TRADIER_KEY")

	symbols := make([]string, len(indicators))
	for symbol := range indicators {
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

			optionsChains, err := tradier.GET_OPTIONS_CHAIN(symbol, tradier_key, int(minDTE), int(maxDTE))
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

		spreadStr := fmt.Sprintf(`
			<tr>
				<td>%d</td>
				<td>%.2f</td>
				<td>%.2f%%</td>
				<td>%.2f</td>
				<td>%d</td>
				<td>%.2f%%</td>
				<td>%.2f%%</td>
				<td>%s</td>
				<td>%s</td>
				<td>%.2f%%</td>
				<td>%.2f</td>
				<td>%.2f</td>
				<td>%.2f</td>
			</tr>`,
			i+1, CompositeScore, averageProbability, Liquidity, Vol, ExpectedShortfall,
			Var95, LongLeg, ShortLeg, RoR, AveragePrice, BSMPrice, MarketPrice)

		spreadStrings[i] = spreadStr
	}

	finalStr := fmt.Sprintf(`
		<h2>Top %d spreads</h2>
		<table border="1" cellpadding="5" cellspacing="0">
			<tr>
				<th>#</th>
				<th>Composite Score</th>
				<th>Average Probability</th>
				<th>Bid-Ask Spread</th>
				<th>Volume</th>
				<th>Expected Shortfall (ES)</th>
				<th>Value at Risk (VaR 95)</th>
				<th>Long Leg</th>
				<th>Short Leg</th>
				<th>Return on Risk (RoR)</th>
				<th>Average Price</th>
				<th>BSM Price</th>
				<th>Market Price</th>
			</tr>
			%s
		</table>`, len(allSpreads), strings.Join(spreadStrings, ""))

	fancyspreads := "fancyspreads.txt"
	err = ioutil.WriteFile(fancyspreads, []byte(finalStr), 0644)
	if err != nil {
		fmt.Printf("Error writing to file %s: %s\n", fancyspreads, err.Error())
		return "Error writing to file"
	}

	fmt.Printf("Successfully wrote top %d spreads to %s\n", len(allSpreads), fancyspreads)

	return finalStr
}

func sendEmail(subject, plainTextContent, htmlContent string) error {
	fromEmail := mail.NewEmail("STOC'D", "daniel@bcdefense.com")
	toEmail := mail.NewEmail("Daniel Bloom", "daniel@bcdefense.com") // Replace with the actual user's email

	message := mail.NewSingleEmail(fromEmail, subject, toEmail, plainTextContent, htmlContent)
	client := sendgrid.NewSendClient(os.Getenv("SENDGRID_API_KEY"))
	response, err := client.Send(message)
	if err != nil {
		return fmt.Errorf("error sending email: %v", err)
	}

	fmt.Printf("Email sent successfully! Status Code: %d\n", response.StatusCode)
	return nil
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Define flags
	symbol := flag.String("symbol", "", "Symbol to analyze")
	minDTE := flag.Float64("minDTE", 0, "Minimum DTE")
	maxDTE := flag.Float64("maxDTE", 0, "Maximum DTE")
	rfr := flag.Float64("rfr", 0, "Risk-free rate")
	indicator := flag.Float64("indicator", 0, "Indicator value (positive for Bull Put, negative for Bear Call)")
	minRoR := flag.Float64("minRoR", 0.175, "Minimum Return on Risk (RoR)")

	flag.Parse()

	if *symbol == "" {
		log.Fatal("Error: symbol is required")
	}

	indicators := map[string]float64{*symbol: *indicator}

	// Call STOCD with the parsed parameters
	result := STOCD(indicators, *minDTE, *maxDTE, *rfr, *minRoR)
	fmt.Printf("STOCD result for %s: %s\n", *symbol, result)

	finalOut := fmt.Sprintf("Symbol: %s\nMinDTE: %.2f\nMaxDTE: %.2f\nRisk Free Rate: %.4f\nIndicator: %.2f\n\n%s", *symbol, *minDTE, *maxDTE, *rfr, *indicator, result)

	// Send the result via email
	err = sendEmail("STOC'D Results", finalOut, finalOut)
	if err != nil {
		log.Fatal("Error sending email:", err)
	}
}
