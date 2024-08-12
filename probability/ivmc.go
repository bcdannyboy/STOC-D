package probability

import (
	"math"
	"sync"

	"github.com/bcdannyboy/dquant/models"
	"github.com/bcdannyboy/dquant/tradier"
	"golang.org/x/exp/rand"
)

const (
	numSimulations = 1000
	timeSteps      = 252 // Assuming 252 trading days in a year
	numWorkers     = 32
)

var rngPool = sync.Pool{
	New: func() interface{} {
		return rand.New(rand.NewSource(uint64(rand.Int63())))
	},
}

var globalHestonModel *models.HestonModel

func calibrateGlobalHestonModel(history tradier.QuoteHistory, chain map[string]*tradier.OptionChain, riskFreeRate float64) {
	if globalHestonModel != nil {
		return // Model already calibrated
	}

	marketPrices := extractHistoricalPrices(history)
	strikes := extractAllStrikes(chain)
	s0 := marketPrices[len(marketPrices)-1]
	t := 1.0 // Use 1 year as a default time to maturity

	initialGuess := models.NewHestonModel(0.04, 2, 0.04, 0.4, -0.5)
	err := initialGuess.Calibrate(marketPrices, strikes, s0, riskFreeRate, t)
	if err != nil {
		// Handle error, perhaps log it
		globalHestonModel = initialGuess // Use initial guess if calibration fails
	} else {
		globalHestonModel = initialGuess
	}
}

func MonteCarloSimulation(spread models.OptionSpread, underlyingPrice, riskFreeRate float64, daysToExpiration int, yangzhangVolatilities, rogerssatchelVolatilities map[string]float64, localVolSurface models.VolatilitySurface, history tradier.QuoteHistory, chain map[string]*tradier.OptionChain) models.SpreadWithProbabilities {
	calibrateGlobalHestonModel(history, chain, riskFreeRate)

	shortLegVol, longLegVol := confirmVolatilities(spread, localVolSurface, daysToExpiration, yangzhangVolatilities, rogerssatchelVolatilities)

	volatilities := calculateVolatilities(shortLegVol, longLegVol, daysToExpiration, yangzhangVolatilities, rogerssatchelVolatilities, localVolSurface, history, spread)

	simulationFuncs := []struct {
		name string
		fn   func(models.OptionSpread, float64, float64, float64, int, *rand.Rand, tradier.QuoteHistory) map[string]float64
	}{
		{name: "Merton", fn: simulateMertonJumpDiffusion},
		{name: "Kou", fn: simulateKouJumpDiffusion},
		{name: "Heston", fn: simulateHeston},
	}

	results := make(map[string]float64, len(volatilities)*len(simulationFuncs)*5)
	var wg sync.WaitGroup
	var mu sync.Mutex

	semaphore := make(chan struct{}, numWorkers)

	for _, vol := range volatilities {
		for _, simFunc := range simulationFuncs {
			wg.Add(1)
			go func(volName, simName string, volatility float64, simFunc func(models.OptionSpread, float64, float64, float64, int, *rand.Rand, tradier.QuoteHistory) map[string]float64) {
				defer wg.Done()
				semaphore <- struct{}{}
				defer func() { <-semaphore }()

				rng := rngPool.Get().(*rand.Rand)
				defer rngPool.Put(rng)

				probMap := simFunc(spread, underlyingPrice, riskFreeRate, volatility, daysToExpiration, rng, history)

				mu.Lock()
				for key, value := range probMap {
					results[volName+"_"+simName+"_"+key] = value
				}
				mu.Unlock()
			}(vol.Name, simFunc.name, vol.Vol, simFunc.fn)
		}
	}

	wg.Wait()

	averageProbability := calculateAverageProbability(results)

	result := models.SpreadWithProbabilities{
		Spread: spread,
		Probability: models.ProbabilityResult{
			AverageProbability: averageProbability,
			Probabilities:      results,
		},
		MeetsRoR: true,
	}

	// Calculate and store Merton parameters
	historicalJumps := calculateHistoricalJumps(history)
	merton := models.NewMertonJumpDiffusion(riskFreeRate, shortLegVol, 1.0, 0, shortLegVol)
	merton.CalibrateJumpSizes(historicalJumps, 1)
	result.MertonParams.Lambda = merton.Lambda
	result.MertonParams.Mu = merton.Mu
	result.MertonParams.Delta = merton.Delta

	// Calculate and store Kou parameters
	historicalPrices := extractHistoricalPrices(history)
	kou := models.NewKouJumpDiffusion(riskFreeRate, shortLegVol, historicalPrices, 1.0/252.0)
	result.KouParams.Lambda = kou.Lambda
	result.KouParams.P = kou.P
	result.KouParams.Eta1 = kou.Eta1
	result.KouParams.Eta2 = kou.Eta2

	hestonModel, hestonVol := calculateHestonModel(spread, history, underlyingPrice, riskFreeRate, float64(daysToExpiration)/365.0)
	result.HestonParams = models.HestonParams{
		V0:    hestonModel.V0,
		Kappa: hestonModel.Kappa,
		Theta: hestonModel.Theta,
		Xi:    hestonModel.Xi,
		Rho:   hestonModel.Rho,
	}

	// Store volatility information
	result.VolatilityInfo = models.VolatilityInfo{
		ShortLegVol:        shortLegVol,
		LongLegVol:         longLegVol,
		YangZhang:          yangzhangVolatilities,
		RogersSatchel:      rogerssatchelVolatilities,
		TotalAvgVolSurface: volatilities[len(volatilities)-1].Vol,
		ShortLegImpliedVols: map[string]float64{
			"Bid": spread.ShortLeg.Option.Greeks.BidIv,
			"Ask": spread.ShortLeg.Option.Greeks.AskIv,
			"Mid": spread.ShortLeg.Option.Greeks.MidIv,
		},
		LongLegImpliedVols: map[string]float64{
			"Bid": spread.LongLeg.Option.Greeks.BidIv,
			"Ask": spread.LongLeg.Option.Greeks.AskIv,
			"Mid": spread.LongLeg.Option.Greeks.MidIv,
		},
		HestonVolatility: hestonVol,
	}

	return result
}

func simulateMertonJumpDiffusion(spread models.OptionSpread, underlyingPrice, riskFreeRate, volatility float64, daysToExpiration int, rng *rand.Rand, history tradier.QuoteHistory) map[string]float64 {
	tau := float64(daysToExpiration) / 365.0

	// Extract historical jumps from the QuoteHistory
	historicalJumps := calculateHistoricalJumps(history)

	// Estimate jump parameters
	lambda := 1.0 // Assuming on average 1 jump per year, adjust as needed

	// Create three Merton models with different jump size calibrations
	merton1x := models.NewMertonJumpDiffusion(riskFreeRate, volatility, lambda, 0, volatility)
	merton2x := models.NewMertonJumpDiffusion(riskFreeRate, volatility, lambda, 0, volatility)
	merton3x := models.NewMertonJumpDiffusion(riskFreeRate, volatility, lambda, 0, volatility)

	merton1x.CalibrateJumpSizes(historicalJumps, 1)
	merton2x.CalibrateJumpSizes(historicalJumps, 2)
	merton3x.CalibrateJumpSizes(historicalJumps, 3)

	profitCount1x := 0
	profitCount2x := 0
	profitCount3x := 0

	for i := 0; i < numSimulations; i++ {
		finalPrice1 := merton1x.SimulatePrice(underlyingPrice, tau, timeSteps, rngPool.Get().(*rand.Rand))
		finalPrice2 := merton2x.SimulatePrice(underlyingPrice, tau, timeSteps, rngPool.Get().(*rand.Rand))
		finalPrice3 := merton3x.SimulatePrice(underlyingPrice, tau, timeSteps, rngPool.Get().(*rand.Rand))

		if models.IsProfitable(spread, finalPrice1) {
			profitCount1x++
		}
		if models.IsProfitable(spread, finalPrice2) {
			profitCount2x++
		}
		if models.IsProfitable(spread, finalPrice3) {
			profitCount3x++
		}
	}

	return map[string]float64{
		"1x":  float64(profitCount1x) / float64(numSimulations),
		"2x":  float64(profitCount2x) / float64(numSimulations),
		"3x":  float64(profitCount3x) / float64(numSimulations),
		"avg": (float64(profitCount1x) + float64(profitCount2x) + float64(profitCount3x)) / (3 * float64(numSimulations)),
	}
}

func simulateKouJumpDiffusion(spread models.OptionSpread, underlyingPrice, riskFreeRate, volatility float64, daysToExpiration int, rng *rand.Rand, history tradier.QuoteHistory) map[string]float64 {
	tau := float64(daysToExpiration) / 365.0

	// Extract historical prices from the QuoteHistory
	historicalPrices := extractHistoricalPrices(history)

	// Calculate time step based on the historical data
	timeStep := 1.0 / 252.0 // Assuming daily data, adjust if necessary

	// Create three Kou models with different parameter calibrations
	kou1x := models.NewKouJumpDiffusion(riskFreeRate, volatility, historicalPrices, timeStep)
	kou2x := models.NewKouJumpDiffusion(riskFreeRate, volatility, scaleHistoricalPrices(historicalPrices, 2), timeStep)
	kou3x := models.NewKouJumpDiffusion(riskFreeRate, volatility, scaleHistoricalPrices(historicalPrices, 3), timeStep)

	// Simulate prices using the batch method
	prices1x := kou1x.SimulatePricesBatch(underlyingPrice, tau, timeSteps, numSimulations)
	prices2x := kou2x.SimulatePricesBatch(underlyingPrice, tau, timeSteps, numSimulations)
	prices3x := kou3x.SimulatePricesBatch(underlyingPrice, tau, timeSteps, numSimulations)

	profitCount1x := 0
	profitCount2x := 0
	profitCount3x := 0

	for i := 0; i < numSimulations; i++ {
		if models.IsProfitable(spread, prices1x[i]) {
			profitCount1x++
		}
		if models.IsProfitable(spread, prices2x[i]) {
			profitCount2x++
		}
		if models.IsProfitable(spread, prices3x[i]) {
			profitCount3x++
		}
	}

	return map[string]float64{
		"1x":  float64(profitCount1x) / float64(numSimulations),
		"2x":  float64(profitCount2x) / float64(numSimulations),
		"3x":  float64(profitCount3x) / float64(numSimulations),
		"avg": (float64(profitCount1x) + float64(profitCount2x) + float64(profitCount3x)) / (3 * float64(numSimulations)),
	}
}

func simulateHeston(spread models.OptionSpread, underlyingPrice, riskFreeRate, volatility float64, daysToExpiration int, rng *rand.Rand, history tradier.QuoteHistory) map[string]float64 {
	tau := float64(daysToExpiration) / 365.0

	// Use the global Heston model instead of creating a new one
	heston := globalHestonModel

	profitCount := 0

	for i := 0; i < numSimulations; i++ {
		finalPrice := heston.SimulatePrice(underlyingPrice, riskFreeRate, tau, timeSteps)

		if models.IsProfitable(spread, finalPrice) {
			profitCount++
		}
	}

	return map[string]float64{
		"probability": float64(profitCount) / float64(numSimulations),
	}
}

func calculateHestonModel(spread models.OptionSpread, history tradier.QuoteHistory, s0, r, t float64) (*models.HestonModel, float64) {
	// Use the global Heston model
	heston := globalHestonModel

	// Simulate prices using calibrated Heston model to estimate volatility
	numSimulations := 1000
	var sumSquaredReturns float64
	for i := 0; i < numSimulations; i++ {
		finalPrice := heston.SimulatePrice(s0, r, t, 252) // 252 trading days in a year
		logReturn := math.Log(finalPrice / s0)
		sumSquaredReturns += logReturn * logReturn
	}

	// Calculate annualized volatility
	hestonVol := math.Sqrt(sumSquaredReturns / float64(numSimulations) / t)
	return heston, hestonVol
}
