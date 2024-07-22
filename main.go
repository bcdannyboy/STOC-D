package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sort"
	"time"

	"github.com/bcdannyboy/dquant/models"
	"github.com/bcdannyboy/dquant/positions"
	"github.com/bcdannyboy/dquant/tradier"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	tradier_key := os.Getenv("TRADIER_KEY")

	Symbol := "EBS"
	minDTE := 5
	maxDTE := 45
	rfr := 0.0416
	indicator := 1
	minRoR := 0.1

	today := time.Now().Format("2006-01-02")
	tenyrsago := time.Now().AddDate(-10, 0, 0).Format("2006-01-02")
	quotes, err := tradier.GET_QUOTES(Symbol, tenyrsago, today, "daily", tradier_key)
	if err != nil {
		fmt.Printf("Error fetching quotes for %s: %s\n", Symbol, err.Error())
		return
	}

	fmt.Printf("Calculating Parkinsons Metrics for %s\n", Symbol)
	ParkinsonsResult := positions.CalculateParkinsonsMetrics(*quotes)

	fP := "ParkinsonsResult.json"
	jParkinsonsResult, err := json.Marshal(ParkinsonsResult)
	if err != nil {
		fmt.Printf("Error marshalling ParkinsonsResult: %s\n", err.Error())
		return
	}

	err = ioutil.WriteFile(fP, jParkinsonsResult, 0644)
	if err != nil {
		fmt.Printf("Error writing to file %s: %s\n", fP, err.Error())
		return
	}

	optionsChains, err := tradier.GET_OPTIONS_CHAIN(Symbol, tradier_key, minDTE, maxDTE)
	if err != nil {
		fmt.Printf("Error fetching options chain for %s: %s\n", Symbol, err.Error())
		return
	}

	last_price := quotes.History.Day[len(quotes.History.Day)-1].Close

	spreads := []models.SpreadWithProbabilities{}
	if indicator > 0 {
		fmt.Printf("Identifying Bull Put Spreads for %s\n", Symbol)
		BullPuts := positions.IdentifyBullPutSpreads(optionsChains, last_price, rfr, *quotes, minRoR, time.Now())
		spreads = BullPuts
	} else {
		fmt.Printf("Identifying Bear Call Spreads for %s\n", Symbol)
		BearCalls := positions.IdentifyBearCallSpreads(optionsChains, last_price, rfr, *quotes, minRoR, time.Now())
		spreads = BearCalls
	}

	sort.Slice(spreads, func(i, j int) bool {
		avgProbI := averageProbability(spreads[i].Probabilities)
		avgProbJ := averageProbability(spreads[j].Probabilities)
		return avgProbI > avgProbJ
	})

	if len(spreads) > 10 {
		spreads = spreads[:10]
	}

	jspreads, err := json.Marshal(spreads)
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
}

func averageProbability(probabilities map[string]float64) float64 {
	total := 0.0
	count := 0
	for _, prob := range probabilities {
		total += prob
		count++
	}
	if count == 0 {
		return 0
	}
	return total / float64(count)
}
