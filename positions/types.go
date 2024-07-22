package positions

import "github.com/bcdannyboy/dquant/tradier"

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

type GARCH11 struct {
	Omega float64
	Alpha float64
	Beta  float64
}

type GARCHResult struct {
	Params     GARCH11
	Volatility float64
	Greeks     BSMResult
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
