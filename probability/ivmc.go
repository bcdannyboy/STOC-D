package probability

import (
	"math"
	"sync"
	"time"

	"github.com/bcdannyboy/dquant/models"
	"github.com/bcdannyboy/dquant/tradier"
	"golang.org/x/exp/rand"
	"gonum.org/v1/gonum/stat"
	"gonum.org/v1/gonum/stat/distuv"
)

const (
	numSimulations = 1000
	timeSteps      = 252 // Assuming 252 trading days in a year
)

func MonteCarloSimulation(spread models.OptionSpread, underlyingPrice, riskFreeRate float64, daysToExpiration int, history tradier.QuoteHistory) models.ProbabilityResult {
	combinedIV := calculateCombinedIV(spread)
	gkVolatilities := models.CalculateGarmanKlassVolatilities(history)
	parkinsonVolatilities := models.CalculateParkinsonsVolatilities(history)

	volatilities := []struct {
		name string
		vol  float64
	}{
		{name: "CombinedIV", vol: combinedIV},
	}

	for period, vol := range gkVolatilities {
		volatilities = append(volatilities, struct {
			name string
			vol  float64
		}{name: "GarmanKlassIV_" + period, vol: vol})
	}

	for period, vol := range parkinsonVolatilities {
		volatilities = append(volatilities, struct {
			name string
			vol  float64
		}{name: "ParkinsonVolatility_" + period, vol: vol})
	}

	simulationFuncs := []struct {
		name string
		fn   func(models.OptionSpread, float64, float64, float64, int, *rand.Rand) float64
	}{
		{name: "Normal", fn: simulateNormal},
		{name: "StudentT", fn: simulateStudentT},
		{name: "GBM", fn: simulateGBM},
		{name: "PoissonJump", fn: simulatePoissonJump},
	}

	results := make(map[string]float64)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, vol := range volatilities {
		for _, simFunc := range simulationFuncs {
			for factor := 1.0; factor <= 3.0; factor++ {
				wg.Add(1)
				go func(volName, simName string, volatility float64, factor float64, simFunc func(models.OptionSpread, float64, float64, float64, int, *rand.Rand) float64) {
					defer wg.Done()
					rng := rand.New(rand.NewSource(uint64(time.Now().UnixNano())))
					prob := simFunc(spread, underlyingPrice, riskFreeRate, volatility*factor, daysToExpiration, rng)
					mu.Lock()
					results[volName+"_"+simName+"_Factor"+string(int(factor))] = prob
					mu.Unlock()
				}(vol.name, simFunc.name, vol.vol, factor, simFunc.fn)
			}
		}
	}

	wg.Wait()

	combinedResults := combineSimulationResults(results)
	averageProbability := calculateAverageProbability(combinedResults)
	return models.ProbabilityResult{
		AverageProbability: averageProbability,
		Probabilities:      combinedResults,
	}
}

func calculateIVMeanAndStd(volatilities []struct {
	name string
	vol  float64
}) (float64, float64) {
	var ivs []float64
	for _, vol := range volatilities {
		ivs = append(ivs, vol.vol)
	}
	mean, std := stat.MeanStdDev(ivs, nil)
	return mean, std
}

func simulateGBM(spread models.OptionSpread, underlyingPrice, riskFreeRate, volatility float64, daysToExpiration int, rng *rand.Rand) float64 {
	dt := float64(daysToExpiration) / 252.0 / float64(timeSteps)
	sqrtDt := math.Sqrt(dt)

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

func simulateNormal(spread models.OptionSpread, underlyingPrice, riskFreeRate, volatility float64, daysToExpiration int, rng *rand.Rand) float64 {
	return simulateWithDistribution(spread, underlyingPrice, riskFreeRate, daysToExpiration, volatility, rng.NormFloat64)
}

func simulateStudentT(spread models.OptionSpread, underlyingPrice, riskFreeRate, volatility float64, daysToExpiration int, rng *rand.Rand) float64 {
	studentT := distuv.StudentsT{Nu: 5, Mu: 0, Sigma: 1, Src: rng}
	return simulateWithDistribution(spread, underlyingPrice, riskFreeRate, daysToExpiration, volatility, studentT.Rand)
}

func simulatePoissonJump(spread models.OptionSpread, underlyingPrice, riskFreeRate, volatility float64, daysToExpiration int, rng *rand.Rand) float64 {
	dt := float64(daysToExpiration) / 252.0 / float64(timeSteps)
	sqrtDt := math.Sqrt(dt)

	// Parameters for the jump process
	avgJumpsPerYear := 5.0 + rng.Float64()*10.0   // Average jumps per year: 5 to 15
	jumpMeanAnnual := -0.05 + rng.Float64()*0.1   // Annual jump mean: -5% to +5%
	jumpStdDevAnnual := 0.05 + rng.Float64()*0.15 // Annual jump std dev: 5% to 20%

	// Scale jump parameters to match the simulation timeframe
	jumpMean := jumpMeanAnnual * dt
	jumpStdDev := jumpStdDevAnnual * math.Sqrt(dt)

	poisson := distuv.Poisson{Lambda: avgJumpsPerYear * dt, Src: rng}

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

func calculateCombinedIV(spread models.OptionSpread) float64 {
	shortVega := spread.ShortLeg.BSMResult.Vega
	longVega := spread.LongLeg.BSMResult.Vega
	shortIV := spread.ShortLeg.BSMResult.ImpliedVolatility
	longIV := spread.LongLeg.BSMResult.ImpliedVolatility

	combinedIV := (shortVega*shortIV + longVega*longIV) / (shortVega + longVega)

	// If combined IV is negative or zero, use the short leg's ask IV
	if combinedIV <= 0 {
		return spread.ShortLeg.Option.Greeks.AskIv
	}

	return combinedIV
}

func combineSimulationResults(results map[string]float64) map[string]float64 {
	combinedResults := make(map[string]float64)
	for key, value := range results {
		combinedResults[key] = value
	}
	return combinedResults
}

func calculateAverageProbability(results map[string]float64) float64 {
	var sum float64
	for _, value := range results {
		sum += value
	}
	return sum / float64(len(results))
}
