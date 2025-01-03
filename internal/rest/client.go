package rest

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"time"

	"bybit-bot/config"
	"bybit-bot/internal/constant"
	"bybit-bot/internal/types"
	"bybit-bot/internal/utils"
)

type RestClient struct {
	baseURL string
	client  *http.Client
	config  *config.Config
}

var rlog = log.New(os.Stdout, "[__REST] ", log.Ldate|log.Lmicroseconds|log.Lmsgprefix)

const binanceBaseURL = "https://fapi.binance.com"
const recvWindow = "5000"

func NewRestClient(cfg *config.Config) *RestClient {
	client := &http.Client{Timeout: 10 * time.Second}

	baseURL := constant.REST_BASE_URL
	if cfg.TestMode {
		baseURL = constant.TEST_REST_BASE_URL
	}

	return &RestClient{
		baseURL: baseURL,
		client:  client,
		config:  cfg,
	}
}

func (c *RestClient) getRequest(params string, endPoint string) (*http.Response, error) {
	now := time.Now()
	unixNano := now.UnixNano()
	time_stamp := unixNano / 1000000
	hmac256 := hmac.New(sha256.New, []byte(c.config.HMACSecret))
	hmac256.Write([]byte(strconv.FormatInt(time_stamp, 10) + c.config.ApiKey + recvWindow + params))
	signature := hex.EncodeToString(hmac256.Sum(nil))
	request, _ := http.NewRequest("GET", c.baseURL+endPoint+"?"+params, nil)
	// request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-BAPI-API-KEY", c.config.ApiKey)
	request.Header.Set("X-BAPI-SIGN", signature)
	request.Header.Set("X-BAPI-TIMESTAMP", strconv.FormatInt(time_stamp, 10))
	// request.Header.Set("X-BAPI-SIGN-TYPE", "2")
	request.Header.Set("X-BAPI-RECV-WINDOW", recvWindow)
	response, error := c.client.Do(request)
	if error != nil {
		return nil, error
	}
	return response, nil
}

func (c *RestClient) postRequest(params interface{}, endPoint string) (*http.Response, error) {
	now := time.Now()
	unixNano := now.UnixNano()
	time_stamp := unixNano / 1000000
	jsonData, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	hmac256 := hmac.New(sha256.New, []byte(c.config.HMACSecret))
	hmac256.Write([]byte(strconv.FormatInt(time_stamp, 10) + c.config.ApiKey + recvWindow + string(jsonData)))
	signature := hex.EncodeToString(hmac256.Sum(nil))
	request, _ := http.NewRequest("POST", c.baseURL+endPoint, bytes.NewBuffer(jsonData))
	// request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-BAPI-API-KEY", c.config.ApiKey)
	request.Header.Set("X-BAPI-SIGN", signature)
	request.Header.Set("X-BAPI-TIMESTAMP", strconv.FormatInt(time_stamp, 10))
	// request.Header.Set("X-BAPI-SIGN-TYPE", "2")
	request.Header.Set("X-BAPI-RECV-WINDOW", recvWindow)
	response, error := c.client.Do(request)
	if error != nil {
		return nil, error
	}
	return response, nil
}

func (c *RestClient) SetLeverage(leverage int, symbol string) {
	endPoint := "/v5/position/set-leverage"
	params := map[string]string{
		"symbol":       symbol,
		"buyLeverage":  strconv.Itoa(leverage),
		"sellLeverage": strconv.Itoa(leverage),
		"category":     "linear",
	}

	resp, err := c.postRequest(params, endPoint)
	if err != nil {
		rlog.Fatalf("failed to set leverage: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code int    `json:"retCode"`
		Msg  string `json:"retMsg"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		rlog.Fatalf("failed to decode response: %v", err)
	}

	if result.Code != 0 && result.Code != 110043 {
		rlog.Fatalf("failed to set leverage: %s, return code: %d", result.Msg, result.Code)
	}
}

func (c *RestClient) SetMarginType(marginType types.MarginType) {
	endPoint := "/v5/account/set-margin-mode"
	params := map[string]string{
		"setMarginMode": string(c.config.MarginType) + "_MARGIN",
	}

	resp, err := c.postRequest(params, endPoint)
	if err != nil {
		rlog.Fatalf("failed to set margin type: %v", err)
	}

	defer resp.Body.Close()

	var result struct {
		Code int    `json:"retCode"`
		Msg  string `json:"retMsg"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		rlog.Fatalf("failed to decode response: %v", err)
	}

	if result.Code != 0 {
		rlog.Fatalf("failed to set margin type: %s, return code: %d", result.Msg, result.Code)
	}
}

func (c *RestClient) GetBalance() (float64, error) {
	endPoint := "/v5/account/wallet-balance"
	params := map[string]string{
		"accountType": "UNIFIED",
	}

	resp, err := c.getRequest(utils.EncodeMap(params), endPoint)
	if err != nil {
		return 0, fmt.Errorf("failed to get balance: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code   int    `json:"retCode"`
		Msg    string `json:"retMsg"`
		Result struct {
			List []struct {
				TotalEquity float64 `json:"totalEquity,string"`
			} `json:"list"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("failed to decode response: %v", err)
	}

	if result.Code != 0 {
		return 0, fmt.Errorf("failed to get balance: %s, return code: %d", result.Msg, result.Code)
	}

	if len(result.Result.List) == 0 {
		return 0, fmt.Errorf("no balance data found")
	}

	return result.Result.List[0].TotalEquity, nil
}

func (c *RestClient) GetAllSymbols() ([]types.Exchange, error) {
	resp, err := c.getRequest("category=linear", "/v5/market/instruments-info")
	if err != nil {
		return nil, fmt.Errorf("failed to get exchange info: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		Result struct {
			List []types.ExchangeResponse
		}
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	var symbols []types.Exchange
	for _, s := range result.Result.List {
		if s.ContractType == "LinearPerpetual" && s.QuoteCoin == "USDT" {
			symbol := types.Exchange{
				Symbol:       s.Symbol,
				ContractType: s.ContractType,
				QuoteAsset:   s.QuoteCoin,
				MinPrice:     s.PriceFilter.MinPrice,
				MinQty:       s.LotSizeFilter.MinOrderQty,
				MaxQty:       s.LotSizeFilter.MaxOrderQty,
				MaxPrice:     s.PriceFilter.MaxPrice,
			}

			symbols = append(symbols, symbol)
		}
	}

	return symbols, nil
}

func (c *RestClient) GetPremiumIndex(symbol string) (types.PremiumIndex, error) {
	url := fmt.Sprintf("%s/fapi/v1/premiumIndex?symbol=%s", binanceBaseURL, symbol)

	resp, err := c.client.Get(url)
	if err != nil {
		return types.PremiumIndex{}, fmt.Errorf("failed to get premium index for %s: %v", symbol, err)
	}
	defer resp.Body.Close()

	var index types.PremiumIndex
	if err := json.NewDecoder(resp.Body).Decode(&index); err != nil {
		return types.PremiumIndex{}, fmt.Errorf("failed to decode response: %v", err)
	}

	return index, nil
}

func (c *RestClient) GetTop5Exchanges() ([]types.ExchangeInfo, time.Time) {
	symbols, err := c.GetAllSymbols()
	if err != nil {
		rlog.Fatalf("failed to get all symbols: %v", err)
	}

	indexChan := make(chan types.ExchangeInfo)
	errChan := make(chan error)
	done := make(chan struct{})

	curTimestamp := time.Now().UnixMilli()
	minTimestamp := int64(999999999999999999)

	const maxConcurrent = 10
	sem := make(chan struct{}, maxConcurrent)

	go func() {
		for _, symbol := range symbols {
			sem <- struct{}{}
			go func(sym types.Exchange) {
				defer func() { <-sem }()
				index, err := c.GetPremiumIndex(sym.Symbol)
				if err != nil {
					errChan <- fmt.Errorf("failed to get premium index for %s: %v", sym.Symbol, err)
					return
				}
				if index.NextFundingTime > curTimestamp && index.NextFundingTime < minTimestamp {
					minTimestamp = index.NextFundingTime
				}
				indexChan <- types.ExchangeInfo{Symbol: sym, PremiumIndex: index}
			}(symbol)
		}
		for i := 0; i < maxConcurrent; i++ {
			sem <- struct{}{}
		}
		close(done)
	}()

	var allExchanges []types.ExchangeInfo
	remaining := len(symbols)

	for remaining > 0 {
		select {
		case index := <-indexChan:
			allExchanges = append(allExchanges, index)
			remaining--
		case err := <-errChan:
			rlog.Printf("Warning: %v\n", err)
			remaining--
		case <-done:
			if remaining == 0 {
				remaining = -1
			}
		}
	}

	rlog.Printf("minTimestamp of next funding time: %+v", time.UnixMilli(minTimestamp))

	filterExchanges := []types.ExchangeInfo{}
	for _, exchange := range allExchanges {
		if exchange.PremiumIndex.NextFundingTime == minTimestamp {
			filterExchanges = append(filterExchanges, exchange)
		}
	}

	sort.Slice(filterExchanges, func(i, j int) bool {
		return math.Abs(filterExchanges[i].PremiumIndex.LastFundingRate) > math.Abs(filterExchanges[j].PremiumIndex.LastFundingRate)
	})

	ret := make([]types.ExchangeInfo, 0, min(5, len(filterExchanges)))

	for idx, x := range filterExchanges {
		if idx >= 5 {
			break
		}
		if math.Abs(x.PremiumIndex.LastFundingRate) < c.config.MinFundingRate {
			rlog.Printf("abs funding rate of %s is %f which is lower than %f\n",
				x.Symbol.Symbol, math.Abs(x.PremiumIndex.LastFundingRate), c.config.MinFundingRate)
			break
		}
		ret = append(ret, x)
	}

	return ret, time.UnixMilli(minTimestamp)
}
