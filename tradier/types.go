package tradier

type QuoteHistory struct {
	History struct {
		Day []struct {
			Date   string  `json:"date"`
			Open   float64 `json:"open"`
			High   float64 `json:"high"`
			Low    float64 `json:"low"`
			Close  float64 `json:"close"`
			Volume int     `json:"volume"`
		} `json:"day"`
	} `json:"history"`
}

type OptionExpirations struct {
	Expirations struct {
		Expiration []struct {
			Date           string `json:"date"`
			ContractSize   int    `json:"contract_size"`
			ExpirationType string `json:"expiration_type"`
			Strikes        struct {
				Strike []float64 `json:"strike"`
			} `json:"strikes"`
		} `json:"expiration"`
	} `json:"expirations"`
}

type Option struct {
	Symbol           string      `json:"symbol"`
	Description      string      `json:"description"`
	Exch             string      `json:"exch"`
	Type             string      `json:"type"`
	Last             interface{} `json:"last"`
	Change           interface{} `json:"change"`
	Volume           int         `json:"volume"`
	Open             interface{} `json:"open"`
	High             interface{} `json:"high"`
	Low              interface{} `json:"low"`
	Close            interface{} `json:"close"`
	Bid              float64     `json:"bid"`
	Ask              float64     `json:"ask"`
	Underlying       string      `json:"underlying"`
	Strike           float64     `json:"strike"`
	ChangePercentage interface{} `json:"change_percentage"`
	AverageVolume    int         `json:"average_volume"`
	LastVolume       int         `json:"last_volume"`
	TradeDate        int         `json:"trade_date"`
	Prevclose        interface{} `json:"prevclose"`
	Week52High       float64     `json:"week_52_high"`
	Week52Low        float64     `json:"week_52_low"`
	Bidsize          int         `json:"bidsize"`
	Bidexch          string      `json:"bidexch"`
	BidDate          int64       `json:"bid_date"`
	Asksize          int         `json:"asksize"`
	Askexch          string      `json:"askexch"`
	AskDate          int64       `json:"ask_date"`
	OpenInterest     int         `json:"open_interest"`
	ContractSize     int         `json:"contract_size"`
	ExpirationDate   string      `json:"expiration_date"`
	ExpirationType   string      `json:"expiration_type"`
	OptionType       string      `json:"option_type"`
	RootSymbol       string      `json:"root_symbol"`
	Greeks           struct {
		Delta     float64 `json:"delta"`
		Gamma     float64 `json:"gamma"`
		Theta     float64 `json:"theta"`
		Vega      float64 `json:"vega"`
		Rho       float64 `json:"rho"`
		Phi       float64 `json:"phi"`
		BidIv     float64 `json:"bid_iv"`
		MidIv     float64 `json:"mid_iv"`
		AskIv     float64 `json:"ask_iv"`
		SmvVol    float64 `json:"smv_vol"`
		UpdatedAt string  `json:"updated_at"`
	} `json:"greeks"`
}

type OptionChain struct {
	Options        OptionList `json:"options"`
	ExpirationDate string     `json:"expiration_date"`
}

type OptionList struct {
	Option []Option
}

type PriceStatistics []struct {
	Request string `json:"request"`
	Type    string `json:"type"`
	Results []struct {
		Type   string `json:"type"`
		ID     string `json:"id"`
		Tables struct {
			PriceStatistics struct {
				Period5D struct {
					ShareClassID              string  `json:"share_class_id"`
					AsOfDate                  string  `json:"as_of_date"`
					Period                    string  `json:"period"`
					ClosePriceToMovingAverage float64 `json:"close_price_to_moving_average"`
					MovingAveragePrice        float64 `json:"moving_average_price"`
				} `json:"period_5d"`
				Period1W struct {
					ShareClassID             string  `json:"share_class_id"`
					AsOfDate                 string  `json:"as_of_date"`
					Period                   string  `json:"period"`
					AverageVolume            int     `json:"average_volume"`
					HighPrice                float64 `json:"high_price"`
					LowPrice                 float64 `json:"low_price"`
					PercentageBelowHighPrice float64 `json:"percentage_below_high_price"`
					TotalVolume              int     `json:"total_volume"`
				} `json:"period_1w"`
				Period10D struct {
					ShareClassID              string  `json:"share_class_id"`
					AsOfDate                  string  `json:"as_of_date"`
					Period                    string  `json:"period"`
					ClosePriceToMovingAverage float64 `json:"close_price_to_moving_average"`
					MovingAveragePrice        float64 `json:"moving_average_price"`
				} `json:"period_10d"`
				Period13D struct {
					ShareClassID              string  `json:"share_class_id"`
					AsOfDate                  string  `json:"as_of_date"`
					Period                    string  `json:"period"`
					ClosePriceToMovingAverage float64 `json:"close_price_to_moving_average"`
					MovingAveragePrice        float64 `json:"moving_average_price"`
				} `json:"period_13d"`
				Period2W struct {
					ShareClassID             string  `json:"share_class_id"`
					AsOfDate                 string  `json:"as_of_date"`
					Period                   string  `json:"period"`
					AverageVolume            int     `json:"average_volume"`
					HighPrice                float64 `json:"high_price"`
					LowPrice                 float64 `json:"low_price"`
					PercentageBelowHighPrice float64 `json:"percentage_below_high_price"`
					TotalVolume              int     `json:"total_volume"`
				} `json:"period_2w"`
				Period20D struct {
					ShareClassID              string  `json:"share_class_id"`
					AsOfDate                  string  `json:"as_of_date"`
					Period                    string  `json:"period"`
					ClosePriceToMovingAverage float64 `json:"close_price_to_moving_average"`
					MovingAveragePrice        float64 `json:"moving_average_price"`
				} `json:"period_20d"`
				Period30D struct {
					ShareClassID              string  `json:"share_class_id"`
					AsOfDate                  string  `json:"as_of_date"`
					Period                    string  `json:"period"`
					ClosePriceToMovingAverage float64 `json:"close_price_to_moving_average"`
					MovingAveragePrice        float64 `json:"moving_average_price"`
				} `json:"period_30d"`
				Period1M struct {
					ShareClassID             string  `json:"share_class_id"`
					AsOfDate                 string  `json:"as_of_date"`
					Period                   string  `json:"period"`
					AverageVolume            int     `json:"average_volume"`
					HighPrice                float64 `json:"high_price"`
					LowPrice                 float64 `json:"low_price"`
					PercentageBelowHighPrice float64 `json:"percentage_below_high_price"`
					TotalVolume              int     `json:"total_volume"`
				} `json:"period_1m"`
				Period50D struct {
					ShareClassID              string  `json:"share_class_id"`
					AsOfDate                  string  `json:"as_of_date"`
					Period                    string  `json:"period"`
					ClosePriceToMovingAverage float64 `json:"close_price_to_moving_average"`
					MovingAveragePrice        float64 `json:"moving_average_price"`
				} `json:"period_50d"`
				Period60D struct {
					ShareClassID              string  `json:"share_class_id"`
					AsOfDate                  string  `json:"as_of_date"`
					Period                    string  `json:"period"`
					ClosePriceToMovingAverage float64 `json:"close_price_to_moving_average"`
					MovingAveragePrice        float64 `json:"moving_average_price"`
				} `json:"period_60d"`
				Period90D struct {
					ShareClassID              string  `json:"share_class_id"`
					AsOfDate                  string  `json:"as_of_date"`
					Period                    string  `json:"period"`
					ClosePriceToMovingAverage float64 `json:"close_price_to_moving_average"`
					MovingAveragePrice        float64 `json:"moving_average_price"`
				} `json:"period_90d"`
				Period3M struct {
					ShareClassID             string  `json:"share_class_id"`
					AsOfDate                 string  `json:"as_of_date"`
					Period                   string  `json:"period"`
					AverageVolume            int     `json:"average_volume"`
					HighPrice                float64 `json:"high_price"`
					LowPrice                 float64 `json:"low_price"`
					PercentageBelowHighPrice float64 `json:"percentage_below_high_price"`
					TotalVolume              int     `json:"total_volume"`
				} `json:"period_3m"`
				Period6M struct {
					ShareClassID             string  `json:"share_class_id"`
					AsOfDate                 string  `json:"as_of_date"`
					Period                   string  `json:"period"`
					AverageVolume            int     `json:"average_volume"`
					HighPrice                float64 `json:"high_price"`
					LowPrice                 float64 `json:"low_price"`
					PercentageBelowHighPrice float64 `json:"percentage_below_high_price"`
					TotalVolume              int64   `json:"total_volume"`
				} `json:"period_6m"`
				Period200D struct {
					ShareClassID              string  `json:"share_class_id"`
					AsOfDate                  string  `json:"as_of_date"`
					Period                    string  `json:"period"`
					ClosePriceToMovingAverage float64 `json:"close_price_to_moving_average"`
					MovingAveragePrice        float64 `json:"moving_average_price"`
				} `json:"period_200d"`
				Period30W struct {
					ShareClassID              string  `json:"share_class_id"`
					AsOfDate                  string  `json:"as_of_date"`
					Period                    string  `json:"period"`
					ClosePriceToMovingAverage float64 `json:"close_price_to_moving_average"`
					MovingAveragePrice        float64 `json:"moving_average_price"`
				} `json:"period_30w"`
				Period9M struct {
					ShareClassID             string  `json:"share_class_id"`
					AsOfDate                 string  `json:"as_of_date"`
					Period                   string  `json:"period"`
					AverageVolume            int     `json:"average_volume"`
					HighPrice                float64 `json:"high_price"`
					LowPrice                 float64 `json:"low_price"`
					PercentageBelowHighPrice float64 `json:"percentage_below_high_price"`
					TotalVolume              int64   `json:"total_volume"`
				} `json:"period_9m"`
				Period1Y struct {
					ShareClassID              string  `json:"share_class_id"`
					AsOfDate                  string  `json:"as_of_date"`
					Period                    string  `json:"period"`
					ArithmeticMean            float64 `json:"arithmetic_mean"`
					AverageVolume             int     `json:"average_volume"`
					Best3MonthTotalReturn     float64 `json:"best3_month_total_return"`
					ClosePriceToMovingAverage float64 `json:"close_price_to_moving_average"`
					HighPrice                 float64 `json:"high_price"`
					LowPrice                  float64 `json:"low_price"`
					MovingAveragePrice        float64 `json:"moving_average_price"`
					PercentageBelowHighPrice  float64 `json:"percentage_below_high_price"`
					StandardDeviation         float64 `json:"standard_deviation"`
					TotalVolume               int64   `json:"total_volume"`
					Worst3MonthTotalReturn    float64 `json:"worst3_month_total_return"`
				} `json:"period_1y"`
				Period3Y struct {
					ShareClassID             string  `json:"share_class_id"`
					AsOfDate                 string  `json:"as_of_date"`
					Period                   string  `json:"period"`
					ArithmeticMean           float64 `json:"arithmetic_mean"`
					AverageVolume            int     `json:"average_volume"`
					Best3MonthTotalReturn    float64 `json:"best3_month_total_return"`
					HighPrice                float64 `json:"high_price"`
					LowPrice                 float64 `json:"low_price"`
					PercentageBelowHighPrice float64 `json:"percentage_below_high_price"`
					StandardDeviation        float64 `json:"standard_deviation"`
					TotalVolume              int64   `json:"total_volume"`
					Worst3MonthTotalReturn   float64 `json:"worst3_month_total_return"`
				} `json:"period_3y"`
				Period5Y struct {
					ShareClassID              string  `json:"share_class_id"`
					AsOfDate                  string  `json:"as_of_date"`
					Period                    string  `json:"period"`
					ArithmeticMean            float64 `json:"arithmetic_mean"`
					AverageVolume             int     `json:"average_volume"`
					Best3MonthTotalReturn     float64 `json:"best3_month_total_return"`
					ClosePriceToMovingAverage float64 `json:"close_price_to_moving_average"`
					HighPrice                 float64 `json:"high_price"`
					LowPrice                  float64 `json:"low_price"`
					MovingAveragePrice        float64 `json:"moving_average_price"`
					PercentageBelowHighPrice  float64 `json:"percentage_below_high_price"`
					StandardDeviation         float64 `json:"standard_deviation"`
					TotalVolume               int64   `json:"total_volume"`
					Worst3MonthTotalReturn    float64 `json:"worst3_month_total_return"`
				} `json:"period_5y"`
				Period10Y struct {
					ShareClassID             string  `json:"share_class_id"`
					AsOfDate                 string  `json:"as_of_date"`
					Period                   string  `json:"period"`
					ArithmeticMean           float64 `json:"arithmetic_mean"`
					AverageVolume            int     `json:"average_volume"`
					Best3MonthTotalReturn    float64 `json:"best3_month_total_return"`
					HighPrice                float64 `json:"high_price"`
					LowPrice                 float64 `json:"low_price"`
					PercentageBelowHighPrice float64 `json:"percentage_below_high_price"`
					StandardDeviation        float64 `json:"standard_deviation"`
					TotalVolume              int64   `json:"total_volume"`
					Worst3MonthTotalReturn   float64 `json:"worst3_month_total_return"`
				} `json:"period_10y"`
			} `json:"price_statistics"`
			TrailingReturns struct {
				Period1D struct {
					ShareClassID string  `json:"share_class_id"`
					AsOfDate     string  `json:"as_of_date"`
					Period       string  `json:"period"`
					TotalReturn  float64 `json:"total_return"`
				} `json:"period_1d"`
				Period5D struct {
					ShareClassID string  `json:"share_class_id"`
					AsOfDate     string  `json:"as_of_date"`
					Period       string  `json:"period"`
					TotalReturn  float64 `json:"total_return"`
				} `json:"period_5d"`
				Period1M struct {
					ShareClassID string  `json:"share_class_id"`
					AsOfDate     string  `json:"as_of_date"`
					Period       string  `json:"period"`
					TotalReturn  float64 `json:"total_return"`
				} `json:"period_1m"`
				Period3M struct {
					ShareClassID string  `json:"share_class_id"`
					AsOfDate     string  `json:"as_of_date"`
					Period       string  `json:"period"`
					TotalReturn  float64 `json:"total_return"`
				} `json:"period_3m"`
				Period6M struct {
					ShareClassID string  `json:"share_class_id"`
					AsOfDate     string  `json:"as_of_date"`
					Period       string  `json:"period"`
					TotalReturn  float64 `json:"total_return"`
				} `json:"period_6m"`
				Period1Y struct {
					ShareClassID string  `json:"share_class_id"`
					AsOfDate     string  `json:"as_of_date"`
					Period       string  `json:"period"`
					TotalReturn  float64 `json:"total_return"`
				} `json:"period_1y"`
				Period3Y struct {
					ShareClassID string  `json:"share_class_id"`
					AsOfDate     string  `json:"as_of_date"`
					Period       string  `json:"period"`
					TotalReturn  float64 `json:"total_return"`
				} `json:"period_3y"`
				Period5Y struct {
					ShareClassID string  `json:"share_class_id"`
					AsOfDate     string  `json:"as_of_date"`
					Period       string  `json:"period"`
					TotalReturn  float64 `json:"total_return"`
				} `json:"period_5y"`
				Period10Y struct {
					ShareClassID string  `json:"share_class_id"`
					AsOfDate     string  `json:"as_of_date"`
					Period       string  `json:"period"`
					TotalReturn  float64 `json:"total_return"`
				} `json:"period_10y"`
				Period15Y struct {
					ShareClassID string  `json:"share_class_id"`
					AsOfDate     string  `json:"as_of_date"`
					Period       string  `json:"period"`
					TotalReturn  float64 `json:"total_return"`
				} `json:"period_15y"`
				MTD struct {
					ShareClassID string  `json:"share_class_id"`
					AsOfDate     string  `json:"as_of_date"`
					Period       string  `json:"period"`
					TotalReturn  float64 `json:"total_return"`
				} `json:"m_t_d"`
				QTD struct {
					ShareClassID string  `json:"share_class_id"`
					AsOfDate     string  `json:"as_of_date"`
					Period       string  `json:"period"`
					TotalReturn  float64 `json:"total_return"`
				} `json:"q_t_d"`
				YTD struct {
					ShareClassID string  `json:"share_class_id"`
					AsOfDate     string  `json:"as_of_date"`
					Period       string  `json:"period"`
					TotalReturn  float64 `json:"total_return"`
				} `json:"y_t_d"`
			} `json:"trailing_returns"`
		} `json:"tables"`
	} `json:"results"`
}
