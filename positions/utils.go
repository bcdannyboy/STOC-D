package positions

import (
	"math"
	"sort"
	"time"

	"github.com/bcdannyboy/stocd/models"
	"github.com/bcdannyboy/stocd/tradier"
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

func calculateAverageVolatility(volatilities map[string]float64) float64 {
	sum := 0.0
	count := 0
	for _, vol := range volatilities {
		sum += vol
		count++
	}
	if count == 0 {
		return 0
	}
	return sum / float64(count)
}

func calculateAverageImpliedVolatility(chain map[string]*tradier.OptionChain) float64 {
	sum := 0.0
	count := 0
	for _, expiration := range chain {
		for _, option := range expiration.Options.Option {
			if option.Greeks.MidIv > 0 {
				sum += option.Greeks.MidIv
				count++
			}
		}
	}
	if count == 0 {
		return 0
	}
	return sum / float64(count)
}

func calculateHistoricalJumps(history tradier.QuoteHistory) []float64 {
	jumps := []float64{}
	for i := 1; i < len(history.History.Day); i++ {
		prevClose := history.History.Day[i-1].Close
		currOpen := history.History.Day[i].Open
		jump := math.Log(currOpen / prevClose)
		jumps = append(jumps, jump)
	}
	return jumps
}

func extractHistoricalPrices(history tradier.QuoteHistory) []float64 {
	prices := make([]float64, len(history.History.Day))
	for i, day := range history.History.Day {
		prices[i] = day.Close
	}
	return prices
}

func scaleHistoricalPrices(prices []float64, factor float64) []float64 {
	scaledPrices := make([]float64, len(prices))
	for i, price := range prices {
		if i == 0 {
			scaledPrices[i] = price
		} else {
			returnRate := math.Log(price / prices[i-1])
			scaledReturn := returnRate * factor
			scaledPrices[i] = scaledPrices[i-1] * math.Exp(scaledReturn)
		}
	}
	return scaledPrices
}

func extractAllStrikes(chain map[string]*tradier.OptionChain) []float64 {
	strikeSet := make(map[float64]struct{})

	for _, expiration := range chain {
		for _, option := range expiration.Options.Option {
			strikeSet[option.Strike] = struct{}{}
		}
	}

	strikes := make([]float64, 0, len(strikeSet))
	for strike := range strikeSet {
		strikes = append(strikes, strike)
	}

	sort.Float64s(strikes)

	return strikes
}
