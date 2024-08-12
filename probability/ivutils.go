package probability

import (
	"math"
	"time"

	"github.com/bcdannyboy/dquant/models"
	"github.com/bcdannyboy/dquant/tradier"
)

func confirmVolatilities(spread models.OptionSpread, localVolSurface models.VolatilitySurface, daysToExpiration int, gkVolatilities, parkinsonVolatilities map[string]float64) (float64, float64) {
	shortLegExpiration, _ := time.Parse("2006-01-02", spread.ShortLeg.Option.ExpirationDate)
	longLegExpiration, _ := time.Parse("2006-01-02", spread.LongLeg.Option.ExpirationDate)

	shortTimeToExpiry := shortLegExpiration.Sub(time.Now()).Hours() / 24 / 365
	longTimeToExpiry := longLegExpiration.Sub(time.Now()).Hours() / 24 / 365

	shortLegVol := interpolateVolatilityFromSurface(localVolSurface, spread.ShortLeg.Option.Strike, shortTimeToExpiry)
	longLegVol := interpolateVolatilityFromSurface(localVolSurface, spread.LongLeg.Option.Strike, longTimeToExpiry)

	shortLegVol = incorporateOptionIVs(shortLegVol, spread.ShortLeg.Option)
	longLegVol = incorporateOptionIVs(longLegVol, spread.LongLeg.Option)

	return shortLegVol, longLegVol
}

func incorporateOptionIVs(baseVol float64, option tradier.Option) float64 {
	count := 1.0
	totalVol := baseVol

	if option.Greeks.BidIv > 0 {
		totalVol += option.Greeks.BidIv
		count++
	}
	if option.Greeks.AskIv > 0 {
		totalVol += option.Greeks.AskIv
		count++
	}
	if option.Greeks.MidIv > 0 {
		totalVol += option.Greeks.MidIv
		count++
	}

	return totalVol / count
}

func calculateVolatilities(shortLegVol, longLegVol float64, daysToExpiration int, gkVolatilities, parkinsonVolatilities map[string]float64, localVolSurface models.VolatilitySurface, history tradier.QuoteHistory, spread models.OptionSpread) []VolType {
	volatilities := []VolType{
		{Name: "ShortLegVol", Vol: shortLegVol},
		{Name: "LongLegVol", Vol: longLegVol},
		{Name: "ShortLeg_BidIV", Vol: spread.ShortLeg.Option.Greeks.BidIv},
		{Name: "ShortLeg_AskIV", Vol: spread.ShortLeg.Option.Greeks.AskIv},
		{Name: "ShortLeg_MidIV", Vol: spread.ShortLeg.Option.Greeks.MidIv},
		{Name: "LongLeg_BidIV", Vol: spread.LongLeg.Option.Greeks.BidIv},
		{Name: "LongLeg_AskIV", Vol: spread.LongLeg.Option.Greeks.AskIv},
		{Name: "LongLeg_MidIV", Vol: spread.LongLeg.Option.Greeks.MidIv},
	}

	yang_zhang := models.CalculateYangZhangVolatility(history)
	rogers_satchell := models.CalculateRogersSatchellVolatility(history)

	for period, vol := range yang_zhang {
		volatilities = append(volatilities, VolType{Name: "YangZhangIV_" + period, Vol: vol})
	}
	for period, vol := range rogers_satchell {
		volatilities = append(volatilities, VolType{Name: "RogersSatchellIV_" + period, Vol: vol})
	}

	avgYZ := calculateAverage(yang_zhang)
	volatilities = append(volatilities, VolType{Name: "avg_YangZhangIV", Vol: avgYZ})

	avgRS := calculateAverage(rogers_satchell)
	volatilities = append(volatilities, VolType{Name: "avg_RogersSatchellIV", Vol: avgRS})

	totalVolatilitySurface := calculateTotalAverageVolatilitySurface(localVolSurface, history)
	volatilities = append(volatilities, VolType{Name: "total_avg_volatility_surface", Vol: totalVolatilitySurface})

	return volatilities
}

func interpolateVolatilityFromSurface(surface models.VolatilitySurface, strike, timeToExpiry float64) float64 {
	return models.InterpolateVolatility(surface, strike, timeToExpiry)
}

func calculateTotalAverageVolatilitySurface(surface models.VolatilitySurface, history tradier.QuoteHistory) float64 {
	totalVol := 0.0
	count := 0

	for _, volList := range surface.Vols {
		for i, vol := range volList {
			if vol == 0 || history.History.Day[i].Volume == 0 {
				continue
			}
			totalVol += vol
			count++
		}
	}

	if count == 0 {
		return 0
	}

	return totalVol / float64(count)
}

func calculateAverageProbability(results map[string]float64) float64 {
	var sum float64
	var count int64
	for _, value := range results {
		sum += value
		count++
	}
	return sum / float64(count)
}

func calculateAverage(volatilities map[string]float64) float64 {
	total := 0.0
	for _, vol := range volatilities {
		total += vol
	}
	return total / float64(len(volatilities))
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
