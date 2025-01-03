package main

import (
	"bybit-bot/config"
	"bybit-bot/internal/rest"
	"bybit-bot/internal/types"
	"bybit-bot/internal/utils"
	"bybit-bot/internal/websocket"
	"log"
	"os"
	"strconv"
	"time"
)

var mlog = log.New(os.Stdout, "[__MAIN] ", log.Ldate|log.Lmicroseconds|log.Lmsgprefix)

func main() {
	cfg := config.NewConfig("config.json")

	restClient := rest.NewRestClient(cfg)
	tradeClient := websocket.NewTradeClient(cfg)
	websocket.NewStreamClient(tradeClient, restClient, cfg)

	balance, err := restClient.GetBalance()
	if err != nil {
		mlog.Printf("Warning: failed to get balance: %v", err)
		balance = 9999999999
	}

	if balance < cfg.Margin {
		mlog.Fatalf("balance is too low: %f", balance)
	}
	mlog.Printf("usdt balance: %f, can trade %d times", balance, int(balance/cfg.Margin))

	restClient.SetMarginType(cfg.MarginType)
	mlog.Printf("set account margin type to %s", cfg.MarginType)

	for {
		// tradeClient.EnsureConnection()
		// streamClient.EnsureConnection()

		mlog.Println("fetching top 5 funding rates")
		top5, fundingTime := restClient.GetTop5Exchanges()
		mlog.Println("Top 5 Funding Rates (", fundingTime, "):")
		for _, rate := range top5 {
			mlog.Printf("%s: %.4f",
				rate.Symbol.Symbol,
				rate.PremiumIndex.LastFundingRate*100,
			)
		}

		// fundingTime = time.Now().Truncate(time.Minute).Add(time.Minute)

		// top5, fundingTime := []types.ExchangeInfo{
		// 	{
		// 		Symbol: types.Exchange{
		// 			Symbol:   "TROYUSDT",
		// 			MinQty:   100,
		// 			MinPrice: 0.000001,
		// 		},
		// 		PremiumIndex: types.PremiumIndex{
		// 			LastFundingRate: -0.008,
		// 			NextFundingTime: 1714857600000,
		// 			MarkPrice:       100000,
		// 		},
		// 	},
		// 	{
		// 		Symbol: types.Exchange{
		// 			Symbol:   "BTCUSDT",
		// 			MinQty:   0.001,
		// 			MinPrice: 0.1,
		// 		},
		// 		PremiumIndex: types.PremiumIndex{
		// 			LastFundingRate: 0.008,
		// 			NextFundingTime: 1714857600000,
		// 			MarkPrice:       100000,
		// 		},
		// 	},
		// }, time.Now().Truncate(time.Minute).Add(time.Minute)

		if time.Until(fundingTime) > time.Minute*5 {
			distance := time.Until(fundingTime)
			mlog.Printf("funding time is too far away, sleep for %s, will wake up at %s", distance-5*time.Minute, fundingTime.Add(-5*time.Minute))
			time.Sleep(distance - 5*time.Minute)
			continue
		}

		if len(top5) == 0 {
			mlog.Println("no funding rate found, sleep for 5 minutes")
			time.Sleep(time.Minute * 5)
			continue
		}

		top := top5[0]

		mlog.Printf("selected symbol: %+v", top)

		restClient.SetLeverage(cfg.Leverage, top.Symbol.Symbol)
		mlog.Printf("set leverage to %dx", cfg.Leverage)

		mlog.Println("sleep. will wake up at ", fundingTime.Add(-time.Minute))
		time.Sleep(time.Until(fundingTime) - time.Minute)

		mlog.Printf("querying latest price for %s", top.Symbol.Symbol)
		premiumIndex, err := restClient.GetPremiumIndex(top.Symbol.Symbol)
		if err != nil {
			mlog.Printf("failed to get premium index: %v", err)
			continue
		}
		priceFloat := premiumIndex.MarkPrice
		mlog.Printf("latest price: %f", priceFloat)

		quantity := (cfg.Margin * float64(cfg.Leverage) / priceFloat)

		mlog.Printf("quantity: %v", quantity)

		if quantity < top.Symbol.MinQty {
			mlog.Fatalf("calculated order quantity is %v, smaller than min qty %v, please adjust balance", quantity, top.Symbol.MinQty)
		}

		quantity = utils.Truncate(quantity, top.Symbol.MinQty)

		side := types.TradeSellSide
		stopSide := types.TradeBuySide

		if top.PremiumIndex.LastFundingRate > 0 {
			side = types.TradeBuySide
			stopSide = types.TradeSellSide
		}

		mlog.Printf("going to place order: %s %s %s %s %s",
			top.Symbol.Symbol,
			side,
			"Market",
			strconv.FormatFloat(quantity, 'f', -1, 64),
			"",
		)

		offset := time.Duration(cfg.FirstOrderTimeOffset) * time.Millisecond
		if offset > time.Second {
			fundingTime = fundingTime.Add(offset.Truncate(time.Second))
			offset = offset % time.Millisecond
		}

		mlog.Printf("waiting until (funding time)+(offset %dms): %s", cfg.FirstOrderTimeOffset, fundingTime.Add(offset))
		utils.Ticker(offset, time.Second, fundingTime)
		mlog.Println("ticker done")

		tradeClient.CreateMarketOrder(
			top.Symbol.Symbol, // symbol
			side,              // side
			quantity,          // quantity
		)
		tradeClient.LastTrade = &types.LastTrade{
			MinQty:   top.Symbol.MinQty,
			MinPrice: top.Symbol.MinPrice,
			Symbol:   top.Symbol.Symbol,
			StopSide: stopSide,
			Quantity: quantity,
		}
		mlog.Printf("setting LastTrade: %+v", tradeClient.LastTrade)
		time.Sleep(time.Minute)
	}
}
