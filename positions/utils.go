package positions

import (
	"log"
	"math"
	"time"

	"github.com/bcdannyboy/dquant/models"
	"github.com/bcdannyboy/dquant/tradier"
	"github.com/shirou/gopsutil/cpu"
	"gonum.org/v1/gonum/optimize"
	"gonum.org/v1/gonum/stat"
	"gonum.org/v1/gonum/stat/distuv"
)

func calculateHestonParameters(history tradier.QuoteHistory, S0, r float64) models.HestonParameters {
	startTime := time.Now()
	log.Printf("calculateHestonParameters started at %v", startTime)
	returns := calculateLogReturns(history)

	// Initial parameter guess
	initialParams := []float64{
		0.04, // v0: initial variance
		2.0,  // kappa: mean reversion speed
		0.04, // theta: long-term variance
		0.3,  // xi: volatility of variance
		-0.7, // rho: correlation
	}

	problem := optimize.Problem{
		Func: func(params []float64) float64 {
			return -logLikelihood(returns, params[0], params[1], params[2], params[3], params[4], r)
		},
	}

	result, err := optimize.Minimize(problem, initialParams, nil, &optimize.NelderMead{})
	if err != nil {
		log.Printf("Error in Heston parameter optimization: %v", err)
		return defaultHestonParameters(returns)
	}

	optimizedParams := result.X
	log.Printf("Optimized Heston parameters in %v: %v", time.Since(startTime), optimizedParams)
	return models.HestonParameters{
		V0:    math.Max(0.0001, optimizedParams[0]),
		Kappa: math.Max(0.001, optimizedParams[1]),
		Theta: math.Max(0.0001, optimizedParams[2]),
		Xi:    math.Max(0.001, optimizedParams[3]),
		Rho:   math.Max(-0.99, math.Min(0.99, optimizedParams[4])),
	}
}

func calculateLogReturns(history tradier.QuoteHistory) []float64 {
	returns := make([]float64, len(history.History.Day)-1)
	for i := 1; i < len(history.History.Day); i++ {
		returns[i-1] = math.Log(history.History.Day[i].Close / history.History.Day[i-1].Close)
	}
	return returns
}

func logLikelihood(returns []float64, v0, kappa, theta, xi, rho, r float64) float64 {
	dt := 1.0 / 252.0 // Assuming daily returns
	logLik := 0.0

	v := v0
	stdNormal := distuv.Normal{Mu: 0, Sigma: 1}

	for _, ret := range returns {
		mu := r - 0.5*v
		sigma := math.Sqrt(v)

		zScore := (ret - mu) / sigma
		logLik += -0.5*math.Log(2*math.Pi*v) - 0.5*zScore*zScore

		// Update v for the next iteration
		v += kappa*(theta-v)*dt + xi*math.Sqrt(v*dt)*(rho*zScore+math.Sqrt(1-rho*rho)*stdNormal.Rand())
		v = math.Max(0.0001, v) // Ensure variance doesn't go negative
	}

	return logLik
}

func defaultHestonParameters(returns []float64) models.HestonParameters {
	variance := stat.Variance(returns, nil)
	return models.HestonParameters{
		V0:    variance,
		Kappa: 2.0,
		Theta: variance,
		Xi:    0.1,
		Rho:   -0.7,
	}
}

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
	if T2 > T1 && T1 > 0 { // Ensure T2 > T1 and both are greater than zero
		return calculateForwardImpliedVol(shortLeg.BSMResult.ImpliedVolatility, T1, longLeg.BSMResult.ImpliedVolatility, T2)
	}
	return 0
}

func calculateTimeToMaturity(expirationDate string) float64 {
	expDate, _ := time.Parse("2006-01-02", expirationDate)
	now := time.Now()
	return expDate.Sub(now).Hours() / 24 / 365 // Convert to years
}
