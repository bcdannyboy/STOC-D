package models

import (
	"github.com/bcdannyboy/dquant/tradier"
)

type SpreadLeg struct {
	Option            tradier.Option
	BSMResult         BSMResult
	GarmanKlassResult GarmanKlassResult
	BidImpliedVol     float64
	AskImpliedVol     float64
	MidImpliedVol     float64
	ExtrinsicValue    float64
	IntrinsicValue    float64
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
	ImpliedVol     SpreadImpliedVol
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

type SpreadImpliedVol struct {
	BidIV               float64
	AskIV               float64
	MidIV               float64
	BSMIV               float64
	GarmanKlassIV       float64
	ParkinsonVolatility float64
	ShortLegBSMIV       float64
}

type SpreadWithProbabilities struct {
	Spread      OptionSpread
	Probability ProbabilityResult
	MeetsRoR    bool
}

type ProbabilityResult struct {
	Probabilities           map[string]float64
	AverageProbability      float64
	FuzzyAverageProbability float64
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

func CalculateAverageVolatility(spread OptionSpread) float64 {
	return (spread.ImpliedVol.BidIV + spread.ImpliedVol.AskIV + spread.ImpliedVol.MidIV +
		spread.ImpliedVol.BSMIV +
		spread.ImpliedVol.ShortLegBSMIV + spread.ImpliedVol.GarmanKlassIV) / 6
}
