package positions

import (
	"math"

	"github.com/bcdannyboy/dquant/models"
	"github.com/bcdannyboy/dquant/tradier"
)

// ShadowGamma calculates the Shadow Up-Gamma and Shadow Down-Gamma
func ShadowGamma(option tradier.Option, underlyingPrice, riskFreeRate, volatility float64, priceChange, volChange float64) (float64, float64) {
	originalDelta := calculateDelta(option, underlyingPrice, riskFreeRate, volatility)

	// Calculate Shadow Up-Gamma
	newPriceUp := underlyingPrice * (1 + priceChange)
	newVolUp := volatility * (1 + volChange)
	newDeltaUp := calculateDelta(option, newPriceUp, riskFreeRate, newVolUp)
	shadowUpGamma := (newDeltaUp - originalDelta) / (newPriceUp - underlyingPrice)

	// Calculate Shadow Down-Gamma
	newPriceDown := underlyingPrice * (1 - priceChange)
	newVolDown := volatility * (1 - volChange)
	newDeltaDown := calculateDelta(option, newPriceDown, riskFreeRate, newVolDown)
	shadowDownGamma := (originalDelta - newDeltaDown) / (underlyingPrice - newPriceDown)

	return shadowUpGamma, shadowDownGamma
}

// SkewGamma calculates the Skew Gamma (Volga)
func SkewGamma(option tradier.Option, underlyingPrice, riskFreeRate, volatility float64, volStep float64) float64 {
	vegaUp := calculateVega(option, underlyingPrice, riskFreeRate, volatility+volStep)
	vegaDown := calculateVega(option, underlyingPrice, riskFreeRate, volatility-volStep)

	return (vegaUp - vegaDown) / (2 * volStep)
}

// Helper function to calculate Delta
func calculateDelta(option tradier.Option, underlyingPrice, riskFreeRate, volatility float64) float64 {
	S := underlyingPrice
	K := option.Strike
	T := calculateTimeToMaturity(option.ExpirationDate)
	r := riskFreeRate
	sigma := volatility

	d1 := (math.Log(S/K) + (r+0.5*sigma*sigma)*T) / (sigma * math.Sqrt(T))

	if option.OptionType == "call" {
		return normalCDF(d1)
	}
	return normalCDF(d1) - 1
}

// Helper function to calculate Vega
func calculateVega(option tradier.Option, underlyingPrice, riskFreeRate, volatility float64) float64 {
	S := underlyingPrice
	K := option.Strike
	T := calculateTimeToMaturity(option.ExpirationDate)
	r := riskFreeRate
	sigma := volatility

	d1 := (math.Log(S/K) + (r+0.5*sigma*sigma)*T) / (sigma * math.Sqrt(T))

	return S * normalPDF(d1) * math.Sqrt(T)
}

func calculateShadowGammas(shortOpt, longOpt tradier.Option, underlyingPrice, riskFreeRate, volatility float64) (float64, float64) {
	shortUpGamma, shortDownGamma := ShadowGamma(shortOpt, underlyingPrice, riskFreeRate, volatility, 0.01, 0.05)
	longUpGamma, longDownGamma := ShadowGamma(longOpt, underlyingPrice, riskFreeRate, volatility, 0.01, 0.05)

	return shortUpGamma - longUpGamma, shortDownGamma - longDownGamma
}

func calculateSkewGamma(shortOpt, longOpt tradier.Option, underlyingPrice, riskFreeRate, volatility float64) float64 {
	shortSkewGamma := SkewGamma(shortOpt, underlyingPrice, riskFreeRate, volatility, 0.01)
	longSkewGamma := SkewGamma(longOpt, underlyingPrice, riskFreeRate, volatility, 0.01)

	return shortSkewGamma - longSkewGamma
}

func calculateSpreadGreeks(shortLeg, longLeg models.SpreadLeg) models.BSMResult {
	return models.BSMResult{
		Price:             sanitizeFloat(shortLeg.BSMResult.Price - longLeg.BSMResult.Price),
		ImpliedVolatility: sanitizeFloat(shortLeg.BSMResult.ImpliedVolatility - longLeg.BSMResult.ImpliedVolatility),
		Delta:             sanitizeFloat(shortLeg.BSMResult.Delta - longLeg.BSMResult.Delta),
		Gamma:             sanitizeFloat(shortLeg.BSMResult.Gamma - longLeg.BSMResult.Gamma),
		Theta:             sanitizeFloat(shortLeg.BSMResult.Theta - longLeg.BSMResult.Theta),
		Vega:              sanitizeFloat(shortLeg.BSMResult.Vega - longLeg.BSMResult.Vega),
		Rho:               sanitizeFloat(shortLeg.BSMResult.Rho - longLeg.BSMResult.Rho),
	}
}

// normalCDF calculates the cumulative distribution function of the standard normal distribution
func normalCDF(x float64) float64 {
	return 0.5 * (1 + math.Erf(x/math.Sqrt2))
}

// normalPDF calculates the probability density function of the standard normal distribution
func normalPDF(x float64) float64 {
	return math.Exp(-0.5*x*x) / math.Sqrt(2*math.Pi)
}
