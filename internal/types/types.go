package types

type TradeSide string

const (
	TradeBuySide  TradeSide = "Buy"
	TradeSellSide TradeSide = "Sell"
)

type WSRequest struct {
	Op   string      `json:"op"`
	Args interface{} `json:"args"`
}

type WSTradeRequest struct {
	Op     string        `json:"op"`
	Header interface{}   `json:"header"`
	Args   []interface{} `json:"args"`
}

type WSResponse struct {
	ID     string      `json:"id"`
	Result interface{} `json:"result"`
	Error  interface{} `json:"error"`
}

type LastTrade struct {
	Quantity float64
	MinQty   float64
	MinPrice float64
	StopSide TradeSide
	Symbol   string
}

type NextTrade struct {
	Quantity float64
	MinQty   float64
	StopSide TradeSide
	Side     TradeSide
	Symbol   string
}

type ExchangeInfo struct {
	Symbol       Exchange
	PremiumIndex PremiumIndex
}

type Exchange struct {
	Symbol       string
	ContractType string
	QuoteAsset   string
	MinPrice     float64
	MinQty       float64
	MaxQty       float64
	MaxPrice     float64
}

type ExchangeResponse struct {
	Symbol       string
	ContractType string
	QuoteCoin    string
	PriceFilter  struct {
		MinPrice float64 `json:"minPrice,string"`
		MaxPrice float64 `json:"maxPrice,string"`
		TickSize float64 `json:"tickSize,string"`
	}
	LotSizeFilter struct {
		MaxOrderQty float64 `json:"maxOrderQty,string"`
		MinOrderQty float64 `json:"minOrderQty,string"`
		QtyStep     float64 `json:"qtyStep,string"`
	}
}

type PremiumIndex struct {
	MarkPrice       float64 `json:"markPrice,string"`
	LastFundingRate float64 `json:"lastFundingRate,string"`
	NextFundingTime int64   `json:"nextFundingTime"`
}

type EventData struct {
	Success      bool        `json:"success,omitempty"`
	Op           string      `json:"op,omitempty"`
	Topic        string      `json:"topic,omitempty"`
	CreationTime int64       `json:"creationTime,omitempty"`
	Data         []OrderData `json:"data,omitempty"`
}

type OrderData struct {
	AvgPrice    string  `json:"avgPrice"`
	Qty         float64 `json:"qty,string"`
	OrderType   string  `json:"orderType"`
	OrderStatus string  `json:"orderStatus"`
}

type TradeEvent struct {
	Code   int         `json:"retCode"`
	Msg    string      `json:"retMsg"`
	Op     string      `json:"op"`
	Data   interface{} `json:"data"`
	Header interface{} `json:"header"`
}

type MarginType string

const (
	MarginTypeIsolated  MarginType = "ISOLATED"
	MarginTypeRegular   MarginType = "REGULAR"
	MarginTypePortfolio MarginType = "PORTFOLIO"
)
