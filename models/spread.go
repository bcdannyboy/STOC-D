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
	CombinedForwardVol  float64
	GarmanKlassVols     map[string]float64
	ParkinsonVols       map[string]float64
	TotalAvgVolSurface  float64
	ShortLegImpliedVols map[string]float64
	LongLegImpliedVols  map[string]float64
}

// Update SpreadWithProbabilities to use the new type
type SpreadWithProbabilities struct {
	Spread       OptionSpread
	Probability  ProbabilityResult
	MeetsRoR     bool
	MertonParams struct {
		Lambda float64
		Mu     float64
		Delta  float64
	}
	KouParams struct {
		Lambda float64
		P      float64
		Eta1   float64
		Eta2   float64
	}
	VolatilityInfo VolatilityInfo
}

type ProbabilityResult struct {
	Probabilities      map[string]float64
	AverageProbability float64
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
