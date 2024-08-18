package probability

import (
	"math"
	"sort"

	"github.com/bcdannyboy/stocd/models"
)

// CalculateVaR computes the Value at Risk for a given spread
func CalculateVaR(spread models.OptionSpread, simulations []float64, confidenceLevel float64) float64 {
	// Calculate the potential loss for each simulation
	losses := make([]float64, len(simulations))
	for i, finalPrice := range simulations {
		pnl := calculatePnL(spread, finalPrice)
		losses[i] = -pnl // Convert profit to loss
	}

	// Sort the losses
	sort.Float64s(losses)

	// Find the VaR at the given confidence level
	index := int(float64(len(losses)) * (1 - confidenceLevel))
	return losses[index]
}

// calculatePnL computes the profit/loss for a spread given a final price
func calculatePnL(spread models.OptionSpread, finalPrice float64) float64 {
	var pnl float64
	if spread.SpreadType == "Bull Put" {
		pnl = math.Max(0, spread.ShortLeg.Option.Strike-finalPrice) -
			math.Max(0, spread.LongLeg.Option.Strike-finalPrice) +
			spread.SpreadCredit
	} else { // Bear Call
		pnl = math.Max(0, finalPrice-spread.ShortLeg.Option.Strike) -
			math.Max(0, finalPrice-spread.LongLeg.Option.Strike) +
			spread.SpreadCredit
	}
	return pnl
}
