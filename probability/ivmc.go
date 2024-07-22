package probability

import (
	"math"
	"math/rand"
	"sync"

	"github.com/bcdannyboy/dquant/models"
	"gonum.org/v1/gonum/stat/distuv"
)

const (
	numSimulations = 2000
	timeSteps      = 252 // Assuming 252 trading days in a year
)

func MonteCarloSimulationBatch(spreads []models.OptionSpread, underlyingPrice, riskFreeRate float64, daysToExpiration int) []models.SpreadWithProbabilities {
	results := make([]models.SpreadWithProbabilities, len(spreads))
	var wg sync.WaitGroup
	resultChan := make(chan models.SpreadWithProbabilities, len(spreads))

	for i := range spreads {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			probabilities := monteCarloSimulation(spreads[i], underlyingPrice, riskFreeRate, daysToExpiration)
			resultChan <- models.SpreadWithProbabilities{Spread: spreads[i], Probabilities: probabilities}
		}(i)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	for result := range resultChan {
		results = append(results, result)
	}

	return results
}

func monteCarloSimulation(spread models.OptionSpread, underlyingPrice, riskFreeRate float64, daysToExpiration int) map[string]float64 {
	results := make(map[string]float64)
	var wg sync.WaitGroup
	var mu sync.Mutex

	simulationFuncs := []struct {
		name string
		fn   func(models.OptionSpread, float64, float64, int) float64
	}{
		{"Normal", simulateNormal},
		{"StudentT", simulateStudentT},
		{"GBM", simulateGBM},
		{"PoissonJump", simulatePoissonJump},
	}

	for _, sim := range simulationFuncs {
		wg.Add(1)
		go func(name string, simFunc func(models.OptionSpread, float64, float64, int) float64) {
			defer wg.Done()
			probability := simFunc(spread, underlyingPrice, riskFreeRate, daysToExpiration)
			mu.Lock()
			results[name] = probability
			mu.Unlock()
		}(sim.name, sim.fn)
	}

	wg.Wait()
	return results
}

func simulateNormal(spread models.OptionSpread, underlyingPrice, riskFreeRate float64, daysToExpiration int) float64 {
	volatility := models.CalculateAverageVolatility(spread)
	return simulateWithDistribution(spread, underlyingPrice, riskFreeRate, daysToExpiration, volatility, rand.NormFloat64)
}

func simulateStudentT(spread models.OptionSpread, underlyingPrice, riskFreeRate float64, daysToExpiration int) float64 {
	volatility := models.CalculateAverageVolatility(spread)
	studentT := distuv.StudentsT{Nu: 5, Mu: 0, Sigma: 1}
	return simulateWithDistribution(spread, underlyingPrice, riskFreeRate, daysToExpiration, volatility, studentT.Rand)
}

func simulateGBM(spread models.OptionSpread, underlyingPrice, riskFreeRate float64, daysToExpiration int) float64 {
	volatility := models.CalculateAverageVolatility(spread)
	dt := float64(daysToExpiration) / 252.0 / float64(timeSteps)

	profitCount := 0
	for i := 0; i < numSimulations; i++ {
		price := underlyingPrice
		for j := 0; j < timeSteps; j++ {
			price *= math.Exp((riskFreeRate-0.5*volatility*volatility)*dt +
				volatility*math.Sqrt(dt)*rand.NormFloat64())
		}

		if models.IsProfitable(spread, price) {
			profitCount++
		}
	}

	return float64(profitCount) / float64(numSimulations)
}

func simulatePoissonJump(spread models.OptionSpread, underlyingPrice, riskFreeRate float64, daysToExpiration int) float64 {
	volatility := models.CalculateAverageVolatility(spread)
	dt := float64(daysToExpiration) / 252.0 / float64(timeSteps)
	lambda := 1.0 // Average number of jumps per year
	jumpMean := 0.0
	jumpStdDev := 0.1

	poisson := distuv.Poisson{Lambda: lambda * dt}

	profitCount := 0
	for i := 0; i < numSimulations; i++ {
		price := underlyingPrice
		for j := 0; j < timeSteps; j++ {
			// Diffusion component
			price *= math.Exp((riskFreeRate-0.5*volatility*volatility)*dt +
				volatility*math.Sqrt(dt)*rand.NormFloat64())

			// Jump component
			numJumps := poisson.Rand()
			for k := 0; k < int(numJumps); k++ {
				jumpSize := math.Exp(jumpMean+jumpStdDev*rand.NormFloat64()) - 1
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
	profitCount := 0
	for i := 0; i < numSimulations; i++ {
		finalPrice := underlyingPrice * math.Exp((riskFreeRate-0.5*volatility*volatility)*float64(daysToExpiration)/252.0+
			volatility*math.Sqrt(float64(daysToExpiration)/252.0)*randFunc())

		if models.IsProfitable(spread, finalPrice) {
			profitCount++
		}
	}

	return float64(profitCount) / float64(numSimulations)
}
