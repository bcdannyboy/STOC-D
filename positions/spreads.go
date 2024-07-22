package positions

import (
	"fmt"
	"math"
	"strings"
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
	var garchResult GARCHResult

	go func() {
		defer wg.Done()
		bsmResult = CalculateOptionMetrics(&option, underlyingPrice, riskFreeRate)
	}()

	go func() {
		defer wg.Done()
		garchResult = CalculateGARCHVolatility(history, option, underlyingPrice, riskFreeRate)
	}()

	wg.Wait()

	intrinsicValue := calculateIntrinsicValue(option, underlyingPrice)
	extrinsicValue := math.Max(0, bsmResult.Price-intrinsicValue)

	return models.SpreadLeg{
		Option:         option,
		BSMResult:      sanitizeBSMResult(bsmResult),
		GARCHResult:    sanitizeGARCHResult(garchResult),
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

func sanitizeGARCHResult(result GARCHResult) models.GARCHResult {
	return models.GARCHResult{
		Params: models.GARCH11{
			Omega: sanitizeFloat(result.Params.Omega),
			Alpha: sanitizeFloat(result.Params.Alpha),
			Beta:  sanitizeFloat(result.Params.Beta),
		},
		Volatility: sanitizeFloat(result.Volatility),
		Greeks:     sanitizeBSMResult(result.Greeks),
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
	maxRisk := math.Abs(spread.ShortLeg.Option.Strike-spread.LongLeg.Option.Strike) - spread.SpreadCredit
	if maxRisk <= 0 {
		return 0 // Avoid division by zero or negative risk
	}
	return spread.SpreadCredit / maxRisk
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
	var spreadsWithProb []models.SpreadWithProbabilities
	var mu sync.Mutex
	var wg sync.WaitGroup

	if len(chain) == 0 {
		fmt.Printf("Warning: Option chain is empty for %s spreads\n", spreadType)
		return spreadsWithProb
	}

	fmt.Printf("Identifying %s Spreads for underlying price: %.2f, Risk-Free Rate: %.4f, Min Return on Risk: %.4f\n", spreadType, underlyingPrice, riskFreeRate, minReturnOnRisk)

	gkVolatility := CalculateGarmanKlassVolatility(history).Volatility
	parkinsonVolatility := CalculateParkinsonsVolatility(history)

	totalTasks := countTotalTasks(chain, strings.ToLower(spreadType[:3]))
	progress := make(chan int, totalTasks)

	go printProgress(fmt.Sprintf("%s Spreads", spreadType), progress, totalTasks)

	semaphore := make(chan struct{}, 10) // Limit concurrent goroutines to 10

	for exp_date, expiration := range chain {
		wg.Add(1)
		go func(exp_date string, expiration *tradier.OptionChain) {
			defer wg.Done()
			defer func() { <-semaphore }()
			semaphore <- struct{}{}

			var options []tradier.Option
			if spreadType == "Bull Put" {
				options = filterPutOptions(expiration.Options.Option)
			} else {
				options = filterCallOptions(expiration.Options.Option)
			}

			if len(options) == 0 {
				fmt.Printf("Warning: No %s options found for expiration date %s\n", strings.ToLower(spreadType[:3]), exp_date)
				return
			}

			expirationDate, err := time.Parse("2006-01-02", exp_date)
			if err != nil {
				fmt.Printf("Error parsing expiration date %s: %v\n", exp_date, err)
				return
			}
			daysToExpiration := int(expirationDate.Sub(currentDate).Hours() / 24)

			fmt.Printf("Analyzing %s spreads for expiration date: %s (DTE: %d)\n", spreadType, exp_date, daysToExpiration)

			spreadSemaphore := make(chan struct{}, 5) // Limit spread calculations to 5 at a time
			for i := 0; i < len(options)-1; i++ {
				for j := i + 1; j < len(options); j++ {
					spreadSemaphore <- struct{}{}
					go func(i, j int) {
						defer func() { <-spreadSemaphore }()
						var spread models.OptionSpread
						if spreadType == "Bull Put" {
							if options[i].Strike > options[j].Strike {
								spread = createOptionSpread(options[i], options[j], spreadType, underlyingPrice, riskFreeRate, history, gkVolatility, parkinsonVolatility)
							} else {
								spread = createOptionSpread(options[j], options[i], spreadType, underlyingPrice, riskFreeRate, history, gkVolatility, parkinsonVolatility)
							}
						} else if spreadType == "Bear Call" {
							if options[i].Strike < options[j].Strike {
								spread = createOptionSpread(options[i], options[j], spreadType, underlyingPrice, riskFreeRate, history, gkVolatility, parkinsonVolatility)
							} else {
								spread = createOptionSpread(options[j], options[i], spreadType, underlyingPrice, riskFreeRate, history, gkVolatility, parkinsonVolatility)
							}
						} else {
							return
						}

						returnOnRisk := calculateReturnOnRisk(spread)
						if returnOnRisk >= minReturnOnRisk {
							probabilityResult := probability.MonteCarloSimulationBatch([]models.OptionSpread{spread}, underlyingPrice, riskFreeRate, daysToExpiration)[0]
							spreadWithProb := models.SpreadWithProbabilities{
								Spread:      spread,
								Probability: probabilityResult,
							}
							mu.Lock()
							spreadsWithProb = append(spreadsWithProb, spreadWithProb)
							mu.Unlock()
						}
						progress <- 1
					}(i, j)
				}
			}
		}(exp_date, expiration)
	}

	wg.Wait()
	close(progress)

	fmt.Printf("\nIdentified %d %s Spreads meeting minimum return on risk\n", len(spreadsWithProb), spreadType)
	return spreadsWithProb
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

func IdentifyBullPutSpreads(chain map[string]*tradier.OptionChain, underlyingPrice, riskFreeRate float64, history tradier.QuoteHistory, minReturnOnRisk float64, currentDate time.Time) []models.SpreadWithProbabilities {
	return IdentifySpreads(chain, underlyingPrice, riskFreeRate, history, minReturnOnRisk, currentDate, "Bull Put")
}

func IdentifyBearCallSpreads(chain map[string]*tradier.OptionChain, underlyingPrice, riskFreeRate float64, history tradier.QuoteHistory, minReturnOnRisk float64, currentDate time.Time) []models.SpreadWithProbabilities {
	return IdentifySpreads(chain, underlyingPrice, riskFreeRate, history, minReturnOnRisk, currentDate, "Bear Call")
}
