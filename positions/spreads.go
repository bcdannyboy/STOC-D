package positions

import (
	"fmt"
	"math"
	"runtime"
	"sync"
	"time"

	"github.com/bcdannyboy/dquant/models"
	"github.com/bcdannyboy/dquant/probability"
	"github.com/bcdannyboy/dquant/tradier"
)

func createOptionSpread(shortOpt, longOpt tradier.Option, spreadType string, underlyingPrice, riskFreeRate float64, history tradier.QuoteHistory, gkVolatility, parkinsonVolatility float64) models.OptionSpread {
	var wg sync.WaitGroup
	wg.Add(2)

	var shortLeg, longLeg models.SpreadLeg
	go func() {
		defer wg.Done()
		shortLeg = createSpreadLeg(shortOpt, underlyingPrice, riskFreeRate, history)
	}()
	go func() {
		defer wg.Done()
		longLeg = createSpreadLeg(longOpt, underlyingPrice, riskFreeRate, history)
	}()

	wg.Wait()

	spreadCredit := shortLeg.Option.Bid - longLeg.Option.Ask

	var intrinsicValue float64
	if spreadType == "Bull Put" {
		intrinsicValue = math.Max(0, shortLeg.Option.Strike-longLeg.Option.Strike-(underlyingPrice-longLeg.Option.Strike))
	} else {
		intrinsicValue = math.Max(0, longLeg.Option.Strike-shortLeg.Option.Strike-(longLeg.Option.Strike-underlyingPrice))
	}

	extrinsicValue := spreadCredit - intrinsicValue

	wg.Add(3)
	var shadowUpGamma, shadowDownGamma, skewGamma float64
	go func() {
		defer wg.Done()
		shadowUpGamma, shadowDownGamma = calculateShadowGammas(shortOpt, longOpt, underlyingPrice, riskFreeRate, shortLeg.BSMResult.ImpliedVolatility)
	}()
	go func() {
		defer wg.Done()
		skewGamma = calculateSkewGamma(shortOpt, longOpt, underlyingPrice, riskFreeRate, shortLeg.BSMResult.ImpliedVolatility)
	}()

	var spreadBSMPrice float64
	go func() {
		defer wg.Done()
		spreadBSMPrice = shortLeg.BSMResult.Price - longLeg.BSMResult.Price
	}()

	wg.Wait()

	shortLegIV := math.Max(shortLeg.Option.Greeks.MidIv, 0)

	spreadImpliedVol := models.SpreadImpliedVol{
		BidIV:               math.Max(shortLeg.Option.Greeks.BidIv, 0),
		AskIV:               math.Max(shortLeg.Option.Greeks.AskIv, 0),
		MidIV:               shortLegIV,
		GARCHIV:             shortLeg.GARCHResult.Volatility,
		BSMIV:               shortLeg.BSMResult.ImpliedVolatility,
		GarmanKlassIV:       gkVolatility,
		ParkinsonVolatility: parkinsonVolatility,
		ShortLegBSMIV:       shortLeg.BSMResult.ImpliedVolatility,
	}

	greeks := calculateSpreadGreeks(shortLeg, longLeg)
	greeks.ShadowUpGamma = shadowUpGamma
	greeks.ShadowDownGamma = shadowDownGamma
	greeks.SkewGamma = skewGamma

	return models.OptionSpread{
		ShortLeg:       shortLeg,
		LongLeg:        longLeg,
		SpreadType:     spreadType,
		SpreadCredit:   spreadCredit,
		SpreadBSMPrice: spreadBSMPrice,
		ExtrinsicValue: extrinsicValue,
		IntrinsicValue: intrinsicValue,
		ImpliedVol:     spreadImpliedVol,
		Greeks:         greeks,
	}
}

func createSpreadLeg(option tradier.Option, underlyingPrice, riskFreeRate float64, history tradier.QuoteHistory) models.SpreadLeg {
	var wg sync.WaitGroup
	wg.Add(2)

	var bsmResult BSMResult

	go func() {
		defer wg.Done()
		bsmResult = CalculateOptionMetrics(&option, underlyingPrice, riskFreeRate)
	}()

	wg.Wait()

	intrinsicValue := calculateIntrinsicValue(option, underlyingPrice)
	extrinsicValue := math.Max(0, bsmResult.Price-intrinsicValue)

	return models.SpreadLeg{
		Option:         option,
		BSMResult:      sanitizeBSMResult(bsmResult),
		BidImpliedVol:  math.Max(0, option.Greeks.BidIv),
		AskImpliedVol:  math.Max(0, option.Greeks.AskIv),
		MidImpliedVol:  math.Max(0, option.Greeks.MidIv),
		ExtrinsicValue: extrinsicValue,
		IntrinsicValue: intrinsicValue,
	}
}

func calculateIntrinsicValue(option tradier.Option, underlyingPrice float64) float64 {
	if option.OptionType == "call" {
		return math.Max(0, underlyingPrice-option.Strike)
	}
	return math.Max(0, option.Strike-underlyingPrice)
}

func sanitizeBSMResult(result BSMResult) models.BSMResult {
	return models.BSMResult{
		Price:             sanitizeFloat(result.Price),
		ImpliedVolatility: sanitizeFloat(result.ImpliedVolatility),
		Delta:             sanitizeFloat(result.Delta),
		Gamma:             sanitizeFloat(result.Gamma),
		Theta:             sanitizeFloat(result.Theta),
		Vega:              sanitizeFloat(result.Vega),
		Rho:               sanitizeFloat(result.Rho),
		ShadowUpGamma:     sanitizeFloat(result.ShadowUpGamma),
		ShadowDownGamma:   sanitizeFloat(result.ShadowDownGamma),
		SkewGamma:         sanitizeFloat(result.SkewGamma),
	}
}

func calculateSpreadIV(shortLeg, longLeg models.SpreadLeg, gkVolatility float64) models.SpreadImpliedVol {
	return models.SpreadImpliedVol{
		BidIV:         sanitizeFloat(shortLeg.BidImpliedVol - longLeg.BidImpliedVol),
		AskIV:         sanitizeFloat(shortLeg.AskImpliedVol - longLeg.AskImpliedVol),
		MidIV:         sanitizeFloat(shortLeg.MidImpliedVol - longLeg.MidImpliedVol),
		GARCHIV:       sanitizeFloat(shortLeg.GARCHResult.Volatility - longLeg.GARCHResult.Volatility),
		BSMIV:         sanitizeFloat(shortLeg.BSMResult.ImpliedVolatility - longLeg.BSMResult.ImpliedVolatility),
		ShortLegBSMIV: sanitizeFloat(shortLeg.BSMResult.ImpliedVolatility),
		GarmanKlassIV: gkVolatility,
	}
}

func calculateReturnOnRisk(spread models.OptionSpread) float64 {
	var maxRisk float64
	if spread.SpreadType == "Bull Put" {
		maxRisk = spread.ShortLeg.Option.Strike - spread.LongLeg.Option.Strike - spread.SpreadCredit
	} else { // Bear Call Spread
		maxRisk = spread.LongLeg.Option.Strike - spread.ShortLeg.Option.Strike - spread.SpreadCredit
	}

	if maxRisk <= 0 {
		fmt.Printf("Invalid maxRisk: %.2f for spread: Short Strike %.2f, Long Strike %.2f, Credit %.2f\n",
			maxRisk, spread.ShortLeg.Option.Strike, spread.LongLeg.Option.Strike, spread.SpreadCredit)
		return 0
	}

	returnOnRisk := spread.SpreadCredit / maxRisk
	return returnOnRisk
}

func filterPutOptions(options []tradier.Option) []tradier.Option {
	var puts []tradier.Option
	for _, opt := range options {
		if opt.OptionType == "put" {
			puts = append(puts, opt)
		}
	}
	return puts
}

func filterCallOptions(options []tradier.Option) []tradier.Option {
	var calls []tradier.Option
	for _, opt := range options {
		if opt.OptionType == "call" {
			calls = append(calls, opt)
		}
	}
	return calls
}

func IdentifySpreads(chain map[string]*tradier.OptionChain, underlyingPrice, riskFreeRate float64, history tradier.QuoteHistory, minReturnOnRisk float64, currentDate time.Time, spreadType string) []models.SpreadWithProbabilities {
	if len(chain) == 0 {
		fmt.Printf("Warning: Option chain is empty for %s spreads\n", spreadType)
		return nil
	}

	fmt.Printf("Identifying %s Spreads for underlying price: %.2f, Risk-Free Rate: %.4f, Min Return on Risk: %.4f\n", spreadType, underlyingPrice, riskFreeRate, minReturnOnRisk)

	gkVolatility := CalculateGarmanKlassVolatility(history).Volatility
	parkinsonVolatility := CalculateParkinsonsVolatility(history)

	maxPotentialSpreads := 0
	for _, expiration := range chain {
		options := filterOptions(expiration.Options.Option, spreadType)
		maxPotentialSpreads += (len(options) * (len(options) - 1)) / 2
	}
	fmt.Printf("Maximum potential spreads: %d\n", maxPotentialSpreads)

	allSpreads := make([]models.SpreadWithProbabilities, 0, maxPotentialSpreads)

	numWorkers := runtime.NumCPU() * 20 // Significantly increase the number of workers
	runtime.GOMAXPROCS(numWorkers)
	fmt.Printf("Using %d workers\n", numWorkers)

	var wg sync.WaitGroup
	spreadChan := make(chan models.SpreadWithProbabilities, numWorkers*100)
	errorChan := make(chan error, numWorkers)
	progressChan := make(chan int, maxPotentialSpreads)

	// batchSize := 1 // Process individual jobs for more even distribution
	jobChan := make(chan job, maxPotentialSpreads)

	worker := func(id int, jobChan <-chan job, spreadChan chan<- models.SpreadWithProbabilities, wg *sync.WaitGroup, errorChan chan<- error, progressChan chan<- int) {
		defer wg.Done()
		for j := range jobChan {
			preliminaryRoR := calculatePreliminaryRoR(j.option1, j.option2, j.spreadType)
			if preliminaryRoR >= j.minReturnOnRisk {
				spread := createOptionSpread(j.option1, j.option2, j.spreadType, j.underlyingPrice, j.riskFreeRate, j.history, j.gkVolatility, j.parkinsonVolatility)
				returnOnRisk := calculateReturnOnRisk(spread)
				if returnOnRisk >= j.minReturnOnRisk {
					probabilityResult := probability.MonteCarloSimulation(spread, j.underlyingPrice, j.riskFreeRate, j.daysToExpiration)
					select {
					case spreadChan <- models.SpreadWithProbabilities{
						Spread:      spread,
						Probability: probabilityResult,
					}:
					default:
						// If channel is full, don't block
						errorChan <- fmt.Errorf("worker %d: spread channel full, skipping spread", id)
					}
				}
			}
			progressChan <- 1
		}
	}

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go worker(i, jobChan, spreadChan, &wg, errorChan, progressChan)
	}

	go func() {
		for exp_date, expiration := range chain {
			options := filterOptions(expiration.Options.Option, spreadType)
			if len(options) == 0 {
				continue
			}

			expirationDate, err := time.Parse("2006-01-02", exp_date)
			if err != nil {
				fmt.Printf("Error parsing expiration date %s: %v\n", exp_date, err)
				continue
			}
			daysToExpiration := int(expirationDate.Sub(currentDate).Hours() / 24)

			for i := 0; i < len(options)-1; i++ {
				for j := i + 1; j < len(options); j++ {
					var option1, option2 tradier.Option
					if spreadType == "Bull Put" {
						if options[i].Strike > options[j].Strike {
							option1, option2 = options[i], options[j]
						} else {
							option1, option2 = options[j], options[i]
						}
					} else { // Bear Call
						if options[i].Strike < options[j].Strike {
							option1, option2 = options[i], options[j]
						} else {
							option1, option2 = options[j], options[i]
						}
					}
					jobChan <- job{
						spreadType:          spreadType,
						option1:             option1,
						option2:             option2,
						underlyingPrice:     underlyingPrice,
						riskFreeRate:        riskFreeRate,
						history:             history,
						gkVolatility:        gkVolatility,
						parkinsonVolatility: parkinsonVolatility,
						minReturnOnRisk:     minReturnOnRisk,
						daysToExpiration:    daysToExpiration,
					}
				}
			}
		}
		close(jobChan)
	}()

	go func() {
		wg.Wait()
		close(spreadChan)
		close(errorChan)
		close(progressChan)
		fmt.Println("\nAll workers finished")
	}()

	spreadsProcessed := 0
	spreadsIdentified := 0
	errorCount := 0
	lastPrintTime := time.Now()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case spreadWithProb, ok := <-spreadChan:
			if !ok {
				fmt.Printf("\nIdentified %d %s Spreads meeting minimum return on risk (Errors: %d)\n", spreadsIdentified, spreadType, errorCount)
				return allSpreads
			}
			allSpreads = append(allSpreads, spreadWithProb)
			spreadsIdentified++
		case <-errorChan:
			errorCount++
		case <-progressChan:
			spreadsProcessed++
		case <-ticker.C:
			now := time.Now()
			if now.Sub(lastPrintTime) >= time.Second {
				progress := float64(spreadsProcessed) / float64(maxPotentialSpreads) * 100
				fmt.Printf("\rProcessed %d/%d spreads: %.2f%% of total | Identified: %d | Errors: %d",
					spreadsProcessed, maxPotentialSpreads, progress, spreadsIdentified, errorCount)
				lastPrintTime = now
			}
		}
	}
}

func calculatePreliminaryRoR(option1, option2 tradier.Option, spreadType string) float64 {
	var spreadCredit, maxRisk float64
	if spreadType == "Bull Put" {
		spreadCredit = option1.Bid - option2.Ask
		maxRisk = option1.Strike - option2.Strike - spreadCredit
	} else { // Bear Call
		spreadCredit = option1.Bid - option2.Ask
		maxRisk = option2.Strike - option1.Strike - spreadCredit
	}

	if maxRisk <= 0 {
		return 0
	}

	return spreadCredit / maxRisk
}

func IdentifyBullPutSpreads(chain map[string]*tradier.OptionChain, underlyingPrice, riskFreeRate float64, history tradier.QuoteHistory, minReturnOnRisk float64, currentDate time.Time) []models.SpreadWithProbabilities {
	return IdentifySpreads(chain, underlyingPrice, riskFreeRate, history, minReturnOnRisk, currentDate, "Bull Put")
}

func IdentifyBearCallSpreads(chain map[string]*tradier.OptionChain, underlyingPrice, riskFreeRate float64, history tradier.QuoteHistory, minReturnOnRisk float64, currentDate time.Time) []models.SpreadWithProbabilities {
	return IdentifySpreads(chain, underlyingPrice, riskFreeRate, history, minReturnOnRisk, currentDate, "Bear Call")
}

func filterOptions(options []tradier.Option, spreadType string) []tradier.Option {
	if spreadType == "Bull Put" {
		return filterPutOptions(options)
	}
	return filterCallOptions(options)
}

func FilterSpreadsByProbability(spreads []models.SpreadWithProbabilities, minProbability float64) []models.SpreadWithProbabilities {
	var filteredSpreads []models.SpreadWithProbabilities
	for _, s := range spreads {
		if s.Probability.AverageProbability >= minProbability {
			filteredSpreads = append(filteredSpreads, s)
		}
	}
	return filteredSpreads
}
