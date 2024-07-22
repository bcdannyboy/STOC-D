package tradier

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
)

func GET_QUOTES(Symbol, Start, End, Interval, Token string) (*QuoteHistory, error) {
	apiURL := fmt.Sprintf("https://api.tradier.com/v1/markets/history?symbol=%s&interval=%s&start=%s&end=%s&session_filter=all", Symbol, Interval, Start, End)

	u, _ := url.ParseRequestURI(apiURL)
	urlStr := u.String()

	client := &http.Client{}
	r, _ := http.NewRequest("GET", urlStr, nil)
	r.Header.Add("Authorization", fmt.Sprintf("Bearer %s", Token))
	r.Header.Add("Accept", "application/json")

	resp, _ := client.Do(r)
	responseData, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return nil, fmt.Errorf("failed to read response data: %s", err)
	}

	defer resp.Body.Close()

	quoteHistory := &QuoteHistory{}

	err = json.Unmarshal(responseData, quoteHistory)

	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response data: %s", err.Error())
	}

	return quoteHistory, nil
}

func GET_OPTIONS_CHAIN(Symbol, Token string, minDTE, maxDTE int) (map[string]*OptionChain, error) {
	expiratons_apiURL := fmt.Sprintf("https://api.tradier.com/v1/markets/options/expirations?symbol=%s&includeAllRoots=true&strikes=true&contractSize=true&expirationType=true", Symbol)

	eu, _ := url.ParseRequestURI(expiratons_apiURL)
	expiratons_urlStr := eu.String()

	client := &http.Client{}
	er, _ := http.NewRequest("GET", expiratons_urlStr, nil)
	er.Header.Add("Authorization", fmt.Sprintf("Bearer %s", Token))
	er.Header.Add("Accept", "application/json")

	expiratons_resp, _ := client.Do(er)
	expiratons_responseData, err := ioutil.ReadAll(expiratons_resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read expirations response data: %s", err)
	}

	defer expiratons_resp.Body.Close()

	expiratons_optionChain := &OptionExpirations{}
	err = json.Unmarshal(expiratons_responseData, expiratons_optionChain)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal expirations response data: %s", err)
	}

	ChainMap := make(map[string]*OptionChain)
	today := time.Now()

	for _, expiration := range expiratons_optionChain.Expirations.Expiration {
		exp_date := expiration.Date
		expirationTime, err := time.Parse("2006-01-02", exp_date)
		if err != nil {
			return nil, fmt.Errorf("failed to parse expiration date: %s", err)
		}

		dte := int(expirationTime.Sub(today).Hours() / 24)
		if dte < minDTE || dte > maxDTE {
			continue
		}

		chain_apiURL := fmt.Sprintf("https://api.tradier.com/v1/markets/options/chains?symbol=%s&expiration=%s&greeks=true", Symbol, exp_date)
		cu, _ := url.ParseRequestURI(chain_apiURL)
		chain_urlStr := cu.String()

		cr, _ := http.NewRequest("GET", chain_urlStr, nil)
		cr.Header.Add("Authorization", fmt.Sprintf("Bearer %s", Token))
		cr.Header.Add("Accept", "application/json")

		chain_resp, _ := client.Do(cr)
		chain_responseData, err := ioutil.ReadAll(chain_resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read chain response data: %s", err)
		}

		defer chain_resp.Body.Close()

		optionChain := &OptionChain{}
		err = json.Unmarshal(chain_responseData, optionChain)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal chain response data: %s", err)
		}

		ChainMap[exp_date] = optionChain
	}

	return ChainMap, nil
}

func GET_PRICE_STATISTICS(symbols, token string) (*PriceStatistics, error) {
	apiURL := fmt.Sprintf("https://api.tradier.com/beta/markets/fundamentals/statistics?symbols=%s", symbols)

	u, _ := url.ParseRequestURI(apiURL)
	urlStr := u.String()

	client := &http.Client{}
	r, _ := http.NewRequest("GET", urlStr, nil)
	r.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	r.Header.Add("Accept", "application/json")

	resp, _ := client.Do(r)
	responseData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response data: %s", err)
	}

	defer resp.Body.Close()

	priceStatistics := &PriceStatistics{}

	err = json.Unmarshal(responseData, priceStatistics)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response data: %s", err)
	}

	return priceStatistics, nil
}
