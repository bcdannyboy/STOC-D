package probability

import (
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
	Heston *models.HestonModel
	Merton *models.MertonJumpDiffusion
	Kou    *models.KouJumpDiffusion
	CGMY   *models.CGMYModel
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
		{name: "CGMY", fn: simulateCGMY},
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

	result.CGMYParams = models.CGMYModel{
		C: globalModels.CGMY.C,
		G: globalModels.CGMY.G,
		M: globalModels.CGMY.M,
		Y: globalModels.CGMY.Y,
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
		finalPrice := merton.SimulatePrice(underlyingPrice, riskFreeRate, tau, timeSteps, rng)

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

	profitCount := 0

	for i := 0; i < numSimulations; i++ {
		finalPrice := kou.SimulatePrice(underlyingPrice, riskFreeRate, tau, timeSteps, rng)

		if models.IsProfitable(spread, finalPrice) {
			profitCount++
		}
	}

	return map[string]float64{
		"probability": float64(profitCount) / float64(numSimulations),
	}
}

func simulateHeston(spread models.OptionSpread, underlyingPrice, riskFreeRate, volatility float64, daysToExpiration int, rng *rand.Rand, history tradier.QuoteHistory, globalModels GlobalModels) map[string]float64 {
	tau := float64(daysToExpiration) / 365.0

	heston := *globalModels.Heston      // Create a copy of the global model
	heston.V0 = volatility * volatility // Set initial variance to square of volatility

	profitCount := 0

	for i := 0; i < numSimulations; i++ {
		finalPrice := heston.SimulatePrice(underlyingPrice, riskFreeRate, tau, timeSteps, rng)

		if models.IsProfitable(spread, finalPrice) {
			profitCount++
		}
	}

	return map[string]float64{
		"probability": float64(profitCount) / float64(numSimulations),
	}
}

func simulateCGMY(spread models.OptionSpread, underlyingPrice, riskFreeRate, volatility float64, daysToExpiration int, rng *rand.Rand, history tradier.QuoteHistory, globalModels GlobalModels) map[string]float64 {
	tau := float64(daysToExpiration) / 365.0

	cgmy := *globalModels.CGMY // Create a copy of the global model

	profitCount := 0

	// Generate volatilities for each time step
	volatilities := make([]float64, timeSteps)
	for i := 0; i < timeSteps; i++ {
		volatilities[i] = volatility
	}

	for i := 0; i < numSimulations; i++ {
		finalPrice := cgmy.SimulatePrice(underlyingPrice, riskFreeRate, tau, timeSteps, rng, volatilities)

		if models.IsProfitable(spread, finalPrice) {
			profitCount++
		}
	}

	probability := float64(profitCount) / float64(numSimulations)

	return map[string]float64{
		"probability": probability,
	}
}
