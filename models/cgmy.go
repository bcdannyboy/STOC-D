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

// Characteristic Function incorporating stochastic volatility as per document
func (m *CGMYModel) CharacteristicFunction(u complex128, t float64, vol float64) complex128 {
	c := complex(m.C, 0)
	mComplex := complex(m.M, 0)
	gComplex := complex(m.G, 0)
	yComplex := complex(m.Y, 0)

	levySymbol := c * (cmplx.Pow(mComplex-u, yComplex) - cmplx.Pow(mComplex, yComplex) +
		cmplx.Pow(gComplex+u, yComplex) - cmplx.Pow(gComplex, yComplex))

	return cmplx.Exp(complex(t*vol, 0) * levySymbol)
}

func (m *CGMYModel) SimulatePrice(s0, r, t float64, steps int, rng *rand.Rand, vol []float64) float64 {
	if m.Y >= 2 {
		panic("Y must be less than 2 for the CGMY process")
	}

	dt := t / float64(steps)
	X := 0.0
	S := s0

	for i := 0; i < steps; i++ {
		dX := m.generateIncrement(dt, rng, vol[i])
		X += dX
		S *= math.Exp((r-0.5*vol[i]*vol[i]-m.calculateCompensator())*dt + dX)
	}

	return S
}

func (m *CGMYModel) generateIncrement(dt float64, rng *rand.Rand, vol float64) float64 {
	numJumps := m.samplePoisson(m.C*dt, rng)
	increment := vol * math.Sqrt(dt) * rng.NormFloat64() // Add diffusion component

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
	return m.C * math.Gamma(-m.Y) * (math.Pow(m.M, m.Y-1) + math.Pow(m.G, m.Y-1))
}

func (m *CGMYModel) Calibrate(historicalReturns []float64) error {
	fmt.Println("Starting CGMY calibration...")

	lowerBounds := []float64{0.0001, 0.01, 0.01, 0.01}
	upperBounds := []float64{100, 100, 100, 1.99}

	objective := func(x []float64) float64 {
		c, g, m, y := x[0], x[1], x[2], x[3]

		// Check bounds
		for i, val := range x {
			if val < lowerBounds[i] || val > upperBounds[i] {
				return math.Inf(1)
			}
		}

		tempM := &CGMYModel{C: c, G: g, M: m, Y: y}
		logLikelihood := 0.0
		for _, r := range historicalReturns {
			cf := tempM.CharacteristicFunction(complex(0, r), 1, 1.0)
			absCF := cmplx.Abs(cf)
			if absCF <= 0 || math.IsNaN(absCF) || math.IsInf(absCF, 0) {
				return math.Inf(1)
			}
			logLikelihood += math.Log(absCF)
		}

		// Use log-barrier penalty to keep parameters away from bounds
		penalty := 0.0
		for i, val := range x {
			penalty -= math.Log(val - lowerBounds[i])
			penalty -= math.Log(upperBounds[i] - val)
		}

		return -logLikelihood + penalty
	}

	problem := optimize.Problem{
		Func: objective,
	}

	method := &optimize.NelderMead{}

	// Use multiple random starts
	bestParams := make([]float64, 4)
	bestF := math.Inf(1)

	for i := 0; i < 100; i++ {
		initialParams := m.estimateInitialParams(historicalReturns)

		result, err := optimize.Minimize(problem, initialParams, &optimize.Settings{
			MajorIterations: 10000,
		}, method)

		if err == nil && result.F < bestF {
			bestF = result.F
			copy(bestParams, result.X)
		}
	}

	if math.IsInf(bestF, 0) {
		fmt.Println("Optimization failed to find valid parameters")
		fmt.Println("Using backup parameter estimation...")
		m.backupParameterEstimation(historicalReturns)
	} else {
		m.C, m.G, m.M, m.Y = bestParams[0], bestParams[1], bestParams[2], bestParams[3]
		fmt.Printf("Calibration complete. C=%f, G=%f, M=%f, Y=%f\n", m.C, m.G, m.M, m.Y)
	}

	return nil
}

func (m *CGMYModel) estimateInitialParams(historicalReturns []float64) []float64 {
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
	y := math.Max(0.1, math.Min(1.9, 2-2/(1+math.Abs(skewness))))
	c := math.Max(0.01, variance/(math.Gamma(2-y)*2))
	g := math.Max(0.1, math.Sqrt(2*c*math.Gamma(2-y)/variance))
	m_param := g // Renamed to avoid confusion with the method receiver

	if skewness < 0 {
		g, m_param = m_param, g
	}

	return []float64{c, g, m_param, y}
}

func (m *CGMYModel) backupParameterEstimation(historicalReturns []float64) {
	params := m.estimateInitialParams(historicalReturns)
	m.C, m.G, m.M, m.Y = params[0], params[1], params[2], params[3]
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
		characteristicFn := m.CharacteristicFunction(complex(u-(alpha+1), 0), t, 1.0)
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
