package models

import (
	"math"
	"runtime"
	"sync"
	"sync/atomic"

	"golang.org/x/exp/rand"
)

type MertonJumpDiffusion struct {
	R      float64 // Risk-free rate
	Sigma  float64 // Volatility
	Lambda float64 // Jump intensity
	Mu     float64 // Mean jump size
	Delta  float64 // Jump size volatility
}

func NewMertonJumpDiffusion(r, sigma, lambda, mu, delta float64) *MertonJumpDiffusion {
	return &MertonJumpDiffusion{
		R:      r,
		Sigma:  sigma,
		Lambda: lambda,
		Mu:     mu,
		Delta:  delta,
	}
}

func (m *MertonJumpDiffusion) SimulatePrice(s0, r, t float64, steps int, rng *rand.Rand) float64 {
	dt := t / float64(steps)
	price := s0

	for i := 0; i < steps; i++ {
		z := rng.NormFloat64()
		diffusion := math.Exp((r-0.5*m.Sigma*m.Sigma)*dt + m.Sigma*math.Sqrt(dt)*z)

		if rng.Float64() < m.Lambda*dt {
			y := rng.NormFloat64()
			jump := math.Exp(m.Mu + m.Delta*y)
			price *= diffusion * jump
		} else {
			price *= diffusion
		}
	}

	return price
}

func (m *MertonJumpDiffusion) OptionPrice(s0, k, r, t float64, isCall bool) float64 {
	numSimulations := 1000
	numWorkers := runtime.GOMAXPROCS(0)
	simulationsPerWorker := numSimulations / numWorkers

	var wg sync.WaitGroup
	var totalPayoff uint64

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			localRng := rand.New(rand.NewSource(uint64(rand.Int63())))
			localPayoff := float64(0)

			for j := 0; j < simulationsPerWorker; j++ {
				sT := m.SimulatePrice(s0, r, t, 252, localRng) // 252 trading days in a year
				var payoff float64
				if isCall {
					payoff = math.Max(sT-k, 0)
				} else {
					payoff = math.Max(k-sT, 0)
				}
				localPayoff += payoff
			}

			atomic.AddUint64(&totalPayoff, math.Float64bits(localPayoff))
		}()
	}

	wg.Wait()

	price := math.Float64frombits(atomic.LoadUint64(&totalPayoff))
	price /= float64(numSimulations)
	price *= math.Exp(-r * t)

	return price
}

func (m *MertonJumpDiffusion) CalibrateJumpSizes(historicalJumps []float64, scaleFactor float64) {
	var sumJumps, sumSquaredJumps float64
	n := float64(len(historicalJumps))

	var wg sync.WaitGroup
	var mu sync.Mutex
	numWorkers := runtime.GOMAXPROCS(0)
	jumpsPerWorker := len(historicalJumps) / numWorkers

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(start int) {
			defer wg.Done()
			localSumJumps, localSumSquaredJumps := 0.0, 0.0

			end := start + jumpsPerWorker
			if end > len(historicalJumps) {
				end = len(historicalJumps)
			}

			for _, jump := range historicalJumps[start:end] {
				scaledJump := jump * scaleFactor
				localSumJumps += scaledJump
				localSumSquaredJumps += scaledJump * scaledJump
			}

			mu.Lock()
			sumJumps += localSumJumps
			sumSquaredJumps += localSumSquaredJumps
			mu.Unlock()
		}(i * jumpsPerWorker)
	}

	wg.Wait()

	m.Mu = sumJumps / n
	m.Delta = math.Sqrt(sumSquaredJumps/n - m.Mu*m.Mu)
}
