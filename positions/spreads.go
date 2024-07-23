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

func createOptionSpread(shortOpt, longOpt tradier.Option, underlyingPrice, riskFreeRate, gkVolatility, parkinsonVolatility float64) models.OptionSpread {
	shortLeg := createSpreadLeg(shortOpt, underlyingPrice, riskFreeRate)
	longLeg := createSpreadLeg(longOpt, underlyingPrice, riskFreeRate)

	spreadCredit := shortLeg.Option.Bid - longLeg.Option.Ask
	spreadType := determineSpreadType(shortOpt, longOpt)

	intrinsicValue := calculateIntrinsicValue(shortLeg, longLeg, underlyingPrice, spreadType)
	extrinsicValue := spreadCredit - intrinsicValue

	spreadBSMPrice := shortLeg.BSMResult.Price - longLeg.BSMResult.Price

	spreadImpliedVol := calculateSpreadImpliedVol(shortLeg, longLeg, gkVolatility, parkinsonVolatility)
	greeks := calculateSpreadGreeks(shortLeg, longLeg)

	return models.OptionSpread{
		ShortLeg:       shortLeg,
		LongLeg:        longLeg,
		SpreadType:     spreadType,
		SpreadCredit:   spreadCredit,
		SpreadBSMPrice: spreadBSMPrice,
		ExtrinsicValue: extrinsicValue,
		IntrinsicValue: intrinsicValue,
		ImpliedVol:     spreadImpliedVol,
		Greeks:         greeks,
	}
}

func calculateSpreadImpliedVol(shortLeg, longLeg models.SpreadLeg, gkVolatility, parkinsonVolatility float64) models.SpreadImpliedVol {
	combinedBidIV := (shortLeg.Option.Greeks.Vega*shortLeg.Option.Greeks.BidIv + longLeg.Option.Greeks.Vega*longLeg.Option.Greeks.BidIv) /
		(shortLeg.Option.Greeks.Vega + longLeg.Option.Greeks.Vega)
	combinedAskIV := (shortLeg.Option.Greeks.Vega*shortLeg.Option.Greeks.AskIv + longLeg.Option.Greeks.Vega*longLeg.Option.Greeks.AskIv) /
		(shortLeg.Option.Greeks.Vega + longLeg.Option.Greeks.Vega)
	combinedMidIV := (shortLeg.Option.Greeks.Vega*shortLeg.Option.Greeks.MidIv + longLeg.Option.Greeks.Vega*longLeg.Option.Greeks.MidIv) /
		(shortLeg.Option.Greeks.Vega + longLeg.Option.Greeks.Vega)
	combinedBSMIV := (shortLeg.BSMResult.Vega*shortLeg.BSMResult.ImpliedVolatility + longLeg.BSMResult.Vega*longLeg.BSMResult.ImpliedVolatility) /
		(shortLeg.BSMResult.Vega + longLeg.BSMResult.Vega)

	return models.SpreadImpliedVol{
		BidIV:               combinedBidIV,
		AskIV:               combinedAskIV,
		MidIV:               combinedMidIV,
		BSMIV:               combinedBSMIV,
		GarmanKlassIV:       gkVolatility,
		ParkinsonVolatility: parkinsonVolatility,
		ShortLegBSMIV:       shortLeg.BSMResult.ImpliedVolatility,
	}
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

func IdentifySpreads(chain map[string]*tradier.OptionChain, underlyingPrice, riskFreeRate float64, history tradier.QuoteHistory, minReturnOnRisk float64, currentDate time.Time, spreadType string) []models.SpreadWithProbabilities {
	startTime := time.Now()
	log.Printf("IdentifySpreads started at %v", startTime)

	if len(chain) == 0 {
		fmt.Printf("Warning: Option chain is empty for %s spreads\n", spreadType)
		return nil
	}

	fmt.Printf("Identifying %s Spreads for underlying price: %.2f, Risk-Free Rate: %.4f, Min Return on Risk: %.4f\n", spreadType, underlyingPrice, riskFreeRate, minReturnOnRisk)

	gkVolatility := CalculateGarmanKlassVolatility(history).Volatility
	parkinsonVolatility := CalculateParkinsonsVolatility(history)

	numCPU := runtime.NumCPU()
	runtime.GOMAXPROCS(numCPU)
	fmt.Printf("Using %d CPUs\n", numCPU)

	log.Printf("Starting processChainOptimized at %v", time.Now())
	spreads := processChainOptimized(chain, underlyingPrice, riskFreeRate, gkVolatility, parkinsonVolatility, minReturnOnRisk, currentDate, spreadType)
	log.Printf("Finished processChainOptimized at %v", time.Now())

	log.Printf("Sorting %d spreads by average probability", len(spreads))
	sort.Slice(spreads, func(i, j int) bool {
		return spreads[i].Probability.AverageProbability > spreads[j].Probability.AverageProbability
	})

	fmt.Printf("\nProcessing complete. Total time: %v\n", time.Since(currentDate))
	fmt.Printf("Identified %d %s Spreads meeting minimum return on risk\n", len(spreads), spreadType)

	log.Printf("IdentifySpreads finished at %v. Total time: %v", time.Now(), time.Since(startTime))
	return spreads
}

func processChainOptimized(chain map[string]*tradier.OptionChain, underlyingPrice, riskFreeRate, gkVolatility, parkinsonVolatility, minReturnOnRisk float64, currentDate time.Time, spreadType string) []models.SpreadWithProbabilities {
	startTime := time.Now()
	log.Printf("processChainOptimized started at %v", startTime)

	jobChan := make(chan job, workerPoolSize)
	resultChan := make(chan models.SpreadWithProbabilities, workerPoolSize)
	done := make(chan struct{})

	log.Printf("Starting worker pool at %v", time.Now())
	var wg sync.WaitGroup
	for i := 0; i < workerPoolSize; i++ {
		wg.Add(1)
		go worker(jobChan, resultChan, &wg, underlyingPrice, riskFreeRate)
	}

	log.Printf("Starting job generator at %v", time.Now())
	go generateJobs(chain, underlyingPrice, riskFreeRate, gkVolatility, parkinsonVolatility, minReturnOnRisk, currentDate, spreadType, jobChan)

	var spreads []models.SpreadWithProbabilities
	go func() {
		for spread := range resultChan {
			spreads = append(spreads, spread)
		}
		close(done)
	}()

	wg.Wait()
	close(resultChan)
	<-done

	log.Printf("processChainOptimized finished at %v. Total time: %v", time.Now(), time.Since(startTime))
	return spreads
}

func worker(jobQueue <-chan job, resultChan chan<- models.SpreadWithProbabilities, wg *sync.WaitGroup, underlyingPrice, riskFreeRate float64) {
	defer wg.Done()
	for j := range jobQueue {
		spread := createOptionSpread(j.option1, j.option2, j.underlyingPrice, j.riskFreeRate, j.gkVolatility, j.parkinsonVolatility)
		returnOnRisk := calculateReturnOnRisk(spread)
		if returnOnRisk >= j.minReturnOnRisk {
			probabilityResult := probability.MonteCarloSimulation(spread, j.underlyingPrice, j.riskFreeRate, j.daysToExpiration)
			select {
			case resultChan <- models.SpreadWithProbabilities{
				Spread:      spread,
				Probability: probabilityResult,
			}:
			default:
				// If the channel is full, log a warning and continue
				log.Printf("Warning: Result channel is full, skipping a result")
			}
		}
	}
}

func generateJobs(chain map[string]*tradier.OptionChain, underlyingPrice, riskFreeRate, gkVolatility, parkinsonVolatility, minReturnOnRisk float64, currentDate time.Time, spreadType string, jobQueue chan<- job) {
	startTime := time.Now()
	log.Printf("generateJobs started at %v", startTime)
	defer close(jobQueue)

	jobCount := 0
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

				select {
				case jobQueue <- job{
					option1:             option1,
					option2:             option2,
					underlyingPrice:     underlyingPrice,
					riskFreeRate:        riskFreeRate,
					gkVolatility:        gkVolatility,
					parkinsonVolatility: parkinsonVolatility,
					minReturnOnRisk:     minReturnOnRisk,
					daysToExpiration:    daysToExpiration,
				}:
					jobCount++
					if jobCount%1000 == 0 {
						log.Printf("Generated %d jobs at %v", jobCount, time.Now())
					}
				default:
					// If the channel is full, wait a bit and try again
					time.Sleep(time.Millisecond)
				}
			}
		}
	}

	log.Printf("generateJobs finished at %v. Total time: %v. Total jobs: %d", time.Now(), time.Since(startTime), jobCount)
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

func calculateSpreadIV(shortLeg, longLeg models.SpreadLeg, gkVolatility float64) models.SpreadImpliedVol {
	return models.SpreadImpliedVol{
		BidIV:         sanitizeFloat(shortLeg.BidImpliedVol - longLeg.BidImpliedVol),
		AskIV:         sanitizeFloat(shortLeg.AskImpliedVol - longLeg.AskImpliedVol),
		MidIV:         sanitizeFloat(shortLeg.MidImpliedVol - longLeg.MidImpliedVol),
		BSMIV:         sanitizeFloat(shortLeg.BSMResult.ImpliedVolatility - longLeg.BSMResult.ImpliedVolatility),
		ShortLegBSMIV: sanitizeFloat(shortLeg.BSMResult.ImpliedVolatility),
		GarmanKlassIV: gkVolatility,
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
		fmt.Printf("Invalid maxRisk: %.2f for spread: Short Strike %.2f, Long Strike %.2f, Credit %.2f\n",
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

func calculatePreliminaryRoR(option1, option2 tradier.Option, spreadType string) float64 {
	var spreadCredit, maxRisk float64
	if spreadType == "Bull Put" {
		spreadCredit = option1.Bid - option2.Ask
		maxRisk = option1.Strike - option2.Strike - spreadCredit
	} else { // Bear Call
		spreadCredit = option1.Bid - option2.Ask
		maxRisk = option2.Strike - option1.Strike - spreadCredit
	}

	if maxRisk <= 0 {
		return 0
	}

	return spreadCredit / maxRisk
}
