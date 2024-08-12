package models

import (
	"math"
	"runtime"
	"sync"

	"golang.org/x/exp/rand"
)

type HestonModel struct {
	V0    float64 // Initial variance
	Kappa float64 // Mean reversion speed of variance
	Theta float64 // Long-term variance
	Xi    float64 // Volatility of variance
	Rho   float64 // Correlation between asset returns and variance
}

var rngPool = sync.Pool{
	New: func() interface{} {
		return rand.New(rand.NewSource(uint64(rand.Int63())))
	},
}

func NewHestonModel(v0, kappa, theta, xi, rho float64) *HestonModel {
	return &HestonModel{
		V0:    v0,
		Kappa: kappa,
		Theta: theta,
		Xi:    xi,
		Rho:   rho,
	}
}

func (h *HestonModel) SimulatePrice(s0, r, t float64, steps int) float64 {
	dt := t / float64(steps)
	sqrtDt := math.Sqrt(dt)

	s := s0
	v := h.V0

	rng := rngPool.Get().(*rand.Rand)
	defer rngPool.Put(rng)

	for i := 0; i < steps; i++ {
		z1 := rng.NormFloat64()
		z2 := rng.NormFloat64()
		z2 = h.Rho*z1 + math.Sqrt(1-h.Rho*h.Rho)*z2

		s *= math.Exp((r-0.5*v)*dt + math.Sqrt(v)*sqrtDt*z1)
		v += h.Kappa*(h.Theta-v)*dt + h.Xi*math.Sqrt(v)*sqrtDt*z2
		v = math.Max(0, v) // Ensure variance stays non-negative
	}

	return s
}

func (h *HestonModel) SimulatePricesBatch(s0, r, t float64, steps, numSimulations int) []float64 {
	results := make([]float64, numSimulations)
	var wg sync.WaitGroup
	numWorkers := runtime.GOMAXPROCS(0)
	simulationsPerWorker := numSimulations / numWorkers

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(start int) {
			defer wg.Done()
			for j := start; j < start+simulationsPerWorker; j++ {
				results[j] = h.SimulatePrice(s0, r, t, steps)
			}
		}(i * simulationsPerWorker)
	}

	wg.Wait()
	return results
}

func (h *HestonModel) Calibrate(marketPrices []float64, strikes []float64, s0, r, t float64) error {
	// Implement calibration logic here
	// This could involve minimizing the difference between model prices and market prices
	// You might use an optimization algorithm like Levenberg-Marquardt or Nelder-Mead

	// For now, we'll use placeholder values
	h.V0 = 0.04
	h.Kappa = 2
	h.Theta = 0.04
	h.Xi = 0.4
	h.Rho = -0.5

	return nil
}
