package models

import (
	"math"
	"sync"

	"golang.org/x/exp/rand"
	"gonum.org/v1/gonum/stat/distuv"
)

type HestonParameters struct {
	V0    float64 // Initial variance
	Kappa float64 // Mean reversion speed of variance
	Theta float64 // Long-term variance
	Xi    float64 // Volatility of variance
	Rho   float64 // Correlation between asset returns and variance
}

func SimulateHestonPaths(S0, r float64, params HestonParameters, T float64, steps, numPaths int) [][]float64 {
	paths := make([][]float64, numPaths)
	var wg sync.WaitGroup
	wg.Add(numPaths)

	for i := 0; i < numPaths; i++ {
		go func(pathIndex int) {
			defer wg.Done()
			paths[pathIndex] = simulateHestonPath(S0, r, params, T, steps)
		}(i)
	}

	wg.Wait()
	return paths
}

func simulateHestonPath(S0, r float64, params HestonParameters, T float64, steps int) []float64 {
	dt := T / float64(steps)
	sqrtDt := math.Sqrt(dt)

	S := make([]float64, steps+1)
	v := make([]float64, steps+1)

	S[0] = S0
	v[0] = params.V0

	// Create a new source for each goroutine to avoid contention
	source := rand.NewSource(uint64(rand.Int63()))
	normalDist := distuv.Normal{Mu: 0, Sigma: 1, Src: source}

	for i := 0; i < steps; i++ {
		z1 := normalDist.Rand()
		z2 := params.Rho*z1 + math.Sqrt(1-params.Rho*params.Rho)*normalDist.Rand()

		S[i+1] = S[i] * math.Exp((r-0.5*v[i])*dt+math.Sqrt(v[i])*sqrtDt*z1)
		v[i+1] = math.Max(0, v[i]+params.Kappa*(params.Theta-v[i])*dt+params.Xi*math.Sqrt(v[i])*sqrtDt*z2)
	}

	return S
}
