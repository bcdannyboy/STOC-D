package models

import (
	"math"
	"runtime"
	"sync"

	"golang.org/x/exp/rand"
	"gonum.org/v1/gonum/optimize"
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

type HestonCalibrationProblem struct {
	MarketPrices []float64
	Strikes      []float64
	S0           float64
	R            float64
	T            float64
}

func (p *HestonCalibrationProblem) Evaluate(ind *HestonParams) (float64, error) {
	return p.objectiveFunction([]float64{ind.V0, ind.Kappa, ind.Theta, ind.Xi, ind.Rho}), nil
}

func (p *HestonCalibrationProblem) objectiveFunction(x []float64) float64 {
	model := NewHestonModel(x[0], x[1], x[2], x[3], x[4])
	mse := 0.0

	for i, strike := range p.Strikes {
		modelPrice := model.CalculateOptionPrice(p.S0, strike, p.R, p.T)
		mse += math.Pow(modelPrice-p.MarketPrices[i], 2)
	}

	return mse / float64(len(p.Strikes))
}

// CalculateOptionPrice calculates the option price using the Heston model
func (h *HestonModel) CalculateOptionPrice(s0, k, r, t float64) float64 {
	// Implement the Heston option pricing formula here
	// You can use numerical integration or an approximation method
	// For simplicity, we'll use a Monte Carlo simulation here
	numSimulations := 1000
	prices := h.SimulatePricesBatch(s0, r, t, 252, numSimulations)

	sum := 0.0
	for _, price := range prices {
		sum += math.Max(price-k, 0)
	}

	return math.Exp(-r*t) * sum / float64(numSimulations)
}

func (h *HestonModel) Calibrate(marketPrices, strikes []float64, s0, r, t float64) error {
	problem := optimize.Problem{
		Func: func(x []float64) float64 {
			h.V0 = x[0]
			h.Kappa = x[1]
			h.Theta = x[2]
			h.Xi = x[3]
			h.Rho = x[4]
			return h.objectiveFunction(marketPrices, strikes, s0, r, t)
		},
	}

	result, err := optimize.Minimize(problem, []float64{h.V0, h.Kappa, h.Theta, h.Xi, h.Rho}, nil, &optimize.NelderMead{})
	if err != nil {
		return err
	}

	h.V0 = result.X[0]
	h.Kappa = result.X[1]
	h.Theta = result.X[2]
	h.Xi = result.X[3]
	h.Rho = result.X[4]

	return nil
}

func (h *HestonModel) objectiveFunction(marketPrices, strikes []float64, s0, r, t float64) float64 {
	mse := 0.0
	for i, strike := range strikes {
		modelPrice := h.CalculateOptionPrice(s0, strike, r, t)
		mse += math.Pow(modelPrice-marketPrices[i], 2)
	}
	return mse / float64(len(strikes))
}
