package positions

import (
	"math"

	"github.com/bcdannyboy/dquant/tradier"
)

func createOptionSpread(shortOpt, longOpt tradier.Option, spreadType string, underlyingPrice, riskFreeRate float64, history tradier.QuoteHistory) OptionSpread {
	shortLeg := createSpreadLeg(shortOpt, underlyingPrice, riskFreeRate, history)
	longLeg := createSpreadLeg(longOpt, underlyingPrice, riskFreeRate, history)

	spreadCredit := shortLeg.Option.Bid - longLeg.Option.Ask
	spreadBSMPrice := shortLeg.BSMResult.Price - longLeg.BSMResult.Price

	extrinsicValue := math.Max(spreadCredit-(longOpt.Strike-shortOpt.Strike), 0)
	intrinsicValue := math.Max((longOpt.Strike-shortOpt.Strike)-spreadCredit, 0)

	return OptionSpread{
		ShortLeg:       shortLeg,
		LongLeg:        longLeg,
		SpreadType:     spreadType,
		SpreadCredit:   spreadCredit,
		SpreadBSMPrice: spreadBSMPrice,
		ExtrinsicValue: extrinsicValue,
		IntrinsicValue: intrinsicValue,
	}
}

func createSpreadLeg(option tradier.Option, underlyingPrice, riskFreeRate float64, history tradier.QuoteHistory) SpreadLeg {
	bsmResult := CalculateOptionMetrics(&option, underlyingPrice, riskFreeRate)
	garchResult := CalculateGARCHVolatility(history, option, underlyingPrice, riskFreeRate)

	// Calculate Garman-Klass volatility
	garmanKlassResults := CalculateGarmanKlassVolatility(history)
	var garmanKlassResult GarmanKlassResult
	if len(garmanKlassResults) > 0 {
		garmanKlassResult = garmanKlassResults[0] // Use the most recent period
	}

	extrinsicValue := math.Max(option.Bid-math.Max(underlyingPrice-option.Strike, 0), 0)
	intrinsicValue := math.Max(underlyingPrice-option.Strike, 0)

	return SpreadLeg{
		Option:            option,
		BSMResult:         bsmResult,
		GARCHResult:       garchResult,
		GarmanKlassResult: garmanKlassResult,
		BidImpliedVol:     option.Greeks.BidIv,
		AskImpliedVol:     option.Greeks.AskIv,
		MidImpliedVol:     option.Greeks.MidIv,
		ExtrinsicValue:    extrinsicValue,
		IntrinsicValue:    intrinsicValue,
	}
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

// Update the function signatures to include the history parameter
func IdentifyBullPutSpreads(chain map[string]*tradier.OptionChain, underlyingPrice, riskFreeRate float64, history tradier.QuoteHistory) []OptionSpread {
	var spreads []OptionSpread

	for _, expiration := range chain {
		puts := filterPutOptions(expiration.Options.Option)
		for i := 0; i < len(puts)-1; i++ {
			for j := i + 1; j < len(puts); j++ {
				if puts[i].Strike < puts[j].Strike {
					spread := createOptionSpread(puts[i], puts[j], "Bull Put", underlyingPrice, riskFreeRate, history)
					spreads = append(spreads, spread)
				}
			}
		}
	}

	return spreads
}

func IdentifyBearCallSpreads(chain map[string]*tradier.OptionChain, underlyingPrice, riskFreeRate float64, history tradier.QuoteHistory) []OptionSpread {
	var spreads []OptionSpread

	for _, expiration := range chain {
		calls := filterCallOptions(expiration.Options.Option)
		for i := 0; i < len(calls)-1; i++ {
			for j := i + 1; j < len(calls); j++ {
				if calls[i].Strike < calls[j].Strike {
					spread := createOptionSpread(calls[i], calls[j], "Bear Call", underlyingPrice, riskFreeRate, history)
					spreads = append(spreads, spread)
				}
			}
		}
	}

	return spreads
}
