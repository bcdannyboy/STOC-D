package positions

import (
	"math"

	"github.com/bcdannyboy/dquant/tradier"
)

func CalculateGarmanKlassVolatility(history tradier.QuoteHistory) []GarmanKlassResult {
	results := []GarmanKlassResult{}

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
		if volatility := calculatePeriodGarmanKlass(history, period.days); volatility != 0 {
			results = append(results, GarmanKlassResult{
				Period:     period.name,
				Volatility: volatility,
			})
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

	return math.Sqrt(sum / float64(n))
}

func AnnualizeGarmanKlass(volatility float64, period string) float64 {
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
		return volatility
	}

	return volatility * math.Sqrt(252/tradingDays)
}
