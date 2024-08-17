package models

import (
	"math"
	"math/cmplx"
	"math/rand"

	"gonum.org/v1/gonum/optimize"
	"gonum.org/v1/gonum/stat/distuv"
)

// CGMYParams represents the parameters of the CGMY model
type CGMYParams struct {
	C float64
	G float64
	M float64
	Y float64
}

// CharacteristicFunction returns the characteristic function of the CGMY process
func (p CGMYParams) CharacteristicFunction(u complex128, t float64) complex128 {
	return cmplx.Exp(complex(t*p.C*math.Gamma(-p.Y), 0) * (cmplx.Pow(complex(p.M, 0)-u, complex(p.Y, 0)) -
		cmplx.Pow(complex(p.M, 0), complex(p.Y, 0)) +
		cmplx.Pow(complex(p.G, 0)+u, complex(p.Y, 0)) -
		cmplx.Pow(complex(p.G, 0), complex(p.Y, 0))))
}

// LevyMeasure returns the Lévy measure of the CGMY process
func (p CGMYParams) LevyMeasure(x float64) float64 {
	if x > 0 {
		return p.C * math.Exp(-p.M*x) / math.Pow(x, 1+p.Y)
	}
	return p.C * math.Exp(-p.G*math.Abs(x)) / math.Pow(math.Abs(x), 1+p.Y)
}

// EstimateCGMYParameters estimates CGMY parameters from historical returns
func EstimateCGMYParameters(returns []float64) CGMYParams {
	initialParams := CGMYParams{C: 1, G: 5, M: 5, Y: 0.5}

	objective := func(params []float64) float64 {
		cgmyParams := CGMYParams{C: params[0], G: params[1], M: params[2], Y: params[3]}
		logLikelihood := 0.0
		for _, r := range returns {
			logLikelihood += math.Log(CGMYDensity(r, cgmyParams))
		}
		return -logLikelihood
	}

	problem := optimize.Problem{
		Func: objective,
		Grad: nil,
	}

	result, _ := optimize.Minimize(problem, []float64{initialParams.C, initialParams.G, initialParams.M, initialParams.Y}, nil, &optimize.NelderMead{})

	return CGMYParams{C: result.X[0], G: result.X[1], M: result.X[2], Y: result.X[3]}
}

// CGMYDensity computes the probability density of the CGMY distribution
func CGMYDensity(x float64, params CGMYParams) float64 {
	integrand := func(u float64) float64 {
		cf := params.CharacteristicFunction(complex(u, 0), 1)
		return real(cmplx.Exp(-complex(0, u*x)) * cf)
	}

	density, _ := integrate(integrand, -1000, 1000)
	return density / (2 * math.Pi)
}

// CGMYOptionPrice prices a European option using the CGMY model and FFT
func CGMYOptionPrice(s, k, r, t float64, params CGMYParams) float64 {
	N := 4096
	alpha := 1.5
	eta := 0.25
	lambda := 2 * math.Pi / (float64(N) * eta)
	b := math.Pi / eta

	x := make([]complex128, N)
	for j := 0; j < N; j++ {
		u := eta * float64(j)
		complexU := complex(u, 0)
		characteristicFn := params.CharacteristicFunction(complex(u-(alpha+1), 0), t)
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

// CGMYMoments calculates the first four moments of the CGMY distribution
func CGMYMoments(params CGMYParams) (mean, variance, skewness, kurtosis float64) {
	mean = params.C * math.Gamma(1-params.Y) * (math.Pow(params.M, params.Y-1) - math.Pow(params.G, params.Y-1))
	variance = params.C * math.Gamma(2-params.Y) * (math.Pow(params.M, params.Y-2) + math.Pow(params.G, params.Y-2))
	skewness = params.C * math.Gamma(3-params.Y) * (math.Pow(params.M, params.Y-3) - math.Pow(params.G, params.Y-3)) /
		math.Pow(variance, 1.5)
	kurtosis = params.C * math.Gamma(4-params.Y) * (math.Pow(params.M, params.Y-4) + math.Pow(params.G, params.Y-4)) /
		(variance * variance)
	return
}

// SimulateCGMY generates a sample path of the CGMY process
func SimulateCGMY(params CGMYParams, T float64, N int) []float64 {
	dt := T / float64(N)
	path := make([]float64, N+1)
	path[0] = 0

	for i := 1; i <= N; i++ {
		dX := generateCGMYIncrement(params, dt)
		path[i] = path[i-1] + dX
	}

	return path
}

// generateCGMYIncrement generates a single increment of the CGMY process
func generateCGMYIncrement(params CGMYParams, dt float64) float64 {
	epsilon := 1e-10
	M := int(math.Ceil(params.C * dt / epsilon))

	increment := 0.0
	for j := 1; j <= M; j++ {
		Gamma_j := generateGamma(j, params.C*dt)
		U_j := rand.Float64()
		V_j := rand.Float64()

		if U_j <= params.C/(params.C+params.M+params.G) {
			X_j := generatePositiveJump(params, Gamma_j, V_j)
			increment += X_j
		} else {
			X_j := generateNegativeJump(params, Gamma_j, V_j)
			increment += X_j
		}
	}

	drift := params.C * math.Gamma(-params.Y) * (math.Pow(params.M, params.Y-1) - math.Pow(params.G, params.Y-1))
	increment -= drift * dt

	return increment
}

// generateGamma generates a sample from Gamma(j, 1)
func generateGamma(j int, rate float64) float64 {
	return distuv.Gamma{Alpha: float64(j), Beta: rate}.Rand()
}

// generatePositiveJump generates a positive jump for the CGMY process
func generatePositiveJump(params CGMYParams, Gamma float64, V float64) float64 {
	if params.Y == 0 {
		return -math.Log(V) / params.M
	} else if params.Y == 1 {
		return Gamma / params.M
	} else {
		return math.Pow(Gamma/params.C, 1/params.Y) * math.Pow(-math.Log(V), -1/params.Y) / params.M
	}
}

// generateNegativeJump generates a negative jump for the CGMY process
func generateNegativeJump(params CGMYParams, Gamma float64, V float64) float64 {
	if params.Y == 0 {
		return math.Log(V) / params.G
	} else if params.Y == 1 {
		return -Gamma / params.G
	} else {
		return -math.Pow(Gamma/params.C, 1/params.Y) * math.Pow(-math.Log(V), -1/params.Y) / params.G
	}
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

// CalibrateCGMYToOptionPrices calibrates CGMY parameters to market option prices
func CalibrateCGMYToOptionPrices(marketPrices []float64, strikes []float64, s, r, t float64) CGMYParams {
	initialParams := CGMYParams{C: 1, G: 5, M: 5, Y: 0.5}

	objective := func(params []float64) float64 {
		cgmyParams := CGMYParams{C: params[0], G: params[1], M: params[2], Y: params[3]}
		mse := 0.0
		for i, strike := range strikes {
			modelPrice := CGMYOptionPrice(s, strike, r, t, cgmyParams)
			mse += math.Pow(modelPrice-marketPrices[i], 2)
		}
		return mse / float64(len(strikes))
	}

	problem := optimize.Problem{
		Func: objective,
		Grad: nil,
	}

	result, _ := optimize.Minimize(problem, []float64{initialParams.C, initialParams.G, initialParams.M, initialParams.Y}, nil, &optimize.NelderMead{})

	return CGMYParams{C: result.X[0], G: result.X[1], M: result.X[2], Y: result.X[3]}
}
