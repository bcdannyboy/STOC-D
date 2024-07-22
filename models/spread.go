package models

import (
	"github.com/bcdannyboy/dquant/tradier"
)

type SpreadLeg struct {
	Option            tradier.Option
	BSMResult         BSMResult
	GARCHResult       GARCHResult
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

type GARCHResult struct {
	Params     GARCH11
	Volatility float64
	Greeks     BSMResult
}

type GARCH11 struct {
	Omega float64
	Alpha float64
	Beta  float64
}

type GarmanKlassResult struct {
	Period     string
	Volatility float64
}

type SpreadImpliedVol struct {
	BidIV         float64
	AskIV         float64
	MidIV         float64
	GARCHIV       float64
	BSMIV         float64
	GarmanKlassIV float64
}

type SpreadWithProbabilities struct {
	Spread        OptionSpread
	Probabilities map[string]float64
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
	return (spread.ImpliedVol.BidIV + spread.ImpliedVol.AskIV + spread.ImpliedVol.MidIV + spread.ImpliedVol.GARCHIV + spread.ImpliedVol.BSMIV + spread.ImpliedVol.GarmanKlassIV) / 6
}
