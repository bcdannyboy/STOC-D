package positions

import (
	"github.com/bcdannyboy/dquant/models"
	"github.com/bcdannyboy/dquant/tradier"
)

type job struct {
	option1, option2      tradier.Option
	underlyingPrice       float64
	riskFreeRate          float64
	gkVolatilities        map[string]float64
	parkinsonVolatilities map[string]float64
	hestonParams          models.HestonParameters
	localVolSurface       models.VolatilitySurface
	daysToExpiration      int
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

type ParkinsonsResult struct {
	Period            string
	ParkinsonsNumber  float64
	StandardDeviation float64
	Difference        float64 // Parkinson's Number - (1.67 * Standard Deviation)
}

type GarmanKlassResult struct {
	Period     string
	Volatility float64
}

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
	ShortFIV       float64
	ROR            float64
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
	Spread        OptionSpread
	Probabilities map[string]float64
}
