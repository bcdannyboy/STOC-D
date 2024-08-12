package models

import (
	"github.com/bcdannyboy/dquant/tradier"
)

type SpreadLeg struct {
	Option         tradier.Option
	BSMResult      BSMResult
	BidImpliedVol  float64
	AskImpliedVol  float64
	MidImpliedVol  float64
	ExtrinsicValue float64
	IntrinsicValue float64
}

type OptionSpread struct {
	ShortLeg       SpreadLeg
	LongLeg        SpreadLeg
	SpreadType     string
	SpreadCredit   float64
	SpreadBSMPrice float64
	ExtrinsicValue float64
	IntrinsicValue float64
	Greeks         BSMResult
	ROR            float64
}

type BSMResult struct {
	Price             float64
	ImpliedVolatility float64
	Delta             float64
	Gamma             float64
	Theta             float64
	Vega              float64
	Rho               float64
	ShadowUpGamma     float64
	ShadowDownGamma   float64
	SkewGamma         float64
}

type GarmanKlassResult struct {
	Period     string
	Volatility float64
}

type VolatilityInfo struct {
	ShortLegVol         float64
	LongLegVol          float64
	YangZhang           map[string]float64
	RogersSatchel       map[string]float64
	TotalAvgVolSurface  float64
	ShortLegImpliedVols map[string]float64
	LongLegImpliedVols  map[string]float64
	HestonVolatility    float64
}

type SpreadWithProbabilities struct {
	Spread       OptionSpread
	Probability  ProbabilityResult
	MeetsRoR     bool
	MertonParams struct {
		Lambda float64 // Intensity of jumps
		Mu     float64 // Drift of jumps
		Delta  float64 // Volatility of jumps
	}
	KouParams struct {
		Lambda float64 // Intensity of jumps
		P      float64 // Probability of up jump
		Eta1   float64 // Magnitude of up jump
		Eta2   float64 // Magnitude of down jump
	}
	HestonParams struct {
		V0    float64 // Initial variance
		Kappa float64 // Mean reversion speed of variance
		Theta float64 // Long-term variance
		Xi    float64 // Volatility of variance
		Rho   float64 // Correlation between asset returns and variance
	}
	VolatilityInfo VolatilityInfo
}

type ProbabilityResult struct {
	Probabilities      map[string]float64
	AverageProbability float64
}

type HestonParams struct {
	V0    float64 // Initial variance
	Kappa float64 // Mean reversion speed of variance
	Theta float64 // Long-term variance
	Xi    float64 // Volatility of variance
	Rho   float64 // Correlation between asset returns and variance
}

type MertonParams struct {
	Lambda float64 // Intensity of jumps
	Mu     float64 // Drift of jumps
	Delta  float64 // Volatility of jumps
}

type KouParams struct {
	Lambda float64 // Intensity of jumps
	P      float64 // Probability of up jump
	Eta1   float64 // Magnitude of up jump
	Eta2   float64 // Magnitude of down jump
}

func IsProfitable(spread OptionSpread, finalPrice float64) bool {
	switch spread.SpreadType {
	case "Bull Put":
		return finalPrice > spread.ShortLeg.Option.Strike
	case "Bear Call":
		return finalPrice < spread.ShortLeg.Option.Strike
	default:
		return false
	}
}
