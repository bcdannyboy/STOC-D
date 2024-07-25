package models

import (
	"math"

	"github.com/bcdannyboy/dquant/tradier"
)

func CalculateParkinsonsVolatilities(history tradier.QuoteHistory) map[string]float64 {
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
			if parkinsons, _ := calculatePeriodMetrics(history, period.days); parkinsons != 0 {
				results[period.name] = AnnualizeParkinson(parkinsons, period.name)
			}
		}
	}

	return results
}

func calculatePeriodMetrics(history tradier.QuoteHistory, days int) (float64, float64) {
	if len(history.History.Day) < days {
		return 0, 0
	}

	highs := make([]float64, days)
	lows := make([]float64, days)
	closes := make([]float64, days)

	for i := 0; i < days; i++ {
		day := history.History.Day[len(history.History.Day)-days+i]
		highs[i] = day.High
		lows[i] = day.Low
		closes[i] = day.Close
	}

	parkinsons := calculateParkinsonsNumber(highs, lows)
	stdDev := calculateStandardDeviation(closes)

	return parkinsons, stdDev
}

func calculateParkinsonsNumber(highs, lows []float64) float64 {
	n := len(highs)
	if n == 0 || n != len(lows) {
		return 0
	}

	sum := 0.0
	for i := 0; i < n; i++ {
		logRatio := math.Log(highs[i] / lows[i])
		sum += math.Pow(logRatio, 2)
	}

	return math.Sqrt(sum / (4 * float64(n) * math.Log(2)))
}

func calculateStandardDeviation(prices []float64) float64 {
	n := len(prices)
	if n < 2 {
		return 0
	}

	// Calculate returns
	returns := make([]float64, n-1)
	for i := 1; i < n; i++ {
		returns[i-1] = math.Log(prices[i] / prices[i-1])
	}

	// Calculate mean of returns
	mean := 0.0
	for _, r := range returns {
		mean += r
	}
	mean /= float64(len(returns))

	// Calculate variance
	variance := 0.0
	for _, r := range returns {
		variance += math.Pow(r-mean, 2)
	}
	variance /= float64(len(returns) - 1)

	// Return standard deviation
	return math.Sqrt(variance)
}

// AnnualizeParkinson annualizes the Parkinson's number
func AnnualizeParkinson(parkinsonsNumber float64, period string) float64 {
	var tradingDays float64
	switch period {
	case "Last Day":
		tradingDays = 1
	case "5d", "1w":
		tradingDays = 5
	case "2w":
		tradingDays = 10
	case "1m":
		tradingDays = 21
	case "3m":
		tradingDays = 63
	case "6m":
		tradingDays = 126
	case "1y":
		tradingDays = 252
	case "3y":
		tradingDays = 756
	case "5y":
		tradingDays = 1260
	case "10y":
		tradingDays = 2520
	default:
		return parkinsonsNumber
	}

	return parkinsonsNumber * math.Sqrt(252/tradingDays)
}
