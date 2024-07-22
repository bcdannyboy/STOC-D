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
	shortLeg := createSpreadLeg(shortOpt, underlyingPrice, riskFreeRate, history)
	longLeg := createSpreadLeg(longOpt, underlyingPrice, riskFreeRate, history)

	spreadCredit := shortLeg.Option.Bid - longLeg.Option.Ask

	// Calculate intrinsic value correctly
	var intrinsicValue float64
	if spreadType == "Bull Put" {
		if underlyingPrice < shortLeg.Option.Strike {
			intrinsicValue = shortLeg.Option.Strike - longLeg.Option.Strike
		} else {
			intrinsicValue = 0
		}
	} else { // Bear Call
		if underlyingPrice > shortLeg.Option.Strike {
			intrinsicValue = longLeg.Option.Strike - shortLeg.Option.Strike
		} else {
			intrinsicValue = 0
		}
	}

	extrinsicValue := spreadCredit - intrinsicValue

	// Use only the short leg's IV
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

	return models.OptionSpread{
		ShortLeg:       shortLeg,
		LongLeg:        longLeg,
		SpreadType:     spreadType,
		SpreadCredit:   spreadCredit,
		ExtrinsicValue: extrinsicValue,
		IntrinsicValue: intrinsicValue,
		ImpliedVol:     spreadImpliedVol,
		Greeks:         calculateSpreadGreeks(shortLeg, longLeg),
	}
}

func createSpreadLeg(option tradier.Option, underlyingPrice, riskFreeRate float64, history tradier.QuoteHistory) models.SpreadLeg {
	bsmResult := CalculateOptionMetrics(&option, underlyingPrice, riskFreeRate)
	garchResult := CalculateGARCHVolatility(history, option, underlyingPrice, riskFreeRate)

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

func sanitizeFloat(f float64) float64 {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0
	}
	return f
}

func calculateSpreadGreeks(shortLeg, longLeg models.SpreadLeg) models.BSMResult {
	return models.BSMResult{
		Price:             sanitizeFloat(shortLeg.BSMResult.Price - longLeg.BSMResult.Price),
		ImpliedVolatility: sanitizeFloat(shortLeg.BSMResult.ImpliedVolatility - longLeg.BSMResult.ImpliedVolatility),
		Delta:             sanitizeFloat(shortLeg.BSMResult.Delta - longLeg.BSMResult.Delta),
		Gamma:             sanitizeFloat(shortLeg.BSMResult.Gamma - longLeg.BSMResult.Gamma),
		Theta:             sanitizeFloat(shortLeg.BSMResult.Theta - longLeg.BSMResult.Theta),
		Vega:              sanitizeFloat(shortLeg.BSMResult.Vega - longLeg.BSMResult.Vega),
		Rho:               sanitizeFloat(shortLeg.BSMResult.Rho - longLeg.BSMResult.Rho),
		ShadowUpGamma:     sanitizeFloat(shortLeg.BSMResult.ShadowUpGamma - longLeg.BSMResult.ShadowUpGamma),
		ShadowDownGamma:   sanitizeFloat(shortLeg.BSMResult.ShadowDownGamma - longLeg.BSMResult.ShadowDownGamma),
		SkewGamma:         sanitizeFloat(shortLeg.BSMResult.SkewGamma - longLeg.BSMResult.SkewGamma),
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
	var wg sync.WaitGroup
	var mu sync.Mutex

	if len(chain) == 0 {
		fmt.Printf("Warning: Option chain is empty for %s spreads\n", spreadType)
		return spreadsWithProb
	}

	fmt.Printf("Identifying %s Spreads for underlying price: %.2f, Risk-Free Rate: %.4f, Min Return on Risk: %.4f\n", spreadType, underlyingPrice, riskFreeRate, minReturnOnRisk)

	// Calculate Garman-Klass volatility for the underlying
	gkResult := CalculateGarmanKlassVolatility(history)
	gkVolatility := gkResult.Volatility

	// Calculate Parkinson's volatility for the underlying
	parkinsonVolatility := CalculateParkinsonsVolatility(history)

	totalTasks := countTotalTasks(chain, strings.ToLower(spreadType[:3])) // "put" for Bull Put, "cal" for Bear Call
	progress := make(chan int, totalTasks)

	go printProgress(fmt.Sprintf("%s Spreads", spreadType), progress, totalTasks)

	for exp_date, expiration := range chain {
		wg.Add(1)
		go func(exp_date string, expiration *tradier.OptionChain) {
			defer wg.Done()
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

			for i := 0; i < len(options)-1; i++ {
				for j := i + 1; j < len(options); j++ {
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
						continue
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
