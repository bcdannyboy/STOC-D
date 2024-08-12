package positions

import (
	"fmt"
	"log"
	"math"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/bcdannyboy/dquant/models"
	"github.com/bcdannyboy/dquant/probability"
	"github.com/bcdannyboy/dquant/tradier"
)

const (
	workerPoolSize = 1000
)

func IdentifySpreads(chain map[string]*tradier.OptionChain, underlyingPrice, riskFreeRate float64, history tradier.QuoteHistory, minReturnOnRisk float64, currentDate time.Time, spreadType string) []models.SpreadWithProbabilities {
	startTime := time.Now()
	log.Printf("IdentifySpreads started at %v", startTime)

	if len(chain) == 0 {
		fmt.Printf("Warning: Option chain is empty for %s spreads\n", spreadType)
		return nil
	}

	fmt.Printf("Identifying %s Spreads for underlying price: %.2f, Risk-Free Rate: %.4f, Min Return on Risk: %.4f\n", spreadType, underlyingPrice, riskFreeRate, minReturnOnRisk)

	gkVolatilities := models.CalculateGarmanKlassVolatilities(history)
	parkinsonVolatilities := models.CalculateParkinsonsVolatilities(history)
	localVolSurface := models.CalculateLocalVolatilitySurface(chain, underlyingPrice)

	numCPU := runtime.NumCPU()
	runtime.GOMAXPROCS(numCPU)
	fmt.Printf("Using %d CPUs\n", numCPU)

	totalJobs := calculateTotalJobs(chain, spreadType)
	fmt.Printf("Total spreads to process: %d\n", totalJobs)

	log.Printf("Starting processChainOptimized at %v", time.Now())
	spreads := processChainOptimized(chain, underlyingPrice, riskFreeRate, gkVolatilities, parkinsonVolatilities, localVolSurface, minReturnOnRisk, currentDate, spreadType, totalJobs, history)
	log.Printf("Finished processChainOptimized at %v", time.Now())

	log.Printf("Sorting %d spreads by highest probability", len(spreads))
	sort.Slice(spreads, func(i, j int) bool {
		return spreads[i].Probability.AverageProbability > spreads[j].Probability.AverageProbability
	})

	fmt.Printf("\nProcessing complete. Total time: %v\n", time.Since(startTime))
	fmt.Printf("Identified %d %s Spreads meeting criteria\n", len(spreads), spreadType)

	for i, spread := range spreads {
		fmt.Printf("\nSpread %d:\n", i+1)
		fmt.Printf("  Short Leg: %s, Long Leg: %s\n", spread.Spread.ShortLeg.Option.Symbol, spread.Spread.LongLeg.Option.Symbol)
		fmt.Printf("  Spread Credit: %.2f, ROR: %.2f%%\n", spread.Spread.SpreadCredit, spread.Spread.ROR*100)
		fmt.Printf("  Probability of Profit: %.2f%%\n", spread.Probability.AverageProbability*100)

		fmt.Printf("  Merton Model Parameters:\n")
		fmt.Printf("    Lambda: %.4f, Mu: %.4f, Delta: %.4f\n", spread.MertonParams.Lambda, spread.MertonParams.Mu, spread.MertonParams.Delta)

		fmt.Printf("  Kou Model Parameters:\n")
		fmt.Printf("    Lambda: %.4f, P: %.4f, Eta1: %.4f, Eta2: %.4f\n", spread.KouParams.Lambda, spread.KouParams.P, spread.KouParams.Eta1, spread.KouParams.Eta2)

		fmt.Printf("  Volatility Information:\n")
		fmt.Printf("    Short Leg Vol: %.4f, Long Leg Vol: %.4f\n", spread.VolatilityInfo.ShortLegVol, spread.VolatilityInfo.LongLegVol)
		fmt.Printf("    Total Avg Vol Surface: %.4f\n", spread.VolatilityInfo.TotalAvgVolSurface)

		fmt.Printf("    Garman-Klass Volatilities:\n")
		for period, vol := range spread.VolatilityInfo.GarmanKlassVols {
			fmt.Printf("      %s: %.4f\n", period, vol)
		}

		fmt.Printf("    Parkinson Volatilities:\n")
		for period, vol := range spread.VolatilityInfo.ParkinsonVols {
			fmt.Printf("      %s: %.4f\n", period, vol)
		}

		fmt.Printf("    Short Leg Implied Vols:\n")
		for type_, vol := range spread.VolatilityInfo.ShortLegImpliedVols {
			fmt.Printf("      %s: %.4f\n", type_, vol)
		}

		fmt.Printf("    Long Leg Implied Vols:\n")
		for type_, vol := range spread.VolatilityInfo.LongLegImpliedVols {
			fmt.Printf("      %s: %.4f\n", type_, vol)
		}
	}

	log.Printf("IdentifySpreads finished at %v. Total time: %v", time.Now(), time.Since(startTime))
	return spreads
}

func processChainOptimized(chain map[string]*tradier.OptionChain, underlyingPrice, riskFreeRate float64, gkVolatilities, parkinsonVolatilities map[string]float64, localVolSurface models.VolatilitySurface, minReturnOnRisk float64, currentDate time.Time, spreadType string, totalJobs int, history tradier.QuoteHistory) []models.SpreadWithProbabilities {
	startTime := time.Now()
	log.Printf("processChainOptimized started at %v", startTime)

	jobChan := make(chan job, workerPoolSize)
	resultChan := make(chan models.SpreadWithProbabilities, workerPoolSize)

	var wg sync.WaitGroup
	for i := 0; i < workerPoolSize; i++ {
		wg.Add(1)
		go worker(jobChan, resultChan, &wg, underlyingPrice, riskFreeRate, minReturnOnRisk, history, localVolSurface)
	}

	go func() {
		generateJobs(chain, underlyingPrice, riskFreeRate, gkVolatilities, parkinsonVolatilities, localVolSurface, currentDate, spreadType, jobChan)
		close(jobChan)
	}()

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	var spreads []models.SpreadWithProbabilities
	var processed int
	for spread := range resultChan {
		if isSpreadViable(spread, minReturnOnRisk) && spread.MeetsRoR {
			spreads = append(spreads, spread)
		}
		processed++
		if processed >= totalJobs {
			break
		}
		fmt.Printf("%d/%d spreads processed (%.2f%%)\n", processed, totalJobs, (float64(processed)/float64(totalJobs))*100)
	}

	log.Printf("processChainOptimized finished at %v. Total time: %v", time.Now(), time.Since(startTime))
	return spreads
}

func generateJobs(chain map[string]*tradier.OptionChain, underlyingPrice, riskFreeRate float64, gkVolatilities, parkinsonVolatilities map[string]float64, localVolSurface models.VolatilitySurface, currentDate time.Time, spreadType string, jobQueue chan<- job) {
	for exp_date, expiration := range chain {
		options := filterOptions(expiration.Options.Option, spreadType)
		if len(options) == 0 {
			continue
		}

		expirationDate, err := time.Parse("2006-01-02", exp_date)
		if err != nil {
			fmt.Printf("Error parsing expiration date %s: %v\n", exp_date, err)
			continue
		}
		daysToExpiration := int(expirationDate.Sub(currentDate).Hours() / 24)

		for i := 0; i < len(options)-1; i++ {
			for j := i + 1; j < len(options); j++ {
				var option1, option2 tradier.Option
				if spreadType == "Bull Put" {
					if options[i].Strike > options[j].Strike {
						option1, option2 = options[i], options[j]
					} else {
						option1, option2 = options[j], options[i]
					}
				} else { // Bear Call
					if options[i].Strike < options[j].Strike {
						option1, option2 = options[i], options[j]
					} else {
						option1, option2 = options[j], options[i]
					}
				}

				jobQueue <- job{
					option1:               option1,
					option2:               option2,
					underlyingPrice:       underlyingPrice,
					riskFreeRate:          riskFreeRate,
					gkVolatilities:        gkVolatilities,
					parkinsonVolatilities: parkinsonVolatilities,
					localVolSurface:       localVolSurface,
					daysToExpiration:      daysToExpiration,
				}
			}
		}
	}
}

func worker(jobQueue <-chan job, resultChan chan<- models.SpreadWithProbabilities, wg *sync.WaitGroup, underlyingPrice, riskFreeRate, minReturnOnRisk float64, history tradier.QuoteHistory, localVolSurface models.VolatilitySurface) {
	defer wg.Done()
	for j := range jobQueue {
		gkVol := j.gkVolatilities[j.option1.ExpirationDate]
		parkinsonVol := j.parkinsonVolatilities[j.option1.ExpirationDate]

		spread := createOptionSpread(j.option1, j.option2, j.underlyingPrice, j.riskFreeRate, gkVol, parkinsonVol)
		returnOnRisk := calculateReturnOnRisk(spread)

		// Check ROR before running Monte Carlo simulation
		if returnOnRisk >= minReturnOnRisk {
			spreadWithProb := probability.MonteCarloSimulation(spread, j.underlyingPrice, j.riskFreeRate, j.daysToExpiration, j.gkVolatilities, j.parkinsonVolatilities, j.localVolSurface, history)
			spreadWithProb.MeetsRoR = true
			resultChan <- spreadWithProb
		} else {
			resultChan <- models.SpreadWithProbabilities{
				Spread:   spread,
				MeetsRoR: false,
			}
		}
	}
}

func createOptionSpread(shortOpt, longOpt tradier.Option, underlyingPrice, riskFreeRate, gkVolatility, parkinsonVolatility float64) models.OptionSpread {
	shortLeg := createSpreadLeg(shortOpt, underlyingPrice, riskFreeRate)
	longLeg := createSpreadLeg(longOpt, underlyingPrice, riskFreeRate)

	spreadType := determineSpreadType(shortOpt, longOpt)

	intrinsicValue := calculateIntrinsicValue(shortLeg, longLeg, underlyingPrice, spreadType)
	spreadCredit := shortLeg.Option.Bid - longLeg.Option.Ask
	extrinsicValue := spreadCredit - intrinsicValue

	spreadBSMPrice := shortLeg.BSMResult.Price - longLeg.BSMResult.Price

	greeks := calculateSpreadGreeks(shortLeg, longLeg)

	ror := calculateReturnOnRisk(models.OptionSpread{
		ShortLeg:       shortLeg,
		LongLeg:        longLeg,
		SpreadType:     spreadType,
		SpreadCredit:   spreadCredit,
		SpreadBSMPrice: spreadBSMPrice,
		ExtrinsicValue: extrinsicValue,
		IntrinsicValue: intrinsicValue,
		Greeks:         greeks,
	})

	return models.OptionSpread{
		ShortLeg:       shortLeg,
		LongLeg:        longLeg,
		SpreadType:     spreadType,
		SpreadCredit:   spreadCredit,
		SpreadBSMPrice: spreadBSMPrice,
		ExtrinsicValue: extrinsicValue,
		IntrinsicValue: intrinsicValue,
		Greeks:         greeks,
		ROR:            ror,
	}
}

func calculateTotalJobs(chain map[string]*tradier.OptionChain, spreadType string) int {
	totalJobs := 0
	for _, expiration := range chain {
		options := filterOptions(expiration.Options.Option, spreadType)
		if len(options) == 0 {
			continue
		}

		for i := 0; i < len(options)-1; i++ {
			for j := i + 1; j < len(options); j++ {
				totalJobs++
			}
		}
	}
	return totalJobs
}

func isSpreadViable(spread models.SpreadWithProbabilities, minROR float64) bool {
	return spread.Spread.ROR > minROR
}

func createSpreadLeg(option tradier.Option, underlyingPrice, riskFreeRate float64) models.SpreadLeg {
	bsmResult := CalculateOptionMetrics(&option, underlyingPrice, riskFreeRate)
	intrinsicValue := calculateSingleOptionIntrinsicValue(option, underlyingPrice)
	extrinsicValue := math.Max(0, bsmResult.Price-intrinsicValue)

	return models.SpreadLeg{
		Option:         option,
		BSMResult:      sanitizeBSMResult(bsmResult),
		BidImpliedVol:  option.Greeks.BidIv,
		AskImpliedVol:  option.Greeks.AskIv,
		MidImpliedVol:  option.Greeks.MidIv,
		ExtrinsicValue: extrinsicValue,
		IntrinsicValue: intrinsicValue,
	}
}

func determineSpreadType(shortOpt, longOpt tradier.Option) string {
	if shortOpt.OptionType == "put" && longOpt.OptionType == "put" {
		return "Bull Put"
	} else if shortOpt.OptionType == "call" && longOpt.OptionType == "call" {
		return "Bear Call"
	}
	return "Unknown"
}

func IdentifyBullPutSpreads(chain map[string]*tradier.OptionChain, underlyingPrice, riskFreeRate float64, history tradier.QuoteHistory, minReturnOnRisk float64, currentDate time.Time) []models.SpreadWithProbabilities {
	return IdentifySpreads(chain, underlyingPrice, riskFreeRate, history, minReturnOnRisk, currentDate, "Bull Put")
}

func IdentifyBearCallSpreads(chain map[string]*tradier.OptionChain, underlyingPrice, riskFreeRate float64, history tradier.QuoteHistory, minReturnOnRisk float64, currentDate time.Time) []models.SpreadWithProbabilities {
	return IdentifySpreads(chain, underlyingPrice, riskFreeRate, history, minReturnOnRisk, currentDate, "Bear Call")
}

func filterOptions(options []tradier.Option, spreadType string) []tradier.Option {
	if spreadType == "Bull Put" {
		return filterPutOptions(options)
	}
	return filterCallOptions(options)
}

func FilterSpreadsByProbability(spreads []models.SpreadWithProbabilities, minProbability float64) []models.SpreadWithProbabilities {
	var filteredSpreads []models.SpreadWithProbabilities
	for _, s := range spreads {
		if s.Probability.AverageProbability >= minProbability {
			filteredSpreads = append(filteredSpreads, s)
		}
	}
	return filteredSpreads
}

func sanitizeBSMResult(result BSMResult) models.BSMResult {
	return models.BSMResult{
		Price:             sanitizeFloat(result.Price),
		ImpliedVolatility: sanitizeFloat(result.ImpliedVolatility),
		Delta:             sanitizeFloat(result.Delta),
		Gamma:             sanitizeFloat(result.Gamma),
		Theta:             sanitizeFloat(result.Theta),
		Vega:              sanitizeFloat(result.Vega),
		Rho:               sanitizeFloat(result.Rho),
		ShadowUpGamma:     sanitizeFloat(result.ShadowUpGamma),
		ShadowDownGamma:   sanitizeFloat(result.ShadowDownGamma),
		SkewGamma:         sanitizeFloat(result.SkewGamma),
	}
}

func calculateReturnOnRisk(spread models.OptionSpread) float64 {
	var maxRisk float64
	if spread.SpreadType == "Bull Put" {
		maxRisk = spread.ShortLeg.Option.Strike - spread.LongLeg.Option.Strike - spread.SpreadCredit
	} else { // Bear Call Spread
		maxRisk = spread.LongLeg.Option.Strike - spread.ShortLeg.Option.Strike - spread.SpreadCredit
	}

	if maxRisk <= 0 {
		log.Printf("Invalid maxRisk: %.2f for spread: Short Strike %.2f, Long Strike %.2f, Credit %.2f\n",
			maxRisk, spread.ShortLeg.Option.Strike, spread.LongLeg.Option.Strike, spread.SpreadCredit)
		return 0
	}

	returnOnRisk := spread.SpreadCredit / maxRisk
	return returnOnRisk
}

func filterPutOptions(options []tradier.Option) []tradier.Option {
	var puts []tradier.Option
	for _, opt := range options {
		if opt.OptionType == "put" {
			puts = append(puts, opt)
		}
	}
	return puts
}

func filterCallOptions(options []tradier.Option) []tradier.Option {
	var calls []tradier.Option
	for _, opt := range options {
		if opt.OptionType == "call" {
			calls = append(calls, opt)
		}
	}
	return calls
}
