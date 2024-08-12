package positions

import (
	"math"
	"time"

	"github.com/bcdannyboy/dquant/models"
	"github.com/bcdannyboy/dquant/tradier"
)

func calculateIntrinsicValue(shortLeg, longLeg models.SpreadLeg, underlyingPrice float64, spreadType string) float64 {
	if spreadType == "Bull Put" {
		return math.Max(0, shortLeg.Option.Strike-longLeg.Option.Strike-(shortLeg.Option.Strike-underlyingPrice))
	} else { // Bear Call
		return math.Max(0, longLeg.Option.Strike-shortLeg.Option.Strike-(underlyingPrice-shortLeg.Option.Strike))
	}
}

func calculateSingleOptionIntrinsicValue(option tradier.Option, underlyingPrice float64) float64 {
	if option.OptionType == "call" {
		return math.Max(0, underlyingPrice-option.Strike)
	}
	return math.Max(0, option.Strike-underlyingPrice)
}

func sanitizeFloat(f float64) float64 {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0
	}
	return f
}

func calculateTimeToMaturity(expirationDate string) float64 {
	expDate, _ := time.Parse("2006-01-02", expirationDate)
	now := time.Now()
	return expDate.Sub(now).Hours() / 24 / 365 // Convert to years
}
