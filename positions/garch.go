package positions

import (
	"fmt"
	"math"

	"github.com/bcdannyboy/dquant/tradier"
	"gonum.org/v1/gonum/optimize"
	"gonum.org/v1/gonum/stat/distuv"
)

// LogLikelihood calculates the log-likelihood of the GARCH(1,1) model
func (g GARCH11) LogLikelihood(returns []float64) float64 {
	n := len(returns)
	logLik := 0.0
	variance := g.Omega / (1 - g.Alpha - g.Beta)

	for i := 1; i < n; i++ {
		variance = g.Omega + g.Alpha*returns[i-1]*returns[i-1] + g.Beta*variance
		logLik += -0.5*math.Log(2*math.Pi) - 0.5*math.Log(variance) - 0.5*returns[i]*returns[i]/variance
	}

	return logLik
}

// EstimateGARCH11 estimates GARCH(1,1) parameters using MCMC and BFGS
func EstimateGARCH11(returns []float64) (GARCH11, error) {
	// Initial guess
	initialGuess := GARCH11{Omega: 0.000001, Alpha: 0.1, Beta: 0.8}

	// MCMC parameters
	numIterations := 2000
	burnIn := 200
	stepSize := 0.01

	// MCMC chain
	chain := make([]GARCH11, numIterations)
	chain[0] = initialGuess

	for i := 1; i < numIterations; i++ {
		// Propose new parameters
		proposal := GARCH11{
			Omega: chain[i-1].Omega + distuv.Normal{Mu: 0, Sigma: stepSize}.Rand(),
			Alpha: chain[i-1].Alpha + distuv.Normal{Mu: 0, Sigma: stepSize}.Rand(),
			Beta:  chain[i-1].Beta + distuv.Normal{Mu: 0, Sigma: stepSize}.Rand(),
		}

		// Ensure parameters are within valid range
		if proposal.Omega <= 0 || proposal.Alpha < 0 || proposal.Beta < 0 || proposal.Alpha+proposal.Beta >= 1 {
			chain[i] = chain[i-1]
			continue
		}

		// Calculate acceptance probability
		logAcceptProb := proposal.LogLikelihood(returns) - chain[i-1].LogLikelihood(returns)

		if math.Log(distuv.Uniform{Min: 0, Max: 1}.Rand()) < logAcceptProb {
			chain[i] = proposal
		} else {
			chain[i] = chain[i-1]
		}
	}

	// Average post burn-in chain for initial Nelder-Mead guess
	avgParams := GARCH11{}
	for i := burnIn; i < numIterations; i++ {
		avgParams.Omega += chain[i].Omega
		avgParams.Alpha += chain[i].Alpha
		avgParams.Beta += chain[i].Beta
	}
	avgParams.Omega /= float64(numIterations - burnIn)
	avgParams.Alpha /= float64(numIterations - burnIn)
	avgParams.Beta /= float64(numIterations - burnIn)

	// Nelder-Mead optimization
	problem := optimize.Problem{
		Func: func(x []float64) float64 {
			return -GARCH11{Omega: x[0], Alpha: x[1], Beta: x[2]}.LogLikelihood(returns)
		},
	}

	result, err := optimize.Minimize(problem, []float64{avgParams.Omega, avgParams.Alpha, avgParams.Beta}, nil, &optimize.NelderMead{})
	if err != nil {
		// If Nelder-Mead fails, return the average parameters from MCMC
		fmt.Println("Nelder-Mead optimization failed:", err)
		return avgParams, nil
	}

	return GARCH11{Omega: result.X[0], Alpha: result.X[1], Beta: result.X[2]}, nil
}

// ConditionalVolatility calculates the conditional volatility using GARCH(1,1)
func (g GARCH11) ConditionalVolatility(returns []float64) float64 {
	n := len(returns)
	variance := g.Omega / (1 - g.Alpha - g.Beta)

	for i := 1; i < n; i++ {
		variance = g.Omega + g.Alpha*returns[i-1]*returns[i-1] + g.Beta*variance
	}

	return math.Sqrt(variance * 252) // Annualized
}

// CalculateReturns calculates daily returns from the QuoteHistory
func CalculateReturns(history tradier.QuoteHistory) []float64 {
	returns := make([]float64, len(history.History.Day)-1)
	for i := 1; i < len(history.History.Day); i++ {
		prevClose := history.History.Day[i-1].Close
		currClose := history.History.Day[i].Close
		returns[i-1] = math.Log(currClose / prevClose)
	}
	return returns
}

// CalculateGARCHVolatility estimates GARCH parameters, calculates volatility, and computes Greeks
func CalculateGARCHVolatility(history tradier.QuoteHistory, option tradier.Option, underlyingPrice, riskFreeRate float64) GARCHResult {
	returns := CalculateReturns(history)
	params, err := EstimateGARCH11(returns)
	if err != nil {
		// If GARCH estimation fails, use a default volatility (e.g., historical volatility)
		defaultVolatility := calculateHistoricalVolatility(returns)
		return GARCHResult{
			Params:     params,
			Volatility: defaultVolatility,
			Greeks:     calculateGreeks(option, underlyingPrice, riskFreeRate, defaultVolatility),
		}
	}
	volatility := params.ConditionalVolatility(returns)
	greeks := calculateGreeks(option, underlyingPrice, riskFreeRate, volatility)

	return GARCHResult{
		Params:     params,
		Volatility: volatility,
		Greeks:     greeks,
	}
}

func calculateHistoricalVolatility(returns []float64) float64 {
	var sum, sumSquared float64
	n := float64(len(returns))

	for _, r := range returns {
		sum += r
		sumSquared += r * r
	}

	mean := sum / n
	variance := (sumSquared/n - mean*mean) * (n / (n - 1))
	return math.Sqrt(variance * 252) // Annualized
}

// calculateGreeks computes the option Greeks using the Black-Scholes-Merton model
func calculateGreeks(option tradier.Option, underlyingPrice, riskFreeRate, volatility float64) BSMResult {
	S := underlyingPrice
	K := option.Strike
	T := calculateTimeToMaturity(option.ExpirationDate)
	r := riskFreeRate
	sigma := volatility

	d1 := (math.Log(S/K) + (r+0.5*sigma*sigma)*T) / (sigma * math.Sqrt(T))
	d2 := d1 - sigma*math.Sqrt(T)

	var price, delta, theta float64
	isCall := option.OptionType == "call"

	if isCall {
		price = S*normalCDF(d1) - K*math.Exp(-r*T)*normalCDF(d2)
		delta = normalCDF(d1)
		theta = -(S*normalPDF(d1)*sigma)/(2*math.Sqrt(T)) - r*K*math.Exp(-r*T)*normalCDF(d2)
	} else {
		price = K*math.Exp(-r*T)*normalCDF(-d2) - S*normalCDF(-d1)
		delta = normalCDF(d1) - 1
		theta = -(S*normalPDF(d1)*sigma)/(2*math.Sqrt(T)) + r*K*math.Exp(-r*T)*normalCDF(-d2)
	}

	gamma := normalPDF(d1) / (S * sigma * math.Sqrt(T))
	vega := S * normalPDF(d1) * math.Sqrt(T)
	rho := K * T * math.Exp(-r*T) * normalCDF(d2)
	if !isCall {
		rho = -K * T * math.Exp(-r*T) * normalCDF(-d2)
	}

	return BSMResult{
		Price:             price,
		ImpliedVolatility: sigma,
		Delta:             delta,
		Gamma:             gamma,
		Theta:             theta,
		Vega:              vega,
		Rho:               rho,
	}
}

// Helper functions for option pricing calculations
func normalCDF(x float64) float64 {
	return 0.5 * (1 + math.Erf(x/math.Sqrt2))
}

func normalPDF(x float64) float64 {
	return math.Exp(-0.5*x*x) / math.Sqrt(2*math.Pi)
}
