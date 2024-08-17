package models

import (
	"fmt"
	"math"
	"math/cmplx"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/optimize"
)

type CGMYModel struct {
	C float64
	G float64
	M float64
	Y float64
}

func NewCGMYModel(c, g, m, y float64) *CGMYModel {
	return &CGMYModel{C: c, G: g, M: m, Y: y}
}

func (m *CGMYModel) CharacteristicFunction(u complex128, t float64) complex128 {
	return cmplx.Exp(complex(t*m.C, 0) * (cmplx.Pow(complex(m.M, 0)-u, complex(m.Y, 0)) -
		cmplx.Pow(complex(m.M, 0), complex(m.Y, 0)) +
		cmplx.Pow(complex(m.G, 0)+u, complex(m.Y, 0)) -
		cmplx.Pow(complex(m.G, 0), complex(m.Y, 0))))
}

func (m *CGMYModel) SimulatePrice(s0, r, t float64, steps int, rng *rand.Rand) float64 {
	if m.Y >= 2 {
		panic("Y must be less than 2 for the CGMY process")
	}

	dt := t / float64(steps)
	X := 0.0

	for i := 0; i < steps; i++ {
		dX := m.generateIncrement(dt, rng)
		X += dX
	}

	return s0 * math.Exp((r-m.calculateCompensator())*t+X)
}

func (m *CGMYModel) generateIncrement(dt float64, rng *rand.Rand) float64 {
	numJumps := m.samplePoisson(m.C*dt, rng)
	increment := 0.0

	for i := 0; i < numJumps; i++ {
		if rng.Float64() < m.M/(m.G+m.M) {
			increment += m.generatePositiveJump(rng)
		} else {
			increment += m.generateNegativeJump(rng)
		}
	}

	return increment
}

func (m *CGMYModel) generatePositiveJump(rng *rand.Rand) float64 {
	return rng.ExpFloat64() / m.M
}

func (m *CGMYModel) generateNegativeJump(rng *rand.Rand) float64 {
	return -rng.ExpFloat64() / m.G
}

func (m *CGMYModel) samplePoisson(lambda float64, rng *rand.Rand) int {
	L := math.Exp(-lambda)
	k := 0
	p := 1.0

	for p > L {
		k++
		p *= rng.Float64()
	}

	return k - 1
}

func (m *CGMYModel) calculateCompensator() float64 {
	return m.C * math.Gamma(-m.Y) * (math.Pow(m.M, m.Y) + math.Pow(m.G, m.Y) - math.Pow(m.M-1, m.Y) - math.Pow(m.G+1, m.Y))
}

func (m *CGMYModel) Calibrate(historicalReturns []float64) error {
	fmt.Println("Starting CGMY calibration...")

	// Initial parameter bounds
	lowerBounds := []float64{0, 0.1, 0.1, 0.1}
	upperBounds := []float64{100, 50, 100, 1.99}

	// Initial guess
	initialParams := []float64{1, 5, 5, 0.5}

	// Objective function
	objective := func(x []float64) float64 {
		c, g, m, y := x[0], x[1], x[2], x[3]

		// Safeguard against invalid parameter values
		if c <= 0 || g <= 0 || m <= 0 || y <= 0 || y >= 2 {
			return math.Inf(1)
		}

		tempM := &CGMYModel{C: c, G: g, M: m, Y: y}
		logLikelihood := 0.0
		for _, r := range historicalReturns {
			cf := tempM.CharacteristicFunction(complex(0, r), 1)
			absCF := cmplx.Abs(cf)
			if absCF <= 0 || math.IsNaN(absCF) || math.IsInf(absCF, 0) {
				return math.Inf(1)
			}
			logLikelihood += math.Log(absCF)
		}

		return -logLikelihood
	}

	problem := optimize.Problem{
		Func: objective,
	}

	result, err := optimize.Minimize(problem, initialParams, &optimize.Settings{
		MajorIterations: 1000,
		Converger: &optimize.FunctionConverge{
			Absolute:   1e-10,
			Relative:   1e-10,
			Iterations: 100,
		},
	}, &optimize.NelderMead{})

	if err != nil {
		fmt.Printf("Full optimization failed: %v\n", err)
		fmt.Println("Attempting fallback optimization...")
		result, err = m.fallbackOptimization(objective, lowerBounds, upperBounds)
		if err != nil {
			fmt.Printf("Fallback optimization failed: %v\n", err)
			fmt.Println("Using backup parameter estimation...")
			m.backupParameterEstimation(historicalReturns)
			return nil
		}
	}

	m.C, m.G, m.M, m.Y = result.X[0], result.X[1], result.X[2], result.X[3]
	return nil
}

func (m *CGMYModel) fallbackOptimization(objective func([]float64) float64, lowerBounds, upperBounds []float64) (*optimize.Result, error) {
	var bestResult *optimize.Result
	bestF := math.Inf(1)

	rng := rand.New(rand.NewSource(uint64(rand.Int63())))

	for i := 0; i < 10; i++ {
		initialParams := make([]float64, 4)
		for j := range initialParams {
			initialParams[j] = lowerBounds[j] + rng.Float64()*(upperBounds[j]-lowerBounds[j])
		}

		result, err := optimize.Minimize(optimize.Problem{Func: objective}, initialParams, nil, &optimize.NelderMead{})
		if err == nil && result.F < bestF {
			bestResult = result
			bestF = result.F
		}
	}

	if bestResult == nil {
		return nil, fmt.Errorf("fallback optimization failed to find valid parameters")
	}

	return bestResult, nil
}

func (m *CGMYModel) backupParameterEstimation(historicalReturns []float64) {
	var mean, variance, skewness, kurtosis float64

	// Calculate sample moments
	n := float64(len(historicalReturns))
	for _, r := range historicalReturns {
		mean += r
	}
	mean /= n

	for _, r := range historicalReturns {
		diff := r - mean
		variance += diff * diff
		skewness += diff * diff * diff
		kurtosis += diff * diff * diff * diff
	}
	variance /= n
	skewness /= n * math.Pow(variance, 1.5)
	kurtosis /= n * variance * variance
	kurtosis -= 3 // Excess kurtosis

	// Estimate CGMY parameters based on moments
	m.Y = 0.5 // Default value for Y
	m.C = variance / (math.Pow(1/m.G, m.Y) + math.Pow(1/m.M, m.Y))
	m.G = math.Sqrt(2 * m.C * math.Gamma(2-m.Y) / variance)
	m.M = m.G

	if skewness < 0 {
		m.G, m.M = m.M, m.G
	}

	fmt.Printf("Backup parameter estimation complete. C=%f, G=%f, M=%f, Y=%f\n", m.C, m.G, m.M, m.Y)
}

func (m *CGMYModel) OptionPrice(s, k, r, t float64) float64 {
	N := 4096
	alpha := 1.5
	eta := 0.25
	lambda := 2 * math.Pi / (float64(N) * eta)
	b := math.Pi / eta

	x := make([]complex128, N)
	for j := 0; j < N; j++ {
		u := eta * float64(j)
		complexU := complex(u, 0)
		characteristicFn := m.CharacteristicFunction(complex(u-(alpha+1), 0), t)
		denominator := complex(alpha*alpha+alpha, 0) - complexU*complexU + complex(0, (2*alpha+1)*u)
		x[j] = cmplx.Exp(complex(0, -b*u)) * characteristicFn / denominator
	}

	y := fft(x)

	v := math.Exp(-r*t) * eta * math.Exp(alpha*math.Log(k)) / math.Pi
	var price float64
	for j := 0; j < N; j++ {
		price += real(y[j]) * math.Exp(-alpha*lambda*float64(j))
	}

	return v * price * s / k
}

// Helper function to perform FFT
func fft(x []complex128) []complex128 {
	n := len(x)
	if n <= 1 {
		return x
	}
	even := fft(x[0:n:2])
	odd := fft(x[1:n:2])
	factor := complex(0, -2*math.Pi/float64(n))
	for k := 0; k < n/2; k++ {
		t := cmplx.Exp(factor*complex(float64(k), 0)) * odd[k]
		x[k] = even[k] + t
		x[k+n/2] = even[k] - t
	}
	return x
}
