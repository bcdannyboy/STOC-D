package probability

import (
	"math"
	"time"

	"github.com/bcdannyboy/dquant/models"
)

func calculateForwardImpliedVol(vol1, T1, vol2, T2 float64) float64 {
	if T2 <= T1 || T1 <= 0 || T2 <= 0 {
		return 0
	}
	return math.Sqrt((vol2*vol2*T2 - vol1*vol1*T1) / (T2 - T1))
}

func calculateCombinedForwardImpliedVol(shortLeg, longLeg models.SpreadLeg) float64 {
	T1 := calculateTimeToMaturity(shortLeg.Option.ExpirationDate)
	T2 := calculateTimeToMaturity(longLeg.Option.ExpirationDate)
	return calculateForwardImpliedVol(shortLeg.BSMResult.ImpliedVolatility, T1, longLeg.BSMResult.ImpliedVolatility, T2)
}

func calculateTimeToMaturity(expirationDate string) float64 {
	expDate, _ := time.Parse("2006-01-02", expirationDate)
	now := time.Now()
	return expDate.Sub(now).Hours() / 24 / 365 // Convert to years
}
