package probability

import (
	"math"
	"strings"
	"sync"

	"github.com/bcdannyboy/stocd/models"
	"github.com/bcdannyboy/stocd/tradier"
	"golang.org/x/exp/rand"
)

const (
	numSimulations = 1000
	timeSteps      = 252 // Assuming 252 trading days in a year
	numWorkers     = 100
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
	CGMY   *models.CGMYProcess
}

func MonteCarloSimulation(spread models.OptionSpread, underlyingPrice, riskFreeRate float64, daysToExpiration int, yangzhangVolatilities, rogerssatchelVolatilities map[string]float64, localVolSurface models.VolatilitySurface, history tradier.QuoteHistory, chain map[string]*tradier.OptionChain, globalModels GlobalModels, avgVol float64) models.SpreadWithProbabilities {
	shortLegVol, longLegVol := confirmVolatilities(spread, localVolSurface, daysToExpiration, yangzhangVolatilities, rogerssatchelVolatilities)

	shortLegLiquidity := calculateLiquidity(spread.ShortLeg.Option)
	longLegLiquidity := calculateLiquidity(spread.LongLeg.Option)
	spreadLiquidity := (shortLegLiquidity + longLegLiquidity) / 2

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
		fn   func(models.OptionSpread, float64, float64, float64, int, *rand.Rand, tradier.QuoteHistory, GlobalModels, bool) (map[string]float64, []float64)
	}{
		{name: "CGMY_Heston", fn: simulateCGMY},
		{name: "Merton_Heston", fn: simulateMertonJumpDiffusion},
		{name: "Kou_Heston", fn: simulateKouJumpDiffusion},
	}

	results := make(map[string]float64, len(volatilities)*len(simulationFuncs))
	var wg sync.WaitGroup
	var mu sync.Mutex

	semaphore := make(chan struct{}, numWorkers)
	var finalPrices []float64

	for _, vol := range volatilities {
		for _, simFunc := range simulationFuncs {
			wg.Add(1)
			go func(volName, simName string, volatility float64, simFunc func(models.OptionSpread, float64, float64, float64, int, *rand.Rand, tradier.QuoteHistory, GlobalModels, bool) (map[string]float64, []float64)) {
				defer wg.Done()
				semaphore <- struct{}{}
				defer func() { <-semaphore }()

				rng := rngPool.Get().(*rand.Rand)
				defer rngPool.Put(rng)

				useHeston := strings.HasSuffix(simName, "Heston")
				probMap, prices := simFunc(spread, underlyingPrice, riskFreeRate, volatility, daysToExpiration, rng, history, globalModels, useHeston)

				mu.Lock()
				for key, value := range probMap {
					results[volName+"_"+simName+"_"+key] = value
				}
				finalPrices = append(finalPrices, prices...)
				mu.Unlock()
			}(vol.Name, simFunc.name, vol.Vol, simFunc.fn)
		}
	}

	wg.Wait()

	// Calculate VaR and Expected Shortfall
	var95 := calculateVaR(spread, finalPrices, 0.95)
	var99 := calculateVaR(spread, finalPrices, 0.99)
	es := calculateExpectedShortfall(spread, finalPrices, 0.95)

	averageProbability := calculateAverageProbability(results)

	result := models.SpreadWithProbabilities{
		Spread:            spread,
		VaR95:             var95,
		VaR99:             var99,
		ExpectedShortfall: es,
		Liquidity:         spreadLiquidity,
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

	result.CGMYParams = models.CGMYParams{
		C: globalModels.CGMY.Params.C,
		G: globalModels.CGMY.Params.G,
		M: globalModels.CGMY.Params.M,
		Y: globalModels.CGMY.Params.Y,
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

func simulateMertonJumpDiffusion(spread models.OptionSpread, underlyingPrice, riskFreeRate, volatility float64, daysToExpiration int, rng *rand.Rand, history tradier.QuoteHistory, globalModels GlobalModels, useHeston bool) (map[string]float64, []float64) {
	tau := float64(daysToExpiration) / 365.0

	merton := *globalModels.Merton // Create a copy of the global model
	merton.Sigma = volatility      // Use the provided volatility

	profitCount := 0
	finalPrices := make([]float64, numSimulations)

	for i := 0; i < numSimulations; i++ {
		var finalPrice float64
		if useHeston {
			volPath := simulateHestonVolPath(globalModels.Heston, volatility, tau, timeSteps, rng)
			finalPrice = simulateMertonPriceWithHestonVol(underlyingPrice, riskFreeRate, tau, timeSteps, rng, merton, volPath)
		} else {
			finalPrice = merton.SimulatePrice(underlyingPrice, riskFreeRate, tau, timeSteps, rng)
		}
		finalPrices[i] = finalPrice

		if models.IsProfitable(spread, finalPrice) {
			profitCount++
		}
	}

	return map[string]float64{
		"probability": float64(profitCount) / float64(numSimulations),
	}, finalPrices
}

func simulateKouJumpDiffusion(spread models.OptionSpread, underlyingPrice, riskFreeRate, volatility float64, daysToExpiration int, rng *rand.Rand, history tradier.QuoteHistory, globalModels GlobalModels, useHeston bool) (map[string]float64, []float64) {
	tau := float64(daysToExpiration) / 365.0

	kou := *globalModels.Kou // Create a copy of the global model
	kou.Sigma = volatility   // Use the provided volatility
	kou.R = riskFreeRate     // Set the risk-free rate

	profitCount := 0
	finalPrices := make([]float64, numSimulations)

	for i := 0; i < numSimulations; i++ {
		var finalPrice float64
		if useHeston {
			volPath := simulateHestonVolPath(globalModels.Heston, volatility, tau, timeSteps, rng)
			finalPrice = simulateKouPriceWithHestonVol(underlyingPrice, riskFreeRate, tau, timeSteps, rng, kou, volPath)
		} else {
			finalPrice = kou.SimulatePrice(underlyingPrice, riskFreeRate, tau, timeSteps, rng)
		}
		finalPrices[i] = finalPrice

		if models.IsProfitable(spread, finalPrice) {
			profitCount++
		}
	}

	return map[string]float64{
		"probability": float64(profitCount) / float64(numSimulations),
	}, finalPrices
}

func simulateCGMY(spread models.OptionSpread, underlyingPrice, riskFreeRate, volatility float64, daysToExpiration int, rng *rand.Rand, history tradier.QuoteHistory, globalModels GlobalModels, useHeston bool) (map[string]float64, []float64) {
	tau := float64(daysToExpiration) / 365.0

	cgmy := *globalModels.CGMY // Create a copy of the global model

	// Adjust CGMY parameters based on the provided volatility
	currentVol := math.Sqrt(cgmy.Params.C * math.Gamma(2-cgmy.Params.Y) * (1/math.Pow(cgmy.Params.M, 2-cgmy.Params.Y) + 1/math.Pow(cgmy.Params.G, 2-cgmy.Params.Y)))
	volAdjustment := volatility / currentVol
	cgmy.Params.C *= math.Pow(volAdjustment, 2)

	profitCount := 0
	finalPrices := make([]float64, numSimulations)

	for i := 0; i < numSimulations; i++ {
		path := cgmy.SimulatePath(tau, tau/float64(timeSteps), rng)
		var finalPrice float64
		if useHeston {
			volPath := simulateHestonVolPath(globalModels.Heston, volatility, tau, timeSteps, rng)
			finalPrice = simulateCGMYPriceWithHestonVol(underlyingPrice, riskFreeRate, tau, path, volPath)
		} else {
			finalPrice = underlyingPrice * math.Exp(path[len(path)-1])
		}
		finalPrices[i] = finalPrice

		if models.IsProfitable(spread, finalPrice) {
			profitCount++
		}
	}

	return map[string]float64{
		"probability": float64(profitCount) / float64(numSimulations),
	}, finalPrices
}

func simulateHestonVolPath(heston *models.HestonModel, initialVol, T float64, steps int, rng *rand.Rand) []float64 {
	dt := T / float64(steps)
	sqrtDt := math.Sqrt(dt)
	volPath := make([]float64, steps+1)
	volPath[0] = initialVol * initialVol // Heston model uses variance, not volatility

	for i := 0; i < steps; i++ {
		dW := rng.NormFloat64() * sqrtDt
		volPath[i+1] = volPath[i] + heston.Kappa*(heston.Theta-volPath[i])*dt + heston.Xi*math.Sqrt(volPath[i])*dW
		volPath[i+1] = math.Max(0, volPath[i+1]) // Ensure non-negative variance
	}

	// Convert variance path to volatility path
	for i := range volPath {
		volPath[i] = math.Sqrt(volPath[i])
	}

	return volPath
}

func simulateMertonPriceWithHestonVol(S0, r, T float64, steps int, rng *rand.Rand, merton models.MertonJumpDiffusion, volPath []float64) float64 {
	dt := T / float64(steps)
	price := S0

	for i := 0; i < steps; i++ {
		dW := rng.NormFloat64() * math.Sqrt(dt)
		jump := 0.0
		if rng.Float64() < merton.Lambda*dt {
			jump = rng.NormFloat64()*merton.Delta + merton.Mu
		}
		price *= math.Exp((r-0.5*volPath[i]*volPath[i])*dt + volPath[i]*dW + jump)
	}

	return price
}

func simulateKouPriceWithHestonVol(S0, r, T float64, steps int, rng *rand.Rand, kou models.KouJumpDiffusion, volPath []float64) float64 {
	dt := T / float64(steps)
	price := S0

	for i := 0; i < steps; i++ {
		dW := rng.NormFloat64() * math.Sqrt(dt)
		diffusion := math.Exp((r-0.5*volPath[i]*volPath[i])*dt + volPath[i]*dW)

		if rng.Float64() < kou.Lambda*dt {
			var jump float64
			if rng.Float64() < kou.P {
				jump = math.Exp(rng.ExpFloat64() / kou.Eta1)
			} else {
				jump = math.Exp(-rng.ExpFloat64() / kou.Eta2)
			}
			price *= diffusion * jump
		} else {
			price *= diffusion
		}
	}

	return price
}

func simulateCGMYPriceWithHestonVol(S0, r, T float64, cgmyPath []float64, volPath []float64) float64 {
	steps := len(cgmyPath) - 1
	dt := T / float64(steps)
	price := S0

	for i := 0; i < steps; i++ {
		price *= math.Exp((r-0.5*volPath[i]*volPath[i])*dt + cgmyPath[i+1] - cgmyPath[i])
	}

	return price
}
