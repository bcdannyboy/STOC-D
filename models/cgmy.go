package models

import (
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
	objective := func(params []float64) float64 {
		m.C, m.G, m.M, m.Y = params[0], params[1], params[2], params[3]
		logLikelihood := 0.0
		for _, r := range historicalReturns {
			logLikelihood += math.Log(m.density(r))
		}
		return -logLikelihood
	}

	problem := optimize.Problem{
		Func: objective,
	}

	result, err := optimize.Minimize(problem, []float64{m.C, m.G, m.M, m.Y}, nil, &optimize.NelderMead{})
	if err != nil {
		return err
	}

	m.C, m.G, m.M, m.Y = result.X[0], result.X[1], result.X[2], result.X[3]
	return nil
}

func (m *CGMYModel) density(x float64) float64 {
	integrand := func(u float64) float64 {
		cf := m.CharacteristicFunction(complex(u, 0), 1)
		return real(cmplx.Exp(-complex(0, u*x)) * cf)
	}

	density, _ := integrate(integrand, -1000, 1000)
	return density / (2 * math.Pi)
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

// Helper function to perform numerical integration
func integrate(f func(float64) float64, a, b float64) (float64, error) {
	n := 1000
	h := (b - a) / float64(n)
	sum := 0.5 * (f(a) + f(b))
	for i := 1; i < n; i++ {
		sum += f(a + float64(i)*h)
	}
	return sum * h, nil
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
