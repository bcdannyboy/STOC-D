package positions

import (
	"math"
	"math/rand"
	"time"

	"github.com/bcdannyboy/dquant/tradier"
)

// CalculateOptionMetrics calculates BSM price, Greeks, and implied volatility for a single option
func CalculateOptionMetrics(option *tradier.Option, underlyingPrice, riskFreeRate float64) BSMResult {
	S := underlyingPrice
	X := option.Strike
	T := calculateTimeToMaturity(option.ExpirationDate)
	r := riskFreeRate

	targetPrice := (option.Bid + option.Ask) / 2 // Use mid-price as target
	isCall := option.OptionType == "call"

	// Calculate implied volatility
	impliedVol := calculateImpliedVolatility(targetPrice, S, X, T, r, isCall)

	// Calculate BSM metrics using the calculated implied volatility
	result := calculateBSM(S, X, T, r, impliedVol, isCall)
	result.ImpliedVolatility = impliedVol

	// Calculate Shadow Gamma (assuming 1% price change and 5% volatility change)
	result.ShadowUpGamma, result.ShadowDownGamma = ShadowGamma(*option, S, r, impliedVol, 0.01, 0.05)

	// Calculate Skew Gamma (assuming 0.1% volatility step)
	result.SkewGamma = SkewGamma(*option, S, r, impliedVol, 0.001)

	return result
}

func calculateBSM(S, X, T, r, sigma float64, isCall bool) BSMResult {
	sqrtT := math.Sqrt(T)
	d1 := (math.Log(S/X) + (r+0.5*sigma*sigma)*T) / (sigma * sqrtT)
	d2 := d1 - sigma*sqrtT

	callPrice := S*normCDF(d1) - X*math.Exp(-r*T)*normCDF(d2)
	putPrice := X*math.Exp(-r*T)*normCDF(-d2) - S*normCDF(-d1)

	price := callPrice
	if !isCall {
		price = putPrice
	}

	delta := normCDF(d1)
	if !isCall {
		delta = delta - 1
	}

	gamma := normPDF(d1) / (S * sigma * sqrtT)
	vega := S * normPDF(d1) * sqrtT

	theta := -(S*normPDF(d1)*sigma)/(2*sqrtT) - r*X*math.Exp(-r*T)*normCDF(d2)
	if !isCall {
		theta = theta + r*X*math.Exp(-r*T)
	}

	rho := X * T * math.Exp(-r*T) * normCDF(d2)
	if !isCall {
		rho = -X * T * math.Exp(-r*T) * normCDF(-d2)
	}

	return BSMResult{
		Price: price,
		Delta: delta,
		Gamma: gamma,
		Theta: theta,
		Vega:  vega,
		Rho:   rho,
	}
}

func calculateImpliedVolatility(targetPrice, S, X, T, r float64, isCall bool) float64 {
	const epsilon = 1e-5
	const maxIterations = 100

	sigma := 0.5 // Initial guess
	for i := 0; i < maxIterations; i++ {
		result := calculateBSM(S, X, T, r, sigma, isCall)
		price := result.Price
		vega := result.Vega

		diff := price - targetPrice
		if math.Abs(diff) < epsilon {
			return sigma
		}

		sigma = sigma - diff/vega
	}

	return math.NaN() // Failed to converge
}

func calculateTimeToMaturity(expirationDate string) float64 {
	expDate, _ := time.Parse("2006-01-02", expirationDate)
	now := time.Now()
	return expDate.Sub(now).Hours() / 24 / 365 // Convert to years
}

func normCDF(x float64) float64 {
	return 0.5 * (1 + math.Erf(x/math.Sqrt2))
}

func normPDF(x float64) float64 {
	return math.Exp(-0.5*x*x) / math.Sqrt(2*math.Pi)
}

// SimulateGBM simulates a Geometric Brownian Motion path
func SimulateGBM(S0, mu, sigma float64, T float64, steps int) []float64 {
	dt := T / float64(steps)
	prices := make([]float64, steps+1)
	prices[0] = S0

	for i := 1; i <= steps; i++ {
		dW := math.Sqrt(dt) * rand.NormFloat64()
		dS := mu*prices[i-1]*dt + sigma*prices[i-1]*dW
		prices[i] = prices[i-1] + dS
	}

	return prices
}
