package probability

import (
	"sync"

	"github.com/bcdannyboy/dquant/models"
	"github.com/bcdannyboy/dquant/tradier"
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
}

func MonteCarloSimulation(spread models.OptionSpread, underlyingPrice, riskFreeRate float64, daysToExpiration int, yangzhangVolatilities, rogerssatchelVolatilities map[string]float64, localVolSurface models.VolatilitySurface, history tradier.QuoteHistory, chain map[string]*tradier.OptionChain, globalModels GlobalModels) models.SpreadWithProbabilities {
	shortLegVol, longLegVol := confirmVolatilities(spread, localVolSurface, daysToExpiration, yangzhangVolatilities, rogerssatchelVolatilities)

	volatilities := calculateVolatilities(shortLegVol, longLegVol, daysToExpiration, yangzhangVolatilities, rogerssatchelVolatilities, localVolSurface, history, spread)

	simulationFuncs := []struct {
		name string
		fn   func(models.OptionSpread, float64, float64, float64, int, *rand.Rand, tradier.QuoteHistory, GlobalModels) map[string]float64
	}{
		{name: "Merton", fn: simulateMertonJumpDiffusion},
		{name: "Kou", fn: simulateKouJumpDiffusion},
		{name: "Heston", fn: simulateHeston},
	}

	results := make(map[string]float64, len(volatilities)*len(simulationFuncs)*5)
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

	// Store model parameters
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

	// Store volatility information
	result.VolatilityInfo = models.VolatilityInfo{
		ShortLegVol:        shortLegVol,
		LongLegVol:         longLegVol,
		YangZhang:          yangzhangVolatilities,
		RogersSatchel:      rogerssatchelVolatilities,
		TotalAvgVolSurface: volatilities[len(volatilities)-1].Vol,
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

	// Use the global Merton model but update the volatility
	merton := *globalModels.Merton // Create a copy of the global model
	merton.Sigma = volatility      // Update the volatility for this specific spread

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

	// Use the global Kou model but update the volatility
	kou := *globalModels.Kou // Create a copy of the global model
	kou.Sigma = volatility   // Update the volatility for this specific spread

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

	// Use the global Heston model
	heston := *globalModels.Heston // Create a copy of the global model

	// Update the initial variance (V0) with the spread-specific volatility
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
