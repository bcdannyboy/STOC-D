package probability

import (
	"math"
	"sync"
	"time"

	"github.com/bcdannyboy/dquant/models"
	"github.com/bcdannyboy/dquant/tradier"
	"golang.org/x/exp/rand"
	"gonum.org/v1/gonum/stat/distuv"
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

	// Append volatilities calculated from the legs
	volatilities := calculateVolatilities(shortLegVol, longLegVol, daysToExpiration, gkVolatilities, parkinsonVolatilities, localVolSurface, history)

	simulationFuncs := []struct {
		name string
		fn   func(models.OptionSpread, float64, float64, float64, int, *rand.Rand) float64
	}{
		{name: "StudentT", fn: simulateStudentT},
	}

	results := make(map[string]float64, len(volatilities)*len(simulationFuncs)*3)
	var wg sync.WaitGroup
	var mu sync.Mutex

	semaphore := make(chan struct{}, numWorkers)

	for _, vol := range volatilities {
		for _, simFunc := range simulationFuncs {
			wg.Add(1)
			go func(volName, simName string, volatility float64, simFunc func(models.OptionSpread, float64, float64, float64, int, *rand.Rand) float64) {
				defer wg.Done()
				semaphore <- struct{}{}
				defer func() { <-semaphore }()

				rng := rngPool.Get().(*rand.Rand)
				defer rngPool.Put(rng)

				prob := simFunc(spread, underlyingPrice, riskFreeRate, volatility, daysToExpiration, rng)

				mu.Lock()
				results[volName+"_"+simName] = prob
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

func simulateStudentT(spread models.OptionSpread, underlyingPrice, riskFreeRate, volatility float64, daysToExpiration int, rng *rand.Rand) float64 {
	studentT := distuv.StudentsT{Nu: 5, Mu: 0, Sigma: 1, Src: rng}
	return simulateWithDistribution(spread, underlyingPrice, riskFreeRate, daysToExpiration, volatility, studentT.Rand)
}

func simulateWithDistribution(spread models.OptionSpread, underlyingPrice, riskFreeRate float64, daysToExpiration int, volatility float64, randFunc func() float64) float64 {
	sqrtT := math.Sqrt(float64(daysToExpiration) / 252.0)
	expTerm := math.Exp((riskFreeRate - 0.5*volatility*volatility) * float64(daysToExpiration) / 252.0)

	profitCount := 0
	for i := 0; i < numSimulations; i++ {
		finalPrice := underlyingPrice * expTerm * math.Exp(volatility*sqrtT*randFunc())
		if models.IsProfitable(spread, finalPrice) {
			profitCount++
		}
	}

	return float64(profitCount) / float64(numSimulations)
}
