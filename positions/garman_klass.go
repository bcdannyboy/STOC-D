package positions

import (
	"math"

	"github.com/bcdannyboy/dquant/models"
	"github.com/bcdannyboy/dquant/tradier"
)

func CalculateGarmanKlassVolatility(history tradier.QuoteHistory) models.GarmanKlassResult {
	// Calculate for the last day only
	if len(history.History.Day) < 1 {
		return models.GarmanKlassResult{}
	}
	lastDay := history.History.Day[len(history.History.Day)-1]
	volatility := calculateGarmanKlass([]float64{lastDay.Open}, []float64{lastDay.High}, []float64{lastDay.Low}, []float64{lastDay.Close})
	return models.GarmanKlassResult{
		Period:     "Last Day",
		Volatility: volatility,
	}
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
