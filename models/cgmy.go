package models

import (
	"fmt"
	"math"
	"math/cmplx"
	"math/rand"

	"gonum.org/v1/gonum/optimize"
	"gonum.org/v1/gonum/stat/distuv"
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

func (m *CGMYModel) SimulatePrice(s0, r, t float64, steps int) float64 {
	if m.Y >= 2 {
		panic("Y must be less than 2 for the CGMY process")
	}

	dt := t / float64(steps)
	X := 0.0

	for i := 0; i < steps; i++ {
		dX := m.generateIncrement(dt)
		X += dX
	}

	return s0 * math.Exp((r-m.calculateCompensator())*t+X)
}

func (m *CGMYModel) generateIncrement(dt float64) float64 {
	numJumps := distuv.Poisson{Lambda: m.C * dt}.Rand()
	increment := 0.0

	for i := 0; i < int(numJumps); i++ {
		if rand.Float64() < m.M/(m.G+m.M) {
			increment += m.generatePositiveJump()
		} else {
			increment += m.generateNegativeJump()
		}
	}

	return increment
}

func (m *CGMYModel) generatePositiveJump() float64 {
	return rand.ExpFloat64() / m.M
}

func (m *CGMYModel) generateNegativeJump() float64 {
	return -rand.ExpFloat64() / m.G
}

func (m *CGMYModel) calculateCompensator() float64 {
	return m.C * math.Gamma(-m.Y) * (math.Pow(m.M, m.Y) + math.Pow(m.G, m.Y) - math.Pow(m.M-1, m.Y) - math.Pow(m.G+1, m.Y))
}

func (m *CGMYModel) Calibrate(historicalReturns []float64) error {
	fmt.Println("Starting CGMY calibration...")

	objective := func(x []float64) float64 {
		c, g, mm, y := math.Exp(x[0]), math.Exp(x[1]), math.Exp(x[2]), 1.99/(1+math.Exp(-x[3]))

		fmt.Printf("Trying parameters: C=%f, G=%f, M=%f, Y=%f\n", c, g, mm, y)

		tempM := &CGMYModel{C: c, G: g, M: mm, Y: y}
		logLikelihood := 0.0
		for _, r := range historicalReturns {
			cf := tempM.CharacteristicFunction(complex(0, r), 1)
			absCF := cmplx.Abs(cf)
			if absCF <= 0 || math.IsNaN(absCF) || math.IsInf(absCF, 0) {
				fmt.Printf("Invalid CF value for r=%f\n", r)
				return math.Inf(1)
			}
			logLikelihood += math.Log(absCF)
		}

		// Adjusted regularization term
		regularization := 0.0001 * (math.Pow(c, 2) + math.Pow(g, 2) + math.Pow(mm, 2) + math.Pow(y, 2))

		result := -logLikelihood + regularization
		fmt.Printf("Objective function value: %f\n", result)
		return result
	}

	p := optimize.Problem{Func: objective}

	initialParams := []float64{math.Log(0.1), math.Log(5), math.Log(5), 0}

	method := &optimize.NelderMead{}

	settings := &optimize.Settings{
		FuncEvaluations: 10000,
		MajorIterations: 1000,
		Converger: &optimize.FunctionConverge{
			Iterations: 100,
			Relative:   1e-6,
		},
	}

	fmt.Println("Running optimization...")
	result, err := optimize.Minimize(p, initialParams, settings, method)

	if err != nil {
		fmt.Printf("Optimization failed: %v\n", err)
		m.C, m.G, m.M, m.Y = 0.1, 5, 5, 0.5
		return fmt.Errorf("optimization failed, using default values: %v", err)
	}

	if result.Status != optimize.Success {
		fmt.Printf("Optimization did not converge: %v\n", result.Status)
		return fmt.Errorf("optimization did not converge: %v", result.Status)
	}

	m.C, m.G, m.M = math.Exp(result.X[0]), math.Exp(result.X[1]), math.Exp(result.X[2])
	m.Y = 1.99 / (1 + math.Exp(-result.X[3]))

	fmt.Printf("Calibration successful. Final parameters: C=%f, G=%f, M=%f, Y=%f\n", m.C, m.G, m.M, m.Y)
	return nil
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
