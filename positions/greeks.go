package positions

import (
	"math"

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
