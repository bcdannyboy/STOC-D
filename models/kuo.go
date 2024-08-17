package models

import (
	"math"
	"runtime"
	"sync"

	"golang.org/x/exp/rand"
)

// KouJumpDiffusion represents the Kou jump diffusion model
type KouJumpDiffusion struct {
	R      float64 // Risk-free rate
	Sigma  float64 // Volatility
	Lambda float64 // Jump intensity
	P      float64 // Probability of upward jump
	Eta1   float64 // Rate of upward jump
	Eta2   float64 // Rate of downward jump
}

var krngPool = sync.Pool{
	New: func() interface{} {
		return rand.New(rand.NewSource(uint64(rand.Int63())))
	},
}

// NewKouJumpDiffusion creates a new Kou jump diffusion model
func NewKouJumpDiffusion(r, sigma float64, historicalPrices []float64, timeStep float64) *KouJumpDiffusion {
	lambda, p := estimateLambdaAndP(historicalPrices, timeStep)
	eta1, eta2 := estimateEta1AndEta2(historicalPrices)

	return &KouJumpDiffusion{
		R:      r,
		Sigma:  sigma,
		Lambda: lambda,
		P:      p,
		Eta1:   eta1,
		Eta2:   eta2,
	}
}

// estimateLambdaAndP calculates lambda and p from historical prices
func estimateLambdaAndP(prices []float64, timeStep float64) (float64, float64) {
	returns := calculateReturns(prices)
	jumps := identifyJumps(returns)

	lambda := float64(len(jumps)) / (float64(len(prices)-1) * timeStep)

	upJumps := 0
	for _, jump := range jumps {
		if jump > 0 {
			upJumps++
		}
	}
	p := float64(upJumps) / float64(len(jumps))

	return lambda, p
}

// estimateEta1AndEta2 calculates eta1 and eta2 from historical prices
func estimateEta1AndEta2(prices []float64) (float64, float64) {
	returns := calculateReturns(prices)
	jumps := identifyJumps(returns)

	var upJumps, downJumps []float64
	for _, jump := range jumps {
		if jump > 0 {
			upJumps = append(upJumps, jump)
		} else {
			downJumps = append(downJumps, -jump)
		}
	}

	eta1 := 1.0 / calculateMean(upJumps)
	eta2 := 1.0 / calculateMean(downJumps)

	return eta1, eta2
}

// calculateReturns computes log returns from prices
func calculateReturns(prices []float64) []float64 {
	returns := make([]float64, len(prices)-1)
	for i := 1; i < len(prices); i++ {
		returns[i-1] = math.Log(prices[i] / prices[i-1])
	}
	return returns
}

// identifyJumps detects jumps in returns using a threshold method
func identifyJumps(returns []float64) []float64 {
	mean := calculateMean(returns)
	std := calculateStdDeviation(returns, mean)
	threshold := 3 * std // Use 3 standard deviations as the threshold

	var jumps []float64
	for _, r := range returns {
		if math.Abs(r-mean) > threshold {
			jumps = append(jumps, r)
		}
	}
	return jumps
}

// calculateMean computes the mean of a slice of float64
func calculateMean(values []float64) float64 {
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// calculateStdDeviation computes the standard deviation
func calculateStdDeviation(values []float64, mean float64) float64 {
	sum := 0.0
	for _, v := range values {
		sum += (v - mean) * (v - mean)
	}
	return math.Sqrt(sum / float64(len(values)))
}

// SimulatePrice simulates the price path using the Kou jump diffusion model
func (k *KouJumpDiffusion) SimulatePrice(s0, r, t float64, steps int, rng *rand.Rand) float64 {
	dt := t / float64(steps)
	price := s0

	for i := 0; i < steps; i++ {
		z := rng.NormFloat64()
		diffusion := math.Exp((r-0.5*k.Sigma*k.Sigma)*dt + k.Sigma*math.Sqrt(dt)*z)

		if rng.Float64() < k.Lambda*dt {
			var jump float64
			if rng.Float64() < k.P {
				jump = math.Exp(rng.ExpFloat64() / k.Eta1)
			} else {
				jump = math.Exp(-rng.ExpFloat64() / k.Eta2)
			}
			price *= diffusion * jump
		} else {
			price *= diffusion
		}
	}

	return price
}

// SimulatePricesBatch simulates multiple price paths in parallel
func (k *KouJumpDiffusion) SimulatePricesBatch(s0, r, t float64, steps, numSimulations int) []float64 {
	results := make([]float64, numSimulations)
	var wg sync.WaitGroup
	numWorkers := runtime.GOMAXPROCS(0)
	simulationsPerWorker := numSimulations / numWorkers

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(start int) {
			defer wg.Done()
			rng := krngPool.Get().(*rand.Rand)
			defer krngPool.Put(rng)
			for j := start; j < start+simulationsPerWorker; j++ {
				results[j] = k.SimulatePrice(s0, r, t, steps, rng)
			}
		}(i * simulationsPerWorker)
	}

	wg.Wait()
	return results
}

// OptionPrice calculates the option price using Monte Carlo simulation
func (k *KouJumpDiffusion) OptionPrice(s0, strike, r, t float64, isCall bool, numSimulations int) float64 {
	simulatedPrices := k.SimulatePricesBatch(s0, r, t, 252, numSimulations)

	var totalPayoff float64
	var wg sync.WaitGroup
	var mu sync.Mutex

	numWorkers := runtime.GOMAXPROCS(0)
	simulationsPerWorker := numSimulations / numWorkers

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(start int) {
			defer wg.Done()
			localPayoff := 0.0

			for j := start; j < start+simulationsPerWorker; j++ {
				sT := simulatedPrices[j]
				var payoff float64
				if isCall {
					payoff = math.Max(sT-strike, 0)
				} else {
					payoff = math.Max(strike-sT, 0)
				}
				localPayoff += payoff
			}

			mu.Lock()
			totalPayoff += localPayoff
			mu.Unlock()
		}(i * simulationsPerWorker)
	}

	wg.Wait()

	price := totalPayoff / float64(numSimulations)
	price *= math.Exp(-r * t)

	return price
}
