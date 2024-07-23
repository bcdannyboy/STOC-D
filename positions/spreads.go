package positions

import (
	"fmt"
	"math"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bcdannyboy/dquant/models"
	"github.com/bcdannyboy/dquant/probability"
	"github.com/bcdannyboy/dquant/tradier"
	"github.com/shirou/gopsutil/cpu"
	mpb "github.com/vbauerster/mpb/v7"
	"github.com/vbauerster/mpb/v7/decor"
)

const (
	jobBatchSize    = 1000
	resultBatchSize = 1000
)

func IdentifySpreads(chain map[string]*tradier.OptionChain, underlyingPrice, riskFreeRate float64, history tradier.QuoteHistory, minReturnOnRisk float64, currentDate time.Time, spreadType string) []models.SpreadWithProbabilities {
	if len(chain) == 0 {
		fmt.Printf("Warning: Option chain is empty for %s spreads\n", spreadType)
		return nil
	}

	fmt.Printf("Identifying %s Spreads for underlying price: %.2f, Risk-Free Rate: %.4f, Min Return on Risk: %.4f\n", spreadType, underlyingPrice, riskFreeRate, minReturnOnRisk)

	gkVolatility := CalculateGarmanKlassVolatility(history).Volatility
	parkinsonVolatility := CalculateParkinsonsVolatility(history)

	jobs := generateJobs(chain, underlyingPrice, riskFreeRate, gkVolatility, parkinsonVolatility, minReturnOnRisk, currentDate, spreadType)
	fmt.Printf("Total potential spreads: %d\n", len(jobs))

	numCPU := runtime.NumCPU()
	runtime.GOMAXPROCS(numCPU)
	fmt.Printf("Using %d CPUs\n", numCPU)

	// Create progress bar
	p := mpb.New(mpb.WithWidth(64))
	bar := p.AddBar(int64(len(jobs)),
		mpb.PrependDecorators(
			decor.Name("Progress"),
			decor.Percentage(decor.WCSyncSpace),
		),
		mpb.AppendDecorators(
			decor.CountersNoUnit("(%d / %d)", decor.WCSyncSpace),
		),
	)

	// Process jobs
	spreads := processJobs(jobs, numCPU, bar)

	// Sort spreads by probability
	sort.Slice(spreads, func(i, j int) bool {
		return spreads[i].Probability.AverageProbability > spreads[j].Probability.AverageProbability
	})

	p.Wait()
	fmt.Printf("\nProcessing complete. Total time: %v\n", time.Since(currentDate))
	fmt.Printf("Identified %d %s Spreads meeting minimum return on risk\n", len(spreads), spreadType)

	return spreads
}

func generateJobs(chain map[string]*tradier.OptionChain, underlyingPrice, riskFreeRate, gkVolatility, parkinsonVolatility, minReturnOnRisk float64, currentDate time.Time, spreadType string) []job {
	var jobs []job

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
				jobs = append(jobs, job{
					option1:             option1,
					option2:             option2,
					underlyingPrice:     underlyingPrice,
					riskFreeRate:        riskFreeRate,
					gkVolatility:        gkVolatility,
					parkinsonVolatility: parkinsonVolatility,
					minReturnOnRisk:     minReturnOnRisk,
					daysToExpiration:    daysToExpiration,
				})
			}
		}
	}

	return jobs
}

func processJobs(jobs []job, numWorkers int, bar *mpb.Bar) []models.SpreadWithProbabilities {
	var wg sync.WaitGroup
	jobChan := make(chan job, jobBatchSize)
	resultChan := make(chan models.SpreadWithProbabilities, resultBatchSize)
	var processed int64

	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go worker(jobChan, resultChan, &wg, &processed, bar)
	}

	// Feed jobs to workers
	go func() {
		for _, j := range jobs {
			jobChan <- j
		}
		close(jobChan)
	}()

	// Collect results
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	var spreads []models.SpreadWithProbabilities
	for spread := range resultChan {
		spreads = append(spreads, spread)
	}

	return spreads
}

func worker(jobs <-chan job, results chan<- models.SpreadWithProbabilities, wg *sync.WaitGroup, processed *int64, bar *mpb.Bar) {
	defer wg.Done()
	for j := range jobs {
		spread := createOptionSpread(j.option1, j.option2, j.underlyingPrice, j.riskFreeRate, j.gkVolatility, j.parkinsonVolatility)
		returnOnRisk := calculateReturnOnRisk(spread)
		if returnOnRisk >= j.minReturnOnRisk {
			probabilityResult := probability.MonteCarloSimulation(spread, j.underlyingPrice, j.riskFreeRate, j.daysToExpiration)
			results <- models.SpreadWithProbabilities{
				Spread:      spread,
				Probability: probabilityResult,
			}
		}
		atomic.AddInt64(processed, 1)
		bar.Increment()
	}
}

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

func determineSpreadType(shortOpt, longOpt tradier.Option) string {
	if shortOpt.OptionType == "put" && longOpt.OptionType == "put" {
		return "Bull Put"
	} else if shortOpt.OptionType == "call" && longOpt.OptionType == "call" {
		return "Bear Call"
	}
	return "Unknown"
}

func calculateSpreadImpliedVol(shortLeg, longLeg models.SpreadLeg, gkVolatility, parkinsonVolatility float64) models.SpreadImpliedVol {
	return models.SpreadImpliedVol{
		BidIV:               math.Max(shortLeg.Option.Greeks.BidIv, 0) - math.Max(longLeg.Option.Greeks.BidIv, 0),
		AskIV:               math.Max(shortLeg.Option.Greeks.AskIv, 0) - math.Max(longLeg.Option.Greeks.AskIv, 0),
		MidIV:               math.Max(shortLeg.Option.Greeks.MidIv, 0) - math.Max(longLeg.Option.Greeks.MidIv, 0),
		BSMIV:               shortLeg.BSMResult.ImpliedVolatility - longLeg.BSMResult.ImpliedVolatility,
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
		BidImpliedVol:  math.Max(0, option.Greeks.BidIv),
		AskImpliedVol:  math.Max(0, option.Greeks.AskIv),
		MidImpliedVol:  math.Max(0, option.Greeks.MidIv),
		ExtrinsicValue: extrinsicValue,
		IntrinsicValue: intrinsicValue,
	}
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

func monitorCPUUsage() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		var cpuUsage float64
		percentage, err := cpu.Percent(time.Second, false)
		if err == nil && len(percentage) > 0 {
			cpuUsage = percentage[0]
		}
		fmt.Printf("\nCPU Usage: %.2f%%\n", cpuUsage)
	}
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
