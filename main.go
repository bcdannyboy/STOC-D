package main

import (
	"fmt"
	"log"
	"os"
	"time"

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

	Symbol := "AAPL"
	minDTE := 5
	maxDTE := 45
	rfr := 0.0416
	indicator := 1

	today := time.Now().Format("2006-01-02")
	tenyrsago := time.Now().AddDate(-10, 0, 0).Format("2006-01-02")
	quotes, err := tradier.GET_QUOTES(Symbol, today, tenyrsago, "daily", tradier_key)
	if err != nil {
		fmt.Printf("Error fetching quotes for %s: %s\n", Symbol, err.Error())
		return
	}

	optionsChains, err := tradier.GET_OPTIONS_CHAIN(Symbol, tradier_key, minDTE, maxDTE)
	if err != nil {
		fmt.Printf("Error fetching options chain for %s: %s\n", Symbol, err.Error())
		return
	}

	last_price := quotes.History.Day[len(quotes.History.Day)-1].Close

	spreads := []positions.OptionSpread{}
	if indicator > 1 {
		BullPuts := positions.IdentifyBullPutSpreads(optionsChains, last_price, rfr, *quotes)

		spreads = BullPuts
	} else {
		BearCalls := positions.IdentifyBearCallSpreads(optionsChains, last_price, rfr, *quotes)

		spreads = BearCalls
	}
}
