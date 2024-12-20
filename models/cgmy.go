package models

import (
	"fmt"
	"math"
	"math/cmplx"
	"runtime"
	"sort"
	"sync"

	"golang.org/x/exp/rand"
)

type CGMYParams struct {
	C, G, M, Y float64
}

type CGMYProcess struct {
	Params CGMYParams
}

func (p *CGMYProcess) ImpliedVolatility(marketPrice, s0, strike, r, t float64, isCall bool) float64 {
	bsFunc := func(vol float64) float64 {
		d1 := (math.Log(s0/strike) + (r+0.5*vol*vol)*t) / (vol * math.Sqrt(t))
		d2 := d1 - vol*math.Sqrt(t)
		var price float64
		if isCall {
			price = s0*mathPhi(d1) - strike*math.Exp(-r*t)*mathPhi(d2)
		} else {
			price = strike*math.Exp(-r*t)*mathPhi(-d2) - s0*mathPhi(-d1)
		}
		return price - marketPrice
	}

	return NewtonRaphson(bsFunc, 0.5, 1e-6, 100)
}

func NewtonRaphson(f func(float64) float64, x0, epsilon float64, maxIterations int) float64 {
	x := x0
	for i := 0; i < maxIterations; i++ {
		fx := f(x)
		if math.Abs(fx) < epsilon {
			return x
		}
		dfx := (f(x+epsilon) - fx) / epsilon
		x = x - fx/dfx
	}
	return x
}

func mathPhi(x float64) float64 {
	return 0.5 * (1 + math.Erf(x/math.Sqrt2))
}

func (cgmy *CGMYProcess) Calibrate(marketPrices []float64, strikes []float64, s0, r, t float64, isCall bool) {
	objectiveFunc := func(params []float64) float64 {
		tempCGMY := NewCGMYProcess(math.Abs(params[0]), math.Abs(params[1]), math.Abs(params[2]), math.Abs(params[3]))
		var mse float64
		for i, strike := range strikes {
			modelPrice := tempCGMY.OptionPrice(s0, strike, r, t, isCall, 1000)
			mse += math.Pow(modelPrice-marketPrices[i], 2)
		}
		return mse / float64(len(strikes))
	}

	initialGuess := []float64{cgmy.Params.C, cgmy.Params.G, cgmy.Params.M, cgmy.Params.Y}
	result := NelderMead(objectiveFunc, initialGuess, 1e-6, 1000)

	cgmy.Params = CGMYParams{C: math.Abs(result[0]), G: math.Abs(result[1]), M: math.Abs(result[2]), Y: math.Abs(result[3])}
}

func (p *CGMYProcess) FastOptionPrice(s0, strike, r, t float64, isCall bool) float64 {
	cf := func(u complex128) complex128 {
		return p.CharacteristicFunction(imag(u))
	}

	integrand := func(u float64) float64 {
		if u == 0 {
			return 0 // Avoid division by zero
		}
		var result float64
		if isCall {
			result = real(cmplx.Exp(-complex(0, u*math.Log(strike/s0))) * cf(complex(0, u-1)) / (complex(0, u) * cf(complex(0, -1))))
		} else {
			result = real(cmplx.Exp(-complex(0, u*math.Log(strike/s0))) * cf(complex(0, u)) / (complex(0, u)))
		}
		if math.IsNaN(result) || math.IsInf(result, 0) {
			return 0 // Return 0 for invalid results
		}
		return result
	}

	integral := integrate(integrand, 1e-8, 100, 1000) // Start from a small positive number instead of 0
	price := s0 * math.Exp(-r*t) * (0.5 + integral/math.Pi)

	if !isCall {
		price = price - s0*math.Exp(-r*t) + strike*math.Exp(-r*t)
	}

	if math.IsNaN(price) || math.IsInf(price, 0) {
		fmt.Printf("Invalid price calculated: %v\n", price)
		fmt.Printf("Params: s0=%.6f, strike=%.6f, r=%.6f, t=%.6f, isCall=%v\n", s0, strike, r, t, isCall)
		fmt.Printf("CGMY params: C=%.6f, G=%.6f, M=%.6f, Y=%.6f\n", p.Params.C, p.Params.G, p.Params.M, p.Params.Y)
		return s0 // Return the current stock price as a fallback
	}

	return price
}

func (p *CGMYProcess) CalculateVolatility() float64 {
	return math.Sqrt(p.Params.C * math.Gamma(2-p.Params.Y) * (1/math.Pow(p.Params.M, 2-p.Params.Y) + 1/math.Pow(p.Params.G, 2-p.Params.Y)))
}

func integrate(f func(float64) float64, a, b float64, n int) float64 {
	if n <= 0 {
		return 0 // Return 0 if n is non-positive
	}
	h := (b - a) / float64(n)
	sum := 0.5 * (f(a) + f(b))
	for i := 1; i < n; i++ {
		sum += f(a + float64(i)*h)
	}
	return sum * h
}

func NelderMead(f func([]float64) float64, start []float64, tol float64, maxIter int) []float64 {
	n := len(start)
	simplex := make([][]float64, n+1)
	simplex[0] = start
	for i := 1; i <= n; i++ {
		simplex[i] = make([]float64, n)
		copy(simplex[i], start)
		if simplex[i][i-1] != 0 {
			simplex[i][i-1] *= 1.05
		} else {
			simplex[i][i-1] = 0.00025
		}
	}

	values := make([]float64, n+1)
	for i := range simplex {
		values[i] = f(simplex[i])
	}

	// Nelder-Mead parameters
	alpha := 1.0 // reflection
	beta := 0.5  // contraction
	gamma := 2.0 // expansion
	delta := 0.5 // shrinkage

	var best []float64
	for iter := 0; iter < maxIter; iter++ {
		// Order
		order := make([]int, n+1)
		for i := range order {
			order[i] = i
		}
		sort.Slice(order, func(i, j int) bool {
			return values[order[i]] < values[order[j]]
		})

		best = simplex[order[0]]
		worst := simplex[order[n]]

		// Centroid
		centroid := make([]float64, n)
		for i := 0; i < n; i++ {
			sum := 0.0
			for j := 0; j < n; j++ {
				sum += simplex[order[j]][i]
			}
			centroid[i] = sum / float64(n)
		}

		// Reflection
		reflection := make([]float64, n)
		for i := range reflection {
			reflection[i] = math.Abs(centroid[i] + alpha*(centroid[i]-worst[i]))
		}
		reflectionValue := f(reflection)

		if reflectionValue < values[order[n-1]] && reflectionValue >= values[order[0]] {
			copy(simplex[order[n]], reflection)
			values[order[n]] = reflectionValue
		} else if reflectionValue < values[order[0]] {
			// Expansion
			expansion := make([]float64, n)
			for i := range expansion {
				expansion[i] = math.Abs(centroid[i] + gamma*(reflection[i]-centroid[i]))
			}
			expansionValue := f(expansion)
			if expansionValue < reflectionValue {
				copy(simplex[order[n]], expansion)
				values[order[n]] = expansionValue
			} else {
				copy(simplex[order[n]], reflection)
				values[order[n]] = reflectionValue
			}
		} else {
			// Contraction
			contraction := make([]float64, n)
			for i := range contraction {
				contraction[i] = math.Abs(centroid[i] + beta*(worst[i]-centroid[i]))
			}
			contractionValue := f(contraction)
			if contractionValue < values[order[n]] {
				copy(simplex[order[n]], contraction)
				values[order[n]] = contractionValue
			} else {
				// Shrink
				for i := 1; i <= n; i++ {
					for j := range simplex[order[i]] {
						simplex[order[i]][j] = math.Abs(best[j] + delta*(simplex[order[i]][j]-best[j]))
					}
					values[order[i]] = f(simplex[order[i]])
				}
			}
		}

		// Check for convergence
		if math.Abs(values[order[n]]-values[order[0]]) < tol {
			return best
		}
	}

	return best
}

///////////////////////////

func NewCGMYProcess(c, g, m, y float64) *CGMYProcess {
	return &CGMYProcess{
		Params: CGMYParams{C: c, G: g, M: m, Y: y},
	}
}

func (p *CGMYProcess) CharacteristicFunction(u float64) complex128 {
	c, g, m, y := p.Params.C, p.Params.G, p.Params.M, p.Params.Y

	term1 := complex(0, u*c*math.Gamma(1-y)*(math.Pow(m, y-1)-math.Pow(g, y-1)))
	term2 := complex(-c*math.Gamma(-y), 0) *
		(cmplx.Pow(complex(m-u, 0), complex(y, 0)) - cmplx.Pow(complex(m, 0), complex(y, 0)) +
			cmplx.Pow(complex(g+u, 0), complex(y, 0)) - cmplx.Pow(complex(g, 0), complex(y, 0)))

	result := cmplx.Exp(term1 + term2)

	if cmplx.IsNaN(result) || cmplx.IsInf(result) {
		return complex(1, 0) // Return 1 as a fallback
	}

	return result
}

func (p *CGMYProcess) SimulatePath(t, dt float64, rng *rand.Rand) []float64 {
	steps := int(t / dt)
	path := make([]float64, steps+1)

	for i := 1; i <= steps; i++ {
		path[i] = path[i-1] + p.SimulateIncrement(dt, rng)
	}

	return path
}

func (p *CGMYProcess) SimulateIncrement(dt float64, rng *rand.Rand) float64 {
	c, g, m, y := p.Params.C, p.Params.G, p.Params.M, p.Params.Y

	// Use the more stable series representation for small time steps
	if dt < 0.1 {
		var sum float64
		for j := 1; ; j++ {
			term := math.Pow(c*dt, float64(j)) / math.Gamma(float64(j)+1) *
				(math.Pow(m, y-float64(j)) + math.Pow(g, y-float64(j)))
			sum += term
			if term < 1e-10 {
				break
			}
		}
		return rng.NormFloat64()*math.Sqrt(dt*c*(1/m+1/g)) +
			c*dt*math.Gamma(-y)*(math.Pow(m, y-1)-math.Pow(g, y-1))
	}

	// Use the shot noise representation for larger time steps
	var sum float64
	for {
		E := rng.ExpFloat64()
		U := rng.Float64()
		if math.Pow(c*dt/E, 1/y) <= U {
			break
		}
		V := rng.Float64()
		W := rng.ExpFloat64()
		if V < 0.5 {
			sum -= math.Pow(W/m, 1/y)
		} else {
			sum += math.Pow(W/g, 1/y)
		}
	}
	return sum + c*dt*math.Gamma(-y)*(math.Pow(m, y-1)-math.Pow(g, y-1))
}

func (p *CGMYProcess) SimulatePathsBatch(t, dt float64, numPaths int) [][]float64 {
	paths := make([][]float64, numPaths)
	numWorkers := runtime.NumCPU()

	// Create a worker pool
	jobs := make(chan int, numPaths)
	results := make(chan struct {
		index int
		path  []float64
	}, numPaths)

	// Launch workers
	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rng := rand.New(rand.NewSource(uint64(rand.Int63())))

			for index := range jobs {
				path := p.SimulatePath(t, dt, rng)
				results <- struct {
					index int
					path  []float64
				}{index, path}
			}
		}()
	}

	// Assign jobs
	go func() {
		for i := 0; i < numPaths; i++ {
			jobs <- i
		}
		close(jobs)
	}()

	// Collect results
	go func() {
		for result := range results {
			paths[result.index] = result.path
		}
	}()

	wg.Wait()
	close(results)

	return paths
}

func (p *CGMYProcess) OptionPrice(s0, strike, r, t float64, isCall bool, numSimulations int) float64 {
	cf := func(u complex128) complex128 {
		return p.CharacteristicFunction(imag(u))
	}

	integrand := func(u float64) float64 {
		if u == 0 {
			return 0 // Avoid division by zero
		}
		var result float64
		if isCall {
			result = real(cmplx.Exp(-complex(0, u*math.Log(strike/s0))) * cf(complex(0, u-1)) / (complex(0, u) * cf(complex(0, -1))))
		} else {
			result = real(cmplx.Exp(-complex(0, u*math.Log(strike/s0))) * cf(complex(0, u)) / (complex(0, u)))
		}
		if math.IsNaN(result) || math.IsInf(result, 0) {
			return 0 // Return 0 for invalid results
		}
		return result
	}

	integral := integrate(integrand, 1e-8, 100, 1000) // Start from a small positive number instead of 0
	price := s0 * math.Exp(-r*t) * (0.5 + integral/math.Pi)

	if !isCall {
		price = price - s0*math.Exp(-r*t) + strike*math.Exp(-r*t)
	}

	if math.IsNaN(price) || math.IsInf(price, 0) {
		fmt.Printf("Invalid price calculated: %v\n", price)
		fmt.Printf("Params: s0=%.6f, strike=%.6f, r=%.6f, t=%.6f, isCall=%v\n", s0, strike, r, t, isCall)
		fmt.Printf("CGMY params: C=%.6f, G=%.6f, M=%.6f, Y=%.6f\n", p.Params.C, p.Params.G, p.Params.M, p.Params.Y)
		return s0 // Return the current stock price as a fallback
	}

	return price
}
