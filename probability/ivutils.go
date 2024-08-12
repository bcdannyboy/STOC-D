package probability

import (
	"math"
	"time"

	"github.com/bcdannyboy/dquant/models"
	"github.com/bcdannyboy/dquant/tradier"
)

func confirmVolatilities(spread models.OptionSpread, localVolSurface models.VolatilitySurface, daysToExpiration int, gkVolatilities, parkinsonVolatilities map[string]float64) (float64, float64) {
	// Convert ExpirationDate strings to time.Time and calculate time to expiry
	shortLegExpiration, _ := time.Parse("2006-01-02", spread.ShortLeg.Option.ExpirationDate)
	longLegExpiration, _ := time.Parse("2006-01-02", spread.LongLeg.Option.ExpirationDate)

	shortTimeToExpiry := shortLegExpiration.Sub(time.Now()).Hours() / 24 / 365
	longTimeToExpiry := longLegExpiration.Sub(time.Now()).Hours() / 24 / 365

	// Use the local volatility surface to get the volatilities for the strikes and dates in the spread
	shortLegVol := interpolateVolatilityFromSurface(localVolSurface, spread.ShortLeg.Option.Strike, shortTimeToExpiry)
	longLegVol := interpolateVolatilityFromSurface(localVolSurface, spread.LongLeg.Option.Strike, longTimeToExpiry)

	// Incorporate bid, ask, and mid IVs from the leg options
	if spread.ShortLeg.Option.Greeks.BidIv > 0 {
		shortLegVol = (shortLegVol + spread.ShortLeg.Option.Greeks.BidIv) / 2
	}
	if spread.ShortLeg.Option.Greeks.AskIv > 0 {
		shortLegVol = (shortLegVol + spread.ShortLeg.Option.Greeks.AskIv) / 2
	}
	if spread.ShortLeg.Option.Greeks.MidIv > 0 {
		shortLegVol = (shortLegVol + spread.ShortLeg.Option.Greeks.MidIv) / 2
	}

	if spread.LongLeg.Option.Greeks.BidIv > 0 {
		longLegVol = (longLegVol + spread.LongLeg.Option.Greeks.BidIv) / 2
	}
	if spread.LongLeg.Option.Greeks.AskIv > 0 {
		longLegVol = (longLegVol + spread.LongLeg.Option.Greeks.AskIv) / 2
	}
	if spread.LongLeg.Option.Greeks.MidIv > 0 {
		longLegVol = (longLegVol + spread.LongLeg.Option.Greeks.MidIv) / 2
	}

	return shortLegVol, longLegVol
}

func calculateVolatilities(shortLegVol, longLegVol float64, daysToExpiration int, gkVolatilities, parkinsonVolatilities map[string]float64, localVolSurface models.VolatilitySurface, history tradier.QuoteHistory) []VolType {
	volatilities := []VolType{}

	volatilities = append(volatilities, VolType{Name: "ShortLegVol", Vol: shortLegVol})

	volatilities = append(volatilities, VolType{Name: "LongLegVol", Vol: longLegVol})

	volatilities = append(volatilities, VolType{Name: "combined_forward_vol", Vol: math.Sqrt((shortLegVol*shortLegVol*float64(daysToExpiration)/365 + longLegVol*longLegVol*float64(daysToExpiration)/365) / 2)})

	// Include Garman-Klass and Parkinson volatilities
	for period, vol := range gkVolatilities {
		volatilities = append(volatilities, VolType{Name: "GarmanKlassIV_" + period, Vol: vol})
	}

	avgGK := calculateAverage(gkVolatilities)
	volatilities = append(volatilities, VolType{Name: "avg_GarmanKlassIV", Vol: avgGK})

	for period, vol := range parkinsonVolatilities {
		volatilities = append(volatilities, VolType{Name: "ParkinsonVolatility_" + period, Vol: vol})
	}

	avgParkinson := calculateAverage(parkinsonVolatilities)
	volatilities = append(volatilities, VolType{Name: "avg_ParkinsonVolatility", Vol: avgParkinson})

	// Calculate total average volatility surface ignoring positions with no volume or no volatility
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
			// Ignore positions with no volume or no volatility
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
