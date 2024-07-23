package positions

import (
	"math"

	"github.com/bcdannyboy/dquant/tradier"
)

const (
	maxIterations = 1000
	epsilon       = 1e-8
)

func CalculateOptionMetrics(option *tradier.Option, underlyingPrice, riskFreeRate float64) BSMResult {
	T := calculateTimeToMaturity(option.ExpirationDate)
	isCall := option.OptionType == "call"

	// Use mid price as target
	targetPrice := (option.Bid + option.Ask) / 2

	// Calculate implied volatility
	impliedVol := calculateImpliedVolatility(targetPrice, underlyingPrice, option.Strike, T, riskFreeRate, isCall)

	// Calculate BSM metrics
	d1 := (math.Log(underlyingPrice/option.Strike) + (riskFreeRate+0.5*impliedVol*impliedVol)*T) / (impliedVol * math.Sqrt(T))
	d2 := d1 - impliedVol*math.Sqrt(T)

	var delta, price float64
	if isCall {
		delta = normCDF(d1)
		price = underlyingPrice*normCDF(d1) - option.Strike*math.Exp(-riskFreeRate*T)*normCDF(d2)
	} else {
		delta = normCDF(d1) - 1
		price = option.Strike*math.Exp(-riskFreeRate*T)*normCDF(-d2) - underlyingPrice*normCDF(-d1)
	}

	gamma := normPDF(d1) / (underlyingPrice * impliedVol * math.Sqrt(T))
	vega := underlyingPrice * normPDF(d1) * math.Sqrt(T)
	theta := -(underlyingPrice*normPDF(d1)*impliedVol)/(2*math.Sqrt(T)) - riskFreeRate*option.Strike*math.Exp(-riskFreeRate*T)*normCDF(d2)
	rho := option.Strike * T * math.Exp(-riskFreeRate*T) * normCDF(d2)

	if !isCall {
		theta = theta + riskFreeRate*option.Strike*math.Exp(-riskFreeRate*T)
		rho = -option.Strike * T * math.Exp(-riskFreeRate*T) * normCDF(-d2)
	}

	// Calculate Shadow Gammas and Skew Gamma
	shadowUpGamma, shadowDownGamma := calculateShadowGamma(option, underlyingPrice, riskFreeRate, impliedVol)
	skewGamma := calculateBSMSkewGamma(option, underlyingPrice, riskFreeRate, impliedVol)

	return BSMResult{
		Price:             price,
		ImpliedVolatility: impliedVol,
		Delta:             delta,
		Gamma:             gamma,
		Theta:             theta,
		Vega:              vega,
		Rho:               rho,
		ShadowUpGamma:     shadowUpGamma,
		ShadowDownGamma:   shadowDownGamma,
		SkewGamma:         skewGamma,
	}
}

func calculateImpliedVolatility(targetPrice, S, K, T, r float64, isCall bool) float64 {
	sigma := 0.5 // Initial guess
	for i := 0; i < maxIterations; i++ {
		price := calculateOptionPrice(S, K, T, r, sigma, isCall)
		vega := calculateBSMVega(S, K, T, r, sigma)

		diff := price - targetPrice
		if math.Abs(diff) < epsilon {
			return sigma
		}

		if math.Abs(vega) < epsilon {
			break
		}

		sigma = sigma - diff/vega
		if sigma <= 0 {
			sigma = 0.0001 // Avoid negative volatility
		}
	}
	return math.NaN() // Failed to converge
}

func calculateBSM(S, K, T, r, sigma float64, isCall bool) BSMResult {
	d1 := (math.Log(S/K) + (r+0.5*sigma*sigma)*T) / (sigma * math.Sqrt(T))
	d2 := d1 - sigma*math.Sqrt(T)

	var delta, price float64
	if isCall {
		delta = normCDF(d1)
		price = S*normCDF(d1) - K*math.Exp(-r*T)*normCDF(d2)
	} else {
		delta = normCDF(d1) - 1
		price = K*math.Exp(-r*T)*normCDF(-d2) - S*normCDF(-d1)
	}

	gamma := normPDF(d1) / (S * sigma * math.Sqrt(T))
	vega := S * normPDF(d1) * math.Sqrt(T)
	theta := -(S*normPDF(d1)*sigma)/(2*math.Sqrt(T)) - r*K*math.Exp(-r*T)*normCDF(d2)
	rho := K * T * math.Exp(-r*T) * normCDF(d2)
	if !isCall {
		theta = theta + r*K*math.Exp(-r*T)
		rho = -K * T * math.Exp(-r*T) * normCDF(-d2)
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

func calculateOptionPrice(S, K, T, r, sigma float64, isCall bool) float64 {
	d1 := (math.Log(S/K) + (r+0.5*sigma*sigma)*T) / (sigma * math.Sqrt(T))
	d2 := d1 - sigma*math.Sqrt(T)

	if isCall {
		return S*normCDF(d1) - K*math.Exp(-r*T)*normCDF(d2)
	}
	return K*math.Exp(-r*T)*normCDF(-d2) - S*normCDF(-d1)
}

func calculateBSMVega(S, K, T, r, sigma float64) float64 {
	d1 := (math.Log(S/K) + (r+0.5*sigma*sigma)*T) / (sigma * math.Sqrt(T))
	return S * normPDF(d1) * math.Sqrt(T)
}

func calculateShadowGamma(option *tradier.Option, S, r, sigma float64) (float64, float64) {
	T := calculateTimeToMaturity(option.ExpirationDate)
	isCall := option.OptionType == "call"

	// Calculate up and down scenarios
	upS := S * 1.01
	downS := S * 0.99
	upSigma := sigma * 1.05
	downSigma := sigma * 0.95

	// Calculate deltas for each scenario
	baseDelta := calculateBSM(S, option.Strike, T, r, sigma, isCall).Delta
	upDelta := calculateBSM(upS, option.Strike, T, r, upSigma, isCall).Delta
	downDelta := calculateBSM(downS, option.Strike, T, r, downSigma, isCall).Delta

	// Calculate Shadow Gammas
	shadowUpGamma := (upDelta - baseDelta) / (upS - S)
	shadowDownGamma := (baseDelta - downDelta) / (S - downS)

	return shadowUpGamma, shadowDownGamma
}

func calculateBSMSkewGamma(option *tradier.Option, S, r, sigma float64) float64 {
	T := calculateTimeToMaturity(option.ExpirationDate)
	isCall := option.OptionType == "call"

	// Calculate vega for slightly different volatilities
	upSigma := sigma * 1.001
	downSigma := sigma * 0.999

	upVega := calculateBSM(S, option.Strike, T, r, upSigma, isCall).Vega
	downVega := calculateBSM(S, option.Strike, T, r, downSigma, isCall).Vega

	// Calculate Skew Gamma (Vomma)
	return (upVega - downVega) / (upSigma - downSigma)
}

func normCDF(x float64) float64 {
	return 0.5 * (1 + math.Erf(x/math.Sqrt2))
}

func normPDF(x float64) float64 {
	return math.Exp(-0.5*x*x) / math.Sqrt(2*math.Pi)
}
