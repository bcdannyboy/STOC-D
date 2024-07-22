// positions/spreads.go
package positions

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/bcdannyboy/dquant/models"
	"github.com/bcdannyboy/dquant/probability"
	"github.com/bcdannyboy/dquant/tradier"
)

func createOptionSpread(shortOpt, longOpt tradier.Option, spreadType string, underlyingPrice, riskFreeRate float64, history tradier.QuoteHistory) models.OptionSpread {
	shortLeg := createSpreadLeg(shortOpt, underlyingPrice, riskFreeRate, history)
	longLeg := createSpreadLeg(longOpt, underlyingPrice, riskFreeRate, history)

	spreadCredit := shortLeg.Option.Bid - longLeg.Option.Ask
	spreadBSMPrice := shortLeg.BSMResult.Price - longLeg.BSMResult.Price

	extrinsicValue := math.Max(spreadCredit-(longOpt.Strike-shortOpt.Strike), 0)
	intrinsicValue := math.Max((longOpt.Strike-shortOpt.Strike)-spreadCredit, 0)

	// Calculate spread Greeks
	spreadGreeks := calculateSpreadGreeks(shortLeg, longLeg)

	// Calculate spread implied volatilities
	spreadIV := calculateSpreadIV(shortLeg, longLeg)

	return models.OptionSpread{
		ShortLeg:       shortLeg,
		LongLeg:        longLeg,
		SpreadType:     spreadType,
		SpreadCredit:   spreadCredit,
		SpreadBSMPrice: spreadBSMPrice,
		ExtrinsicValue: extrinsicValue,
		IntrinsicValue: intrinsicValue,
		Greeks:         spreadGreeks,
		ImpliedVol:     spreadIV,
	}
}

func createSpreadLeg(option tradier.Option, underlyingPrice, riskFreeRate float64, history tradier.QuoteHistory) models.SpreadLeg {
	bsmResult := CalculateOptionMetrics(&option, underlyingPrice, riskFreeRate)
	garchResult := CalculateGARCHVolatility(history, option, underlyingPrice, riskFreeRate)

	// Calculate Garman-Klass volatility
	garmanKlassResults := CalculateGarmanKlassVolatility(history)
	var garmanKlassResult models.GarmanKlassResult
	if len(garmanKlassResults) > 0 {
		garmanKlassResult = models.GarmanKlassResult{
			Period:     garmanKlassResults[0].Period,
			Volatility: garmanKlassResults[0].Volatility,
		}
	}

	extrinsicValue := math.Max(option.Bid-math.Max(underlyingPrice-option.Strike, 0), 0)
	intrinsicValue := math.Max(underlyingPrice-option.Strike, 0)

	return models.SpreadLeg{
		Option: option,
		BSMResult: models.BSMResult{
			Price:             bsmResult.Price,
			ImpliedVolatility: bsmResult.ImpliedVolatility,
			Delta:             bsmResult.Delta,
			Gamma:             bsmResult.Gamma,
			Theta:             bsmResult.Theta,
			Vega:              bsmResult.Vega,
			Rho:               bsmResult.Rho,
			ShadowUpGamma:     bsmResult.ShadowUpGamma,
			ShadowDownGamma:   bsmResult.ShadowDownGamma,
			SkewGamma:         bsmResult.SkewGamma,
		},
		GARCHResult: models.GARCHResult{
			Params: models.GARCH11{
				Omega: garchResult.Params.Omega,
				Alpha: garchResult.Params.Alpha,
				Beta:  garchResult.Params.Beta,
			},
			Volatility: garchResult.Volatility,
			Greeks: models.BSMResult{
				Price:             garchResult.Greeks.Price,
				ImpliedVolatility: garchResult.Greeks.ImpliedVolatility,
				Delta:             garchResult.Greeks.Delta,
				Gamma:             garchResult.Greeks.Gamma,
				Theta:             garchResult.Greeks.Theta,
				Vega:              garchResult.Greeks.Vega,
				Rho:               garchResult.Greeks.Rho,
				ShadowUpGamma:     garchResult.Greeks.ShadowUpGamma,
				ShadowDownGamma:   garchResult.Greeks.ShadowDownGamma,
				SkewGamma:         garchResult.Greeks.SkewGamma,
			},
		},
		GarmanKlassResult: garmanKlassResult,
		BidImpliedVol:     option.Greeks.BidIv,
		AskImpliedVol:     option.Greeks.AskIv,
		MidImpliedVol:     option.Greeks.MidIv,
		ExtrinsicValue:    extrinsicValue,
		IntrinsicValue:    intrinsicValue,
	}
}

func calculateSpreadGreeks(shortLeg, longLeg models.SpreadLeg) models.BSMResult {
	return models.BSMResult{
		Price:             shortLeg.BSMResult.Price - longLeg.BSMResult.Price,
		ImpliedVolatility: shortLeg.BSMResult.ImpliedVolatility - longLeg.BSMResult.ImpliedVolatility,
		Delta:             shortLeg.BSMResult.Delta - longLeg.BSMResult.Delta,
		Gamma:             shortLeg.BSMResult.Gamma - longLeg.BSMResult.Gamma,
		Theta:             shortLeg.BSMResult.Theta - longLeg.BSMResult.Theta,
		Vega:              shortLeg.BSMResult.Vega - longLeg.BSMResult.Vega,
		Rho:               shortLeg.BSMResult.Rho - longLeg.BSMResult.Rho,
		ShadowUpGamma:     shortLeg.BSMResult.ShadowUpGamma - longLeg.BSMResult.ShadowUpGamma,
		ShadowDownGamma:   shortLeg.BSMResult.ShadowDownGamma - longLeg.BSMResult.ShadowDownGamma,
		SkewGamma:         shortLeg.BSMResult.SkewGamma - longLeg.BSMResult.SkewGamma,
	}
}

func calculateSpreadIV(shortLeg, longLeg models.SpreadLeg) models.SpreadImpliedVol {
	return models.SpreadImpliedVol{
		BidIV:         shortLeg.BidImpliedVol - longLeg.BidImpliedVol,
		AskIV:         shortLeg.AskImpliedVol - longLeg.AskImpliedVol,
		MidIV:         shortLeg.MidImpliedVol - longLeg.MidImpliedVol,
		GARCHIV:       shortLeg.GARCHResult.Volatility - longLeg.GARCHResult.Volatility,
		BSMIV:         shortLeg.BSMResult.ImpliedVolatility - longLeg.BSMResult.ImpliedVolatility,
		GarmanKlassIV: shortLeg.GarmanKlassResult.Volatility - longLeg.GarmanKlassResult.Volatility,
	}
}

func calculateReturnOnRisk(spread models.OptionSpread) float64 {
	if spread.SpreadType == "Bull Put" {
		maxRisk := spread.LongLeg.Option.Strike - spread.ShortLeg.Option.Strike - spread.SpreadCredit
		if maxRisk <= 0 {
			return 0 // Avoid division by zero
		}
		return spread.SpreadCredit / maxRisk
	} else if spread.SpreadType == "Bear Call" {
		maxRisk := spread.LongLeg.Option.Strike - spread.ShortLeg.Option.Strike - spread.SpreadCredit
		if maxRisk <= 0 {
			return 0 // Avoid division by zero
		}
		return spread.SpreadCredit / maxRisk
	}
	return 0 // Default case
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

func FilterSpreadsByProbability(spreads []models.SpreadWithProbabilities, minProbability float64) []models.SpreadWithProbabilities {
	var filteredSpreads []models.SpreadWithProbabilities
	for _, s := range spreads {
		avgProbability := (s.Probabilities["Normal"] + s.Probabilities["StudentT"] + s.Probabilities["GBM"] + s.Probabilities["PoissonJump"]) / 4
		if avgProbability >= minProbability {
			filteredSpreads = append(filteredSpreads, s)
		}
	}
	return filteredSpreads
}

func IdentifyBullPutSpreads(chain map[string]*tradier.OptionChain, underlyingPrice, riskFreeRate float64, history tradier.QuoteHistory, minReturnOnRisk float64, currentDate time.Time) []models.SpreadWithProbabilities {
	var spreadsWithProb []models.SpreadWithProbabilities
	var wg sync.WaitGroup
	var mu sync.Mutex

	totalTasks := countTotalTasks(chain, "put")
	progress := make(chan int, totalTasks)

	go printProgress("Bull Put Spreads", progress, totalTasks)

	for _, expiration := range chain {
		wg.Add(1)
		go func(expiration *tradier.OptionChain) {
			defer wg.Done()
			puts := filterPutOptions(expiration.Options.Option)

			expirationDate, err := time.Parse("2006-01-02", expiration.ExpirationDate)
			if err != nil {
				fmt.Printf("Error parsing expiration date: %v\n", err)
				return
			}
			daysToExpiration := int(expirationDate.Sub(currentDate).Hours() / 24)

			for i := 0; i < len(puts)-1; i++ {
				for j := i + 1; j < len(puts); j++ {
					if puts[i].Strike < puts[j].Strike {
						spread := createOptionSpread(puts[i], puts[j], "Bull Put", underlyingPrice, riskFreeRate, history)
						returnOnRisk := calculateReturnOnRisk(spread)
						if returnOnRisk >= minReturnOnRisk {
							probabilities := probability.MonteCarloSimulationBatch([]models.OptionSpread{spread}, underlyingPrice, riskFreeRate, daysToExpiration)[0].Probabilities
							spreadWithProb := models.SpreadWithProbabilities{
								Spread:        spread,
								Probabilities: probabilities,
							}
							mu.Lock()
							spreadsWithProb = append(spreadsWithProb, spreadWithProb)
							mu.Unlock()
						}
					}
					progress <- 1
				}
			}
		}(expiration)
	}

	wg.Wait()
	close(progress)

	fmt.Printf("\nIdentified %d Bull Put Spreads meeting minimum return on risk\n", len(spreadsWithProb))
	return spreadsWithProb
}

func IdentifyBearCallSpreads(chain map[string]*tradier.OptionChain, underlyingPrice, riskFreeRate float64, history tradier.QuoteHistory, minReturnOnRisk float64, currentDate time.Time) []models.SpreadWithProbabilities {
	var spreadsWithProb []models.SpreadWithProbabilities
	var wg sync.WaitGroup
	var mu sync.Mutex

	totalTasks := countTotalTasks(chain, "call")
	progress := make(chan int, totalTasks)

	go printProgress("Bear Call Spreads", progress, totalTasks)

	for _, expiration := range chain {
		wg.Add(1)
		go func(expiration *tradier.OptionChain) {
			defer wg.Done()
			calls := filterCallOptions(expiration.Options.Option)

			expirationDate, err := time.Parse("2006-01-02", expiration.ExpirationDate)
			if err != nil {
				fmt.Printf("Error parsing expiration date: %v\n", err)
				return
			}
			daysToExpiration := int(expirationDate.Sub(currentDate).Hours() / 24)

			for i := 0; i < len(calls)-1; i++ {
				for j := i + 1; j < len(calls); j++ {
					if calls[i].Strike < calls[j].Strike {
						spread := createOptionSpread(calls[i], calls[j], "Bear Call", underlyingPrice, riskFreeRate, history)
						returnOnRisk := calculateReturnOnRisk(spread)
						if returnOnRisk >= minReturnOnRisk {
							probabilities := probability.MonteCarloSimulationBatch([]models.OptionSpread{spread}, underlyingPrice, riskFreeRate, daysToExpiration)[0].Probabilities
							spreadWithProb := models.SpreadWithProbabilities{
								Spread:        spread,
								Probabilities: probabilities,
							}
							mu.Lock()
							spreadsWithProb = append(spreadsWithProb, spreadWithProb)
							mu.Unlock()
						}
					}
					progress <- 1
				}
			}
		}(expiration)
	}

	wg.Wait()
	close(progress)

	fmt.Printf("\nIdentified %d Bear Call Spreads meeting minimum return on risk\n", len(spreadsWithProb))
	return spreadsWithProb
}
