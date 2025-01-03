package config

import (
	"bybit-bot/internal/types"
	"encoding/json"
	"log"
	"os"
)

var clog = log.New(os.Stdout, "[CONFIG] ", log.Ldate|log.Lmicroseconds|log.Lmsgprefix)

type Config struct {
	ApiKey                 string           `json:"api_key"`
	HMACSecret             string           `json:"hmac_secret"`
	TestMode               bool             `json:"test_mode"`
	TestApiKey             string           `json:"test_api_key"`
	TestHMACSecret         string           `json:"test_hmac_secret"`
	Margin                 float64          `json:"margin"`
	MinFundingRate         float64          `json:"min_funding_rate_percent"`
	StopRatio              float64          `json:"stop_percent"`
	TakeProfitRatio        float64          `json:"take_profit_percent"`
	FirstOrderTimeOffset   int64            `json:"first_order_time_offset_ms"`
	MarginType             types.MarginType `json:"margin_type"`
	Leverage               int              `json:"leverage"`
	BreakevenEnabled       bool             `json:"breakeven_enabled"`
	BreakevenPercent       float64          `json:"breakeven_percent"`
	BreakevenWindowSize    int              `json:"breakeven_window_size"`
	BreakevenPlaceDuration int              `json:"breakeven_place_duration"`
}

func NewConfig(configPath string) *Config {
	data, err := os.ReadFile(configPath)
	if err != nil {
		panic(err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		panic(err)
	}

	if config.TestMode {
		clog.Println("test mode enabled")
		config.ApiKey = config.TestApiKey
		config.HMACSecret = config.TestHMACSecret
	}

	config.MinFundingRate = config.MinFundingRate / 100
	config.StopRatio = config.StopRatio / 100
	config.TakeProfitRatio = config.TakeProfitRatio / 100

	if config.MarginType == "" {
		clog.Fatalf("margin_type is required")
	}

	if config.Leverage == 0 {
		clog.Fatalf("leverage is required")
	}

	if config.Margin == 0 {
		clog.Fatalf("margin is required")
	}

	if config.MarginType != types.MarginTypeIsolated && config.MarginType != types.MarginTypeRegular && config.MarginType != types.MarginTypePortfolio {
		clog.Fatalf("margin_type should only be one of ISOLATED, REGULAR or PORTFOLIO (case sensitive)")
	}

	return &config
}
