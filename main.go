package main

import (
	"fmt"
	"io/ioutil"
	"log"
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

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	tradier_key := os.Getenv("TRADIER_KEY")

	symbols := []string{"RDDT"}
	indicators := map[string]int{
		"RDDT": 1,
	}

	minDTE := 5
	maxDTE := 45
	rfr := 0.0394
	minRoR := 0.2

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

	sort.Slice(allSpreads, func(i, j int) bool {
		return allSpreads[i].Probability.AverageProbability > allSpreads[j].Probability.AverageProbability
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
