package positions

import (
	"math"

	"github.com/bcdannyboy/dquant/tradier"
)

func CalculateParkinsonsMetrics(history tradier.QuoteHistory) []ParkinsonsResult {
	results := []ParkinsonsResult{}

	periods := []struct {
		name string
		days int
	}{
		{"Last Day", 1},
		{"period_5d", 5},
		{"period_1w", 5},
		{"period_2w", 10},
		{"period_1m", 21},
		{"period_3m", 63},
		{"period_6m", 126},
		{"period_1y", 252},
		{"period_3y", 756},
		{"period_5y", 1260},
		{"period_10y", 2520},
	}

	for _, period := range periods {
		if parkinsons, stdDev := calculatePeriodMetrics(history, period.days); parkinsons != 0 {
			results = append(results, ParkinsonsResult{
				Period:            period.name,
				ParkinsonsNumber:  parkinsons,
				StandardDeviation: stdDev,
				Difference:        parkinsons - (1.67 * stdDev),
			})
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
	case "period_5d", "period_1w":
		tradingDays = 5
	case "period_2w":
		tradingDays = 10
	case "period_1m":
		tradingDays = 21
	case "period_3m":
		tradingDays = 63
	case "period_6m":
		tradingDays = 126
	case "period_1y":
		tradingDays = 252
	case "period_3y":
		tradingDays = 756
	case "period_5y":
		tradingDays = 1260
	case "period_10y":
		tradingDays = 2520
	default:
		return parkinsonsNumber
	}

	return parkinsonsNumber * math.Sqrt(252/tradingDays)
}

// AnnualizeStandardDeviation annualizes the standard deviation
func AnnualizeStandardDeviation(stdDev float64, period string) float64 {
	var tradingDays float64
	switch period {
	case "Last Day":
		tradingDays = 1
	case "period_5d", "period_1w":
		tradingDays = 5
	case "period_2w":
		tradingDays = 10
	case "period_1m":
		tradingDays = 21
	case "period_3m":
		tradingDays = 63
	case "period_6m":
		tradingDays = 126
	case "period_1y":
		tradingDays = 252
	case "period_3y":
		tradingDays = 756
	case "period_5y":
		tradingDays = 1260
	case "period_10y":
		tradingDays = 2520
	default:
		return stdDev
	}

	return stdDev * math.Sqrt(252/tradingDays)
}
