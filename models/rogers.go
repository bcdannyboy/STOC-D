package models

import (
	"math"

	"github.com/bcdannyboy/dquant/tradier"
)

func CalculateRogersSatchellVolatility(history tradier.QuoteHistory) map[string]float64 {
	results := make(map[string]float64)

	periods := []struct {
		name string
		days int
	}{
		{"1w", 5},
		{"1m", 21},
		{"3m", 63},
		{"6m", 126},
	}

	for _, period := range periods {
		if len(history.History.Day) >= period.days {
			if volatility := calculatePeriodRogersSatchell(history, period.days); volatility != 0 {
				results[period.name] = volatility
			}
		}
	}

	return results
}

func calculatePeriodRogersSatchell(history tradier.QuoteHistory, days int) float64 {
	if len(history.History.Day) < days {
		return 0
	}

	opens := make([]float64, days)
	highs := make([]float64, days)
	lows := make([]float64, days)
	closes := make([]float64, days)

	for i := 0; i < days; i++ {
		day := history.History.Day[len(history.History.Day)-days+i]
		opens[i] = day.Open
		highs[i] = day.High
		lows[i] = day.Low
		closes[i] = day.Close
	}

	return calculateRogersSatchell(opens, highs, lows, closes)
}

func calculateRogersSatchell(opens, highs, lows, closes []float64) float64 {
	n := len(opens)
	if n == 0 || n != len(highs) || n != len(lows) || n != len(closes) {
		return 0
	}

	sum := 0.0
	for i := 0; i < n; i++ {
		sum += math.Log(highs[i]/closes[i])*math.Log(highs[i]/opens[i]) +
			math.Log(lows[i]/closes[i])*math.Log(lows[i]/opens[i])
	}

	// Annualize the volatility
	return math.Sqrt(sum / float64(n) * 252)
}
