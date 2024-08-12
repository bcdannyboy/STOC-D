package probability

import (
	"sync"
	"time"

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
		return rand.New(rand.NewSource(uint64(time.Now().UnixNano())))
	},
}

func MonteCarloSimulation(spread models.OptionSpread, underlyingPrice, riskFreeRate float64, daysToExpiration int, gkVolatilities, parkinsonVolatilities map[string]float64, localVolSurface models.VolatilitySurface, history tradier.QuoteHistory) models.ProbabilityResult {
	shortLegVol, longLegVol := confirmVolatilities(spread, localVolSurface, daysToExpiration, gkVolatilities, parkinsonVolatilities)

	volatilities := calculateVolatilities(shortLegVol, longLegVol, daysToExpiration, gkVolatilities, parkinsonVolatilities, localVolSurface, history, spread)

	simulationFuncs := []struct {
		name string
		fn   func(models.OptionSpread, float64, float64, float64, int, *rand.Rand, []float64) map[string]float64
	}{
		{name: "MertonJD", fn: simulateMertonJumpDiffusion},
	}

	results := make(map[string]float64, len(volatilities)*len(simulationFuncs)*5)
	var wg sync.WaitGroup
	var mu sync.Mutex

	semaphore := make(chan struct{}, numWorkers)

	historicalJumps := calculateHistoricalJumps(history)

	for _, vol := range volatilities {
		for _, simFunc := range simulationFuncs {
			wg.Add(1)
			go func(volName, simName string, volatility float64, simFunc func(models.OptionSpread, float64, float64, float64, int, *rand.Rand, []float64) map[string]float64) {
				defer wg.Done()
				semaphore <- struct{}{}
				defer func() { <-semaphore }()

				rng := rngPool.Get().(*rand.Rand)
				defer rngPool.Put(rng)

				probMap := simFunc(spread, underlyingPrice, riskFreeRate, volatility, daysToExpiration, rng, historicalJumps)

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
	return models.ProbabilityResult{
		AverageProbability: averageProbability,
		Probabilities:      results,
	}
}

func simulateMertonJumpDiffusion(spread models.OptionSpread, underlyingPrice, riskFreeRate, volatility float64, daysToExpiration int, rng *rand.Rand, historicalJumps []float64) map[string]float64 {
	tau := float64(daysToExpiration) / 365.0

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
		finalPrice1 := merton1x.SimulatePrice(underlyingPrice, tau, timeSteps, rngPool.New().(*rand.Rand))
		finalPrice2 := merton2x.SimulatePrice(underlyingPrice, tau, timeSteps, rngPool.New().(*rand.Rand))
		finalPrice3 := merton3x.SimulatePrice(underlyingPrice, tau, timeSteps, rngPool.New().(*rand.Rand))

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
