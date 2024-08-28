package positions

import (
	"fmt"
	"log"
	"math"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bcdannyboy/stocd/models"
	"github.com/bcdannyboy/stocd/probability"
	"github.com/bcdannyboy/stocd/tradier"
	"github.com/slack-go/slack"
)

const (
	workerPoolSize = 1000
)

var globalModels probability.GlobalModels
var modelsCalibrated bool

func IdentifySpreads(chain map[string]*tradier.OptionChain, underlyingPrice, riskFreeRate float64, history tradier.QuoteHistory, minReturnOnRisk float64, currentDate time.Time, spreadType string, progressChan chan<- int, slackClient *slack.Client, channelID string, calibrationChan chan<- string, globalModels probability.GlobalModels) []models.SpreadWithProbabilities {
	startTime := time.Now()
	log.Printf("IdentifySpreads started at %v", startTime)

	if len(chain) == 0 {
		fmt.Printf("Warning: Option chain is empty for %s spreads\n", spreadType)
		return nil
	}

	fmt.Printf("Identifying %s Spreads for underlying price: %.2f, Risk-Free Rate: %.4f, Min Return on Risk: %.4f\n", spreadType, underlyingPrice, riskFreeRate, minReturnOnRisk)

	yzVolatilities := models.CalculateYangZhangVolatility(history)
	rsVolatilities := models.CalculateRogersSatchellVolatility(history)
	localVolSurface := models.CalculateLocalVolatilitySurface(chain, underlyingPrice)

	avgYZ := calculateAverageVolatility(yzVolatilities)
	avgRS := calculateAverageVolatility(rsVolatilities)
	avgIV := calculateAverageImpliedVolatility(chain)
	avgVol := (avgYZ + avgRS + avgIV) / 3

	fmt.Printf("Average Yang-Zhang Volatility: %.4f\n", avgYZ)
	fmt.Printf("Average Rogers-Satchell Volatility: %.4f\n", avgRS)
	fmt.Printf("Average Implied Volatility: %.4f\n", avgIV)
	fmt.Printf("Average Volatility: %.4f\n", avgVol)

	calibrateGlobalModels(history, chain, underlyingPrice, riskFreeRate, yzVolatilities, rsVolatilities, spreadType, slackClient, channelID, calibrationChan)

	numCPU := runtime.NumCPU()
	runtime.GOMAXPROCS(numCPU)
	fmt.Printf("Using %d CPUs\n", numCPU)

	totalJobs := calculateTotalJobs(chain, spreadType)
	fmt.Printf("Total spreads to process: %d\n", totalJobs)

	log.Printf("Starting processChainOptimized at %v", time.Now())
	spreads := processChainOptimized(chain, underlyingPrice, riskFreeRate, yzVolatilities, rsVolatilities, localVolSurface, minReturnOnRisk, currentDate, spreadType, totalJobs, history, avgVol, progressChan)
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

		fmt.Printf("    Yang-Zhang Volatilities:\n")
		for period, vol := range spread.VolatilityInfo.YangZhang {
			fmt.Printf("      %s: %.4f\n", period, vol)
		}

		fmt.Printf("    Rogers-Satchell Volatilities:\n")
		for period, vol := range spread.VolatilityInfo.RogersSatchel {
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

		fmt.Printf("  Heston Model Parameters:\n")
		fmt.Printf("    V0: %.4f, Kappa: %.4f, Theta: %.4f, Xi: %.4f, Rho: %.4f\n", spread.HestonParams.V0, spread.HestonParams.Kappa, spread.HestonParams.Theta, spread.HestonParams.Xi, spread.HestonParams.Rho)
	}

	log.Printf("IdentifySpreads finished at %v. Total time: %v", time.Now(), time.Since(startTime))
	return spreads
}

func processChainOptimized(chain map[string]*tradier.OptionChain, underlyingPrice, riskFreeRate float64, yzVolatilities, rsVolatilities map[string]float64, localVolSurface models.VolatilitySurface, minReturnOnRisk float64, currentDate time.Time, spreadType string, totalJobs int, history tradier.QuoteHistory, avgVol float64, progressChan chan<- int) []models.SpreadWithProbabilities {
	startTime := time.Now()
	log.Printf("processChainOptimized started at %v", startTime)

	jobChan := make(chan job, workerPoolSize)
	resultChan := make(chan models.SpreadWithProbabilities, workerPoolSize)

	var wg sync.WaitGroup
	for i := 0; i < workerPoolSize; i++ {
		wg.Add(1)
		go worker(jobChan, resultChan, &wg, minReturnOnRisk, history, chain, avgVol)
	}

	go func() {
		generateJobs(chain, underlyingPrice, riskFreeRate, yzVolatilities, rsVolatilities, localVolSurface, currentDate, spreadType, jobChan)
		close(jobChan)
	}()

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	var spreads []models.SpreadWithProbabilities
	var processed int
	for spread := range resultChan {
		// Skip spreads with zero volume in either leg
		if spread.Spread.ShortLeg.Option.Volume == 0 || spread.Spread.LongLeg.Option.Volume == 0 {
			processed++
			if processed >= totalJobs {
				break
			}
			continue
		}

		if isSpreadViable(spread, minReturnOnRisk) && spread.MeetsRoR {
			spreads = append(spreads, spread)
		}
		processed++
		if processed >= totalJobs {
			break
		}
		progress := int((float64(processed) / float64(totalJobs)) * 100)
		progressChan <- progress
	}

	log.Printf("processChainOptimized finished at %v. Total time: %v", time.Now(), time.Since(startTime))
	return spreads
}

func calibrateGlobalModels(history tradier.QuoteHistory, chain map[string]*tradier.OptionChain, underlyingPrice, riskFreeRate float64, yangzhangVolatilities, rogerssatchelVolatilities map[string]float64, spreadType string, slackClient *slack.Client, channelID string, calibrationChan chan<- string) {
	if modelsCalibrated {
		return // Models already calibrated
	}

	sendCalibrationMessage := func(message string) {
		calibrationChan <- message
	}

	sendCalibrationMessage("Starting model calibration...")
	sendCalibrationMessage(fmt.Sprintf("Risk-Free Rate: %.4f", riskFreeRate))

	fmt.Printf("Calibrating models...\n")
	fmt.Printf("Risk-Free Rate: %.4f\n", riskFreeRate)
	fmt.Printf("Extracting historical prices and strikes...\n")
	sendCalibrationMessage("Extracting historical prices and strikes...")
	marketPrices := extractHistoricalPrices(history)
	fmt.Printf("Extracting all strikes...\n")
	sendCalibrationMessage("Extracting all strikes...")
	strikes := extractAllStrikes(chain)
	s0 := marketPrices[len(marketPrices)-1]
	t := 1.0 // Use 1 year as a default time to maturity

	// Calculate average volatilities
	avgYZ := calculateAverageVolatility(yangzhangVolatilities)
	avgRS := calculateAverageVolatility(rogerssatchelVolatilities)
	avgIV := calculateAverageImpliedVolatility(chain)
	avgVol := (avgYZ + avgRS + avgIV) / 3

	volatilityMsg := fmt.Sprintf("Average Volatilities:\nYang-Zhang: %.4f\nRogers-Satchell: %.4f\nImplied: %.4f\nOverall: %.4f", avgYZ, avgRS, avgIV, avgVol)
	sendCalibrationMessage(volatilityMsg)

	// Calibrate Merton model
	sendCalibrationMessage("Calibrating Merton model...")
	fmt.Printf("Calculating historical jumps...\n")
	historicalJumps := calculateHistoricalJumps(history)
	mertonModel := models.NewMertonJumpDiffusion(riskFreeRate, avgVol, 1.0, 0, avgVol)
	fmt.Printf("Calibrating Merton model with historical jumps...\n")
	mertonModel.CalibrateJumpSizes(historicalJumps, 1)
	globalModels.Merton = mertonModel

	// Calibrate Kou model
	sendCalibrationMessage("Calibrating Kou model...")
	fmt.Printf("Calibrating Kou model...\n")
	kouModel := models.NewKouJumpDiffusion(riskFreeRate, avgVol, marketPrices, 1.0/252.0)
	globalModels.Kou = kouModel

	// Calibrate CGMY model
	sendCalibrationMessage("Calibrating CGMY model...")
	fmt.Printf("Calibrating CGMY model...\n")
	cgmyProcess := models.NewCGMYProcess(0.1, 5.0, 10.0, 0.5) // Initial guess
	cgmyt := 1.0                                              // Use 1 year as a default time to maturity
	isCall := true                                            // Assume we're using call options for calibration

	if strings.Contains(strings.ToLower(spreadType), "put") {
		fmt.Printf("Using put options for CGMY calibration\n")
		sendCalibrationMessage("Using put options for CGMY calibration")
	}

	cgmyProcess.Calibrate(marketPrices, strikes, underlyingPrice, riskFreeRate, cgmyt, isCall)
	globalModels.CGMY = cgmyProcess

	// Calibrate Heston model
	sendCalibrationMessage("Calibrating Heston model...")
	fmt.Printf("Calibrating Heston model...\n")
	hestonModel := models.NewHestonModel(avgVol*avgVol, 2, avgVol*avgVol, 0.4, -0.5)
	err := hestonModel.Calibrate(marketPrices, strikes, s0, riskFreeRate, t)
	if err != nil {
		errMsg := fmt.Sprintf("Error calibrating Heston model: %v", err)
		fmt.Println(errMsg)
		sendCalibrationMessage(errMsg)
		// TODO: Handle calibration error
	}
	globalModels.Heston = hestonModel

	modelsCalibrated = true
	fmt.Printf("Models calibrated\n")
	sendCalibrationMessage("All models calibrated successfully")
}

func generateJobs(chain map[string]*tradier.OptionChain, underlyingPrice, riskFreeRate float64, yzVolatilities, rsVolatilities map[string]float64, localVolSurface models.VolatilitySurface, currentDate time.Time, spreadType string, jobQueue chan<- job) {
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
					option1:          option1,
					option2:          option2,
					underlyingPrice:  underlyingPrice,
					riskFreeRate:     riskFreeRate,
					yzVolatilities:   yzVolatilities,
					rsVolatilities:   rsVolatilities,
					localVolSurface:  localVolSurface,
					daysToExpiration: daysToExpiration,
				}
			}
		}
	}
}

func worker(jobQueue <-chan job, resultChan chan<- models.SpreadWithProbabilities, wg *sync.WaitGroup, minReturnOnRisk float64, history tradier.QuoteHistory, chain map[string]*tradier.OptionChain, avgVol float64) {
	defer wg.Done()
	for j := range jobQueue {
		spread := createOptionSpread(j.option1, j.option2, j.underlyingPrice, j.riskFreeRate)
		returnOnRisk := calculateReturnOnRisk(spread)

		if returnOnRisk >= minReturnOnRisk {
			spreadWithProb := probability.MonteCarloSimulation(spread, j.underlyingPrice, j.riskFreeRate, j.daysToExpiration, j.yzVolatilities, j.rsVolatilities, j.localVolSurface, history, chain, globalModels, avgVol)
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

func createOptionSpread(shortOpt, longOpt tradier.Option, underlyingPrice, riskFreeRate float64) models.OptionSpread {
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

func IdentifyBullPutSpreads(chain map[string]*tradier.OptionChain, underlyingPrice, riskFreeRate float64, history tradier.QuoteHistory, minReturnOnRisk float64, currentDate time.Time, progressChan chan<- int, slackClient *slack.Client, channelID string, calibrationChan chan<- string, globalModels probability.GlobalModels) []models.SpreadWithProbabilities {
	return IdentifySpreads(chain, underlyingPrice, riskFreeRate, history, minReturnOnRisk, currentDate, "Bull Put", progressChan, slackClient, channelID, calibrationChan, globalModels)
}

func IdentifyBearCallSpreads(chain map[string]*tradier.OptionChain, underlyingPrice, riskFreeRate float64, history tradier.QuoteHistory, minReturnOnRisk float64, currentDate time.Time, progressChan chan<- int, slackClient *slack.Client, channelID string, calibrationChan chan<- string, globalModels probability.GlobalModels) []models.SpreadWithProbabilities {
	return IdentifySpreads(chain, underlyingPrice, riskFreeRate, history, minReturnOnRisk, currentDate, "Bear Call", progressChan, slackClient, channelID, calibrationChan, globalModels)
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
