package probability

import (
	"math"
	"sync"
	"time"

	"github.com/bcdannyboy/dquant/models"
	"golang.org/x/exp/rand"
	"gonum.org/v1/gonum/stat/distuv"
)

const (
	numSimulations = 1000
	timeSteps      = 252 // Assuming 252 trading days in a year
)

var (
	globalRNG = rand.New(rand.NewSource(uint64(time.Now().UnixNano())))
	rngPool   = sync.Pool{
		New: func() interface{} {
			return rand.New(rand.NewSource(globalRNG.Uint64()))
		},
	}
)

func MonteCarloSimulationBatch(spreads []models.OptionSpread, underlyingPrice, riskFreeRate float64, daysToExpiration int) []models.ProbabilityResult {
	results := make([]models.ProbabilityResult, len(spreads))
	var wg sync.WaitGroup
	wg.Add(len(spreads))

	for i := range spreads {
		go func(i int) {
			defer wg.Done()
			results[i] = monteCarloSimulation(spreads[i], underlyingPrice, riskFreeRate, daysToExpiration)
		}(i)
	}

	wg.Wait()
	return results
}

func MonteCarloSimulation(spread models.OptionSpread, underlyingPrice, riskFreeRate float64, daysToExpiration int) models.ProbabilityResult {
	return monteCarloSimulation(spread, underlyingPrice, riskFreeRate, daysToExpiration)
}

func monteCarloSimulation(spread models.OptionSpread, underlyingPrice, riskFreeRate float64, daysToExpiration int) models.ProbabilityResult {
	volatilities := []struct {
		name string
		vol  float64
	}{
		{"BidIV", spread.ImpliedVol.BidIV},
		{"AskIV", spread.ImpliedVol.AskIV},
		{"MidIV", spread.ImpliedVol.MidIV},
		{"GARCHIV", spread.ImpliedVol.GARCHIV},
		{"BSMIV", spread.ImpliedVol.BSMIV},
		{"GarmanKlassIV", spread.ImpliedVol.GarmanKlassIV},
		{"ParkinsonVolatility", spread.ImpliedVol.ParkinsonVolatility},
	}

	results := make(map[string]float64, len(volatilities)*4)
	var wg sync.WaitGroup
	resultsChan := make(chan struct {
		key   string
		value float64
	}, len(volatilities)*4)

	simulationFuncs := []struct {
		name string
		fn   func(models.OptionSpread, float64, float64, float64, int) float64
	}{
		{"Normal", simulateNormal},
		{"StudentT", simulateStudentT},
		{"GBM", simulateGBM},
		{"PoissonJump", simulatePoissonJump},
	}

	for _, vol := range volatilities {
		for _, simFunc := range simulationFuncs {
			wg.Add(1)
			go func(volName, simName string, volatility float64, simFunc func(models.OptionSpread, float64, float64, float64, int) float64) {
				defer wg.Done()
				prob := simFunc(spread, underlyingPrice, riskFreeRate, volatility, daysToExpiration)
				resultsChan <- struct {
					key   string
					value float64
				}{volName + "_" + simName, prob}
			}(vol.name, simFunc.name, vol.vol, simFunc.fn)
		}
	}

	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	for result := range resultsChan {
		results[result.key] = result.value
	}

	var totalProb float64
	for _, prob := range results {
		totalProb += prob
	}
	averageProbability := totalProb / float64(len(results))

	return models.ProbabilityResult{
		Probabilities:      results,
		AverageProbability: averageProbability,
	}
}

func simulateNormal(spread models.OptionSpread, underlyingPrice, riskFreeRate, volatility float64, daysToExpiration int) float64 {
	return simulateWithDistribution(spread, underlyingPrice, riskFreeRate, daysToExpiration, volatility, normalRand)
}

func simulateStudentT(spread models.OptionSpread, underlyingPrice, riskFreeRate, volatility float64, daysToExpiration int) float64 {
	studentT := distuv.StudentsT{Nu: 5, Mu: 0, Sigma: 1}
	return simulateWithDistribution(spread, underlyingPrice, riskFreeRate, daysToExpiration, volatility, studentT.Rand)
}

func simulateGBM(spread models.OptionSpread, underlyingPrice, riskFreeRate, volatility float64, daysToExpiration int) float64 {
	dt := float64(daysToExpiration) / 252.0 / float64(timeSteps)
	sqrtDt := math.Sqrt(dt)

	rng := rngPool.Get().(*rand.Rand)
	defer rngPool.Put(rng)

	profitCount := 0
	for i := 0; i < numSimulations; i++ {
		price := underlyingPrice
		for j := 0; j < timeSteps; j++ {
			price *= math.Exp((riskFreeRate-0.5*volatility*volatility)*dt +
				volatility*sqrtDt*rng.NormFloat64())
		}

		if models.IsProfitable(spread, price) {
			profitCount++
		}
	}

	return float64(profitCount) / float64(numSimulations)
}

func simulatePoissonJump(spread models.OptionSpread, underlyingPrice, riskFreeRate, volatility float64, daysToExpiration int) float64 {
	dt := float64(daysToExpiration) / 252.0 / float64(timeSteps)
	sqrtDt := math.Sqrt(dt)
	lambda := 1.0 // Average number of jumps per year
	jumpMean := 0.0
	jumpStdDev := 0.1

	rng := rngPool.Get().(*rand.Rand)
	defer rngPool.Put(rng)

	poisson := distuv.Poisson{Lambda: lambda * dt}

	profitCount := 0
	for i := 0; i < numSimulations; i++ {
		price := underlyingPrice
		for j := 0; j < timeSteps; j++ {
			// Diffusion component
			price *= math.Exp((riskFreeRate-0.5*volatility*volatility)*dt +
				volatility*sqrtDt*rng.NormFloat64())

			// Jump component
			numJumps := poisson.Rand()
			for k := 0; k < int(numJumps); k++ {
				jumpSize := math.Exp(jumpMean+jumpStdDev*rng.NormFloat64()) - 1
				price *= (1 + jumpSize)
			}
		}

		if models.IsProfitable(spread, price) {
			profitCount++
		}
	}

	return float64(profitCount) / float64(numSimulations)
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

func normalRand() float64 {
	rng := rngPool.Get().(*rand.Rand)
	defer rngPool.Put(rng)
	return rng.NormFloat64()
}
