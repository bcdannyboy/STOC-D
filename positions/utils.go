package positions

import (
	"log"
	"math"
	"time"

	"github.com/bcdannyboy/dquant/models"
	"github.com/bcdannyboy/dquant/tradier"
	"github.com/shirou/gopsutil/cpu"
)

func monitorCPUUsage() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		var cpuUsage float64
		percentage, err := cpu.Percent(time.Second, false)
		if err == nil && len(percentage) > 0 {
			cpuUsage = percentage[0]
		}
		log.Printf("CPU Usage: %.2f%%", cpuUsage)
	}
}

func estimateTotalJobs(chain map[string]*tradier.OptionChain) int64 {
	total := int64(0)
	for _, expiration := range chain {
		n := int64(len(expiration.Options.Option))
		total += (n * (n - 1)) / 2
	}
	return total
}

func calculateIntrinsicValue(shortLeg, longLeg models.SpreadLeg, underlyingPrice float64, spreadType string) float64 {
	if spreadType == "Bull Put" {
		return math.Max(0, shortLeg.Option.Strike-longLeg.Option.Strike-(shortLeg.Option.Strike-underlyingPrice))
	} else { // Bear Call
		return math.Max(0, longLeg.Option.Strike-shortLeg.Option.Strike-(underlyingPrice-shortLeg.Option.Strike))
	}
}

func calculateSingleOptionIntrinsicValue(option tradier.Option, underlyingPrice float64) float64 {
	if option.OptionType == "call" {
		return math.Max(0, underlyingPrice-option.Strike)
	}
	return math.Max(0, option.Strike-underlyingPrice)
}

func sanitizeFloat(f float64) float64 {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0
	}
	return f
}

func calculateForwardImpliedVol(vol1, T1, vol2, T2 float64) float64 {
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
