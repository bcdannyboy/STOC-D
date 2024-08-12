package models

import (
	"math"

	"github.com/bcdannyboy/dquant/tradier"
)

func CalculateYangZhangVolatility(history tradier.QuoteHistory) map[string]float64 {
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
			if volatility := calculatePeriodYangZhang(history, period.days); volatility != 0 {
				results[period.name] = volatility
			}
		}
	}

	return results
}

func calculatePeriodYangZhang(history tradier.QuoteHistory, days int) float64 {
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

	return calculateYangZhang(opens, highs, lows, closes)
}

func calculateYangZhang(opens, highs, lows, closes []float64) float64 {
	n := len(opens)
	if n == 0 || n != len(highs) || n != len(lows) || n != len(closes) {
		return 0
	}

	k := 0.34 / (1.34 + (float64(n)+1)/(float64(n)-1))
	overNightVol := calculateOverNightVolatility(closes, opens, n)
	openCloseVol := calculateOpenCloseVolatility(opens, closes, n)
	rsVol := calculateRogersSatchellVolatility(opens, highs, lows, closes)

	yzVol := math.Sqrt(overNightVol + k*openCloseVol + (1-k)*rsVol)

	// Annualize the volatility
	return yzVol * math.Sqrt(252)
}

func calculateOverNightVolatility(closes, opens []float64, n int) float64 {
	sum := 0.0
	mean := 0.0
	for i := 1; i < n; i++ {
		logReturn := math.Log(opens[i] / closes[i-1])
		mean += logReturn
		sum += logReturn * logReturn
	}
	mean /= float64(n - 1)
	return (sum/float64(n-1) - mean*mean) * float64(n) / float64(n-1)
}

func calculateOpenCloseVolatility(opens, closes []float64, n int) float64 {
	sum := 0.0
	mean := 0.0
	for i := 0; i < n; i++ {
		logReturn := math.Log(closes[i] / opens[i])
		mean += logReturn
		sum += logReturn * logReturn
	}
	mean /= float64(n)
	return (sum/float64(n) - mean*mean) * float64(n) / float64(n-1)
}

func calculateRogersSatchellVolatility(opens, highs, lows, closes []float64) float64 {
	n := len(opens)
	if n == 0 || n != len(highs) || n != len(lows) || n != len(closes) {
		return 0
	}

	sum := 0.0
	for i := 0; i < n; i++ {
		sum += math.Log(highs[i]/closes[i])*math.Log(highs[i]/opens[i]) +
			math.Log(lows[i]/closes[i])*math.Log(lows[i]/opens[i])
	}

	return sum / float64(n)
}
