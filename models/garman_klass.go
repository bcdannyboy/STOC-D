package models

import (
	"math"

	"github.com/bcdannyboy/dquant/tradier"
)

func CalculateGarmanKlassVolatilities(history tradier.QuoteHistory) map[string]float64 {
	results := make(map[string]float64)

	periods := []struct {
		name string
		days int
	}{
		{"Last Day", 1},
		{"5d", 5},
		{"1w", 5},
		{"2w", 10},
		{"1m", 21},
		{"3m", 63},
		{"6m", 126},
		{"1y", 252},
		{"3y", 756},
		{"5y", 1260},
		{"10y", 2520},
	}

	for _, period := range periods {
		if len(history.History.Day) >= period.days {
			if volatility := calculatePeriodGarmanKlass(history, period.days); volatility != 0 {
				results[period.name] = volatility
			}
		}
	}

	return results
}

func calculatePeriodGarmanKlass(history tradier.QuoteHistory, days int) float64 {
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

	return calculateGarmanKlass(opens, highs, lows, closes)
}

func calculateGarmanKlass(opens, highs, lows, closes []float64) float64 {
	n := len(opens)
	if n == 0 || n != len(highs) || n != len(lows) || n != len(closes) {
		return 0
	}

	sum := 0.0
	for i := 0; i < n; i++ {
		hl := 0.5 * math.Pow(math.Log(highs[i]/lows[i]), 2)
		co := (2*math.Log(2) - 1) * math.Pow(math.Log(closes[i]/opens[i]), 2)
		sum += hl - co
	}

	// Annualize the volatility
	return math.Sqrt(sum / float64(n) * 252)
}
