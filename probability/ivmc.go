package probability

import (
	"math"
	"sync"

	"github.com/bcdannyboy/stocd/models"
	"github.com/bcdannyboy/stocd/tradier"
	"golang.org/x/exp/rand"
)

const (
	numSimulations = 1000
	timeSteps      = 252 // Assuming 252 trading days in a year
	numWorkers     = 32
)

var rngPool = sync.Pool{
	New: func() interface{} {
		return rand.New(rand.NewSource(uint64(rand.Int63())))
	},
}

type GlobalModels struct {
	Heston        *models.HestonModel
	Merton        *models.MertonJumpDiffusion
	Kou           *models.KouJumpDiffusion
	VarianceGamma *models.VarianceGamma
}

func MonteCarloSimulation(spread models.OptionSpread, underlyingPrice, riskFreeRate float64, daysToExpiration int, yangzhangVolatilities, rogerssatchelVolatilities map[string]float64, localVolSurface models.VolatilitySurface, history tradier.QuoteHistory, chain map[string]*tradier.OptionChain, globalModels GlobalModels, avgVol float64) models.SpreadWithProbabilities {
	shortLegVol, longLegVol := confirmVolatilities(spread, localVolSurface, daysToExpiration, yangzhangVolatilities, rogerssatchelVolatilities)

	volatilities := []VolType{
		{Name: "ShortLegVol", Vol: shortLegVol},
		{Name: "LongLegVol", Vol: longLegVol},
		{Name: "YZ_1m", Vol: yangzhangVolatilities["1m"]},
		{Name: "YZ_3m", Vol: yangzhangVolatilities["3m"]},
		{Name: "YZ_6m", Vol: yangzhangVolatilities["6m"]},
		{Name: "YZ_1y", Vol: yangzhangVolatilities["1y"]},
		{Name: "RS_1m", Vol: rogerssatchelVolatilities["1m"]},
		{Name: "RS_3m", Vol: rogerssatchelVolatilities["3m"]},
		{Name: "RS_6m", Vol: rogerssatchelVolatilities["6m"]},
		{Name: "RS_1y", Vol: rogerssatchelVolatilities["1y"]},
		{Name: "ShortLeg_AskIV", Vol: spread.ShortLeg.Option.Greeks.AskIv},
		{Name: "ShortLeg_BidIV", Vol: spread.ShortLeg.Option.Greeks.BidIv},
		{Name: "ShortLeg_MidIV", Vol: spread.ShortLeg.Option.Greeks.MidIv},
		{Name: "ShortLeg_AvgIV", Vol: (spread.ShortLeg.Option.Greeks.AskIv + spread.ShortLeg.Option.Greeks.BidIv) / 2},
		{Name: "LongLeg_AskIV", Vol: spread.LongLeg.Option.Greeks.AskIv},
		{Name: "LongLeg_BidIV", Vol: spread.LongLeg.Option.Greeks.BidIv},
		{Name: "LongLeg_MidIV", Vol: spread.LongLeg.Option.Greeks.MidIv},
		{Name: "LongLeg_AvgIV", Vol: (spread.LongLeg.Option.Greeks.AskIv + spread.LongLeg.Option.Greeks.BidIv) / 2},
		{Name: "YZ_avg", Vol: calculateAverage(yangzhangVolatilities)},
		{Name: "RS_avg", Vol: calculateAverage(rogerssatchelVolatilities)},
		{Name: "AvgYZ_RS", Vol: (calculateAverage(yangzhangVolatilities) + calculateAverage(rogerssatchelVolatilities)) / 2},
		{Name: "TotalAvgVolSurface", Vol: avgVol},
		{Name: "HestonModelVol", Vol: globalModels.Heston.V0},
	}

	totalAvg := 0.0
	for _, vol := range volatilities {
		totalAvg += vol.Vol
	}
	averageVol := totalAvg / float64(len(volatilities))
	volatilities = append(volatilities, VolType{Name: "Complete_AvgVol", Vol: averageVol})

	simulationFuncs := []struct {
		name string
		fn   func(models.OptionSpread, float64, float64, float64, int, *rand.Rand, tradier.QuoteHistory, GlobalModels) map[string]float64
	}{
		{name: "Merton", fn: simulateMertonJumpDiffusion},
		{name: "Kou", fn: simulateKouJumpDiffusion},
		{name: "Heston", fn: simulateHeston},
		{name: "VarianceGamma", fn: simulateVarianceGamma},
	}

	results := make(map[string]float64, len(volatilities)*len(simulationFuncs))
	var wg sync.WaitGroup
	var mu sync.Mutex

	semaphore := make(chan struct{}, numWorkers)

	for _, vol := range volatilities {
		for _, simFunc := range simulationFuncs {
			wg.Add(1)
			go func(volName, simName string, volatility float64, simFunc func(models.OptionSpread, float64, float64, float64, int, *rand.Rand, tradier.QuoteHistory, GlobalModels) map[string]float64) {
				defer wg.Done()
				semaphore <- struct{}{}
				defer func() { <-semaphore }()

				rng := rngPool.Get().(*rand.Rand)
				defer rngPool.Put(rng)

				probMap := simFunc(spread, underlyingPrice, riskFreeRate, volatility, daysToExpiration, rng, history, globalModels)

				mu.Lock()
				for key, value := range probMap {
					results[volName+"_"+simName+"_"+key] = value
				}
				mu.Unlock()
			}(vol.Name, simFunc.name, vol.Vol, simFunc.fn)
		}
	}

	wg.Wait()

	averageProbability := calculateAverageProbability(results)

	result := models.SpreadWithProbabilities{
		Spread: spread,
		Probability: models.ProbabilityResult{
			AverageProbability: averageProbability,
			Probabilities:      results,
		},
		MeetsRoR: true,
	}

	result.MertonParams = models.MertonParams{
		Lambda: globalModels.Merton.Lambda,
		Mu:     globalModels.Merton.Mu,
		Delta:  globalModels.Merton.Delta,
	}

	result.KouParams = models.KouParams{
		Lambda: globalModels.Kou.Lambda,
		P:      globalModels.Kou.P,
		Eta1:   globalModels.Kou.Eta1,
		Eta2:   globalModels.Kou.Eta2,
	}

	result.HestonParams = models.HestonParams{
		V0:    globalModels.Heston.V0,
		Kappa: globalModels.Heston.Kappa,
		Theta: globalModels.Heston.Theta,
		Xi:    globalModels.Heston.Xi,
		Rho:   globalModels.Heston.Rho,
	}

	result.VolatilityInfo = models.VolatilityInfo{
		ShortLegVol:        shortLegVol,
		LongLegVol:         longLegVol,
		YangZhang:          yangzhangVolatilities,
		RogersSatchel:      rogerssatchelVolatilities,
		TotalAvgVolSurface: avgVol,
		ShortLegImpliedVols: map[string]float64{
			"Bid": spread.ShortLeg.Option.Greeks.BidIv,
			"Ask": spread.ShortLeg.Option.Greeks.AskIv,
			"Mid": spread.ShortLeg.Option.Greeks.MidIv,
		},
		LongLegImpliedVols: map[string]float64{
			"Bid": spread.LongLeg.Option.Greeks.BidIv,
			"Ask": spread.LongLeg.Option.Greeks.AskIv,
			"Mid": spread.LongLeg.Option.Greeks.MidIv,
		},
		HestonVolatility: globalModels.Heston.V0,
	}

	return result
}

func simulateMertonJumpDiffusion(spread models.OptionSpread, underlyingPrice, riskFreeRate, volatility float64, daysToExpiration int, rng *rand.Rand, history tradier.QuoteHistory, globalModels GlobalModels) map[string]float64 {
	tau := float64(daysToExpiration) / 365.0

	merton := *globalModels.Merton // Create a copy of the global model
	merton.Sigma = volatility      // Use the provided volatility

	profitCount := 0

	for i := 0; i < numSimulations; i++ {
		finalPrice := merton.SimulatePrice(underlyingPrice, tau, timeSteps, rng)

		if models.IsProfitable(spread, finalPrice) {
			profitCount++
		}
	}

	return map[string]float64{
		"probability": float64(profitCount) / float64(numSimulations),
	}
}

func simulateKouJumpDiffusion(spread models.OptionSpread, underlyingPrice, riskFreeRate, volatility float64, daysToExpiration int, rng *rand.Rand, history tradier.QuoteHistory, globalModels GlobalModels) map[string]float64 {
	tau := float64(daysToExpiration) / 365.0

	kou := *globalModels.Kou // Create a copy of the global model
	kou.Sigma = volatility   // Use the provided volatility

	prices := kou.SimulatePricesBatch(underlyingPrice, tau, timeSteps, numSimulations)

	profitCount := 0
	for _, price := range prices {
		if models.IsProfitable(spread, price) {
			profitCount++
		}
	}

	return map[string]float64{
		"probability": float64(profitCount) / float64(numSimulations),
	}
}

func simulateHeston(spread models.OptionSpread, underlyingPrice, riskFreeRate, volatility float64, daysToExpiration int, rng *rand.Rand, history tradier.QuoteHistory, globalModels GlobalModels) map[string]float64 {
	tau := float64(daysToExpiration) / 365.0

	heston := *globalModels.Heston // Create a copy of the global model

	// Update the initial variance (V0) with the provided volatility
	heston.V0 = volatility * volatility

	profitCount := 0

	for i := 0; i < numSimulations; i++ {
		finalPrice := heston.SimulatePrice(underlyingPrice, riskFreeRate, tau, timeSteps)

		if models.IsProfitable(spread, finalPrice) {
			profitCount++
		}
	}

	return map[string]float64{
		"probability": float64(profitCount) / float64(numSimulations),
	}
}

func simulateVarianceGamma(spread models.OptionSpread, underlyingPrice, riskFreeRate, volatility float64, daysToExpiration int, rng *rand.Rand, history tradier.QuoteHistory, globalModels GlobalModels) map[string]float64 {
	tau := float64(daysToExpiration) / 365.0

	vg := *globalModels.VarianceGamma // Create a copy of the global model

	// Adjust the volatility parameter (Alpha) based on the provided volatility
	vg.Alpha = math.Sqrt(volatility*volatility + vg.Beta*vg.Beta)

	profitCount := 0

	for i := 0; i < numSimulations; i++ {
		finalPrice := simulateVGPrice(underlyingPrice, riskFreeRate, tau, &vg, rng)

		if models.IsProfitable(spread, finalPrice) {
			profitCount++
		}
	}

	return map[string]float64{
		"probability": float64(profitCount) / float64(numSimulations),
	}
}

func simulateVGPrice(s0, r, t float64, vg *models.VarianceGamma, rng *rand.Rand) float64 {
	gamma := math.Sqrt(vg.Alpha*vg.Alpha - vg.Beta*vg.Beta)
	omega := math.Log(1-vg.Beta*vg.Beta/(vg.Alpha*vg.Alpha)-vg.Alpha*vg.Alpha*vg.Lambda/2) / vg.Lambda

	g := sampleGamma(vg.Lambda*t, 1/vg.Lambda, rng)
	z := rng.NormFloat64()

	return s0 * math.Exp(r*t+omega*t+vg.Beta*g+math.Sqrt(g)*gamma*z)
}

// sampleGamma samples from a Gamma distribution with shape k and scale theta
func sampleGamma(k, theta float64, rng *rand.Rand) float64 {
	if k < 1 {
		// Use Johnk's algorithm for k < 1
		return sampleGammaSmallShape(k, theta, rng)
	}

	// Use Marsaglia and Tsang's method for k >= 1
	d := k - 1/3
	c := 1 / math.Sqrt(9*d)

	for {
		x := rng.NormFloat64()
		v := 1 + c*x
		v = v * v * v
		u := rng.Float64()

		if u < 1-0.0331*x*x*x*x {
			return d * v * theta
		}

		if math.Log(u) < 0.5*x*x+d*(1-v+math.Log(v)) {
			return d * v * theta
		}
	}
}

// sampleGammaSmallShape samples from a Gamma distribution with shape k < 1
func sampleGammaSmallShape(k, theta float64, rng *rand.Rand) float64 {
	for {
		u := rng.Float64()
		v := rng.Float64()
		if u <= math.E/(math.E+k) {
			x := math.Pow(v, 1/k)
			if math.Log(u) <= -x {
				return x * theta
			}
		} else {
			x := 1 - math.Log(v)
			if math.Log(u) <= math.Pow(x, k-1) {
				return x * theta
			}
		}
	}
}
