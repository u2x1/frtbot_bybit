package websocket

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"bybit-bot/config"
	"bybit-bot/internal/constant"
	"bybit-bot/internal/types"
	"bybit-bot/internal/utils"

	"github.com/gorilla/websocket"
)

var tlog = log.New(os.Stdout, "[_TRADE] ", log.Ldate|log.Lmicroseconds|log.Lmsgprefix)

type TradeClient struct {
	conn         *websocket.Conn
	config       *config.Config
	done         chan struct{}
	pongDone     *chan struct{}
	NextTrade    *types.NextTrade
	LastTrade    *types.LastTrade
	lastConnTime time.Time
}

func NewTradeWebsocketConn(tradeClient *TradeClient, cfg *config.Config) *websocket.Conn {
	dialer := websocket.DefaultDialer

	dialer.WriteBufferSize = 0
	dialer.ReadBufferSize = 0

	baseURL := constant.WS_TRADE_URL
	if cfg.TestMode {
		baseURL = constant.TEST_WS_TRADE_URL
	}

	conn, _, err := dialer.Dial(baseURL, nil)
	if err != nil {
		tlog.Fatalf("websocket dial error: %v", err)
	}

	Auth(conn, cfg)

	if tradeClient != nil && tradeClient.pongDone != nil {
		select {
		case <-*tradeClient.pongDone:
			break
		default:
			close(*tradeClient.pongDone)
		}
	}

	pongChan := make(chan struct{})
	tradeClient.pongDone = &pongChan
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-*tradeClient.pongDone:
				return
			case <-ticker.C:
				conn.WriteMessage(websocket.TextMessage, []byte(`{"op":"ping"}`))
			}
		}
	}()
	return conn
}

func NewTradeClient(cfg *config.Config) *TradeClient {

	client := &TradeClient{
		config:       cfg,
		done:         make(chan struct{}),
		lastConnTime: time.Now(),
	}

	client.conn = NewTradeWebsocketConn(client, cfg)

	tlog.Printf("trade client(websocket) initialized")

	go client.StartMessageHandler()

	return client
}

func (c *TradeClient) StartMessageHandler() {
	defer func() {
		tlog.Println("Message handler stopped")
	}()
	tlog.Println("Message handler started")

	for {
		select {
		case <-c.done:
			return
		default:
			_, message, err := c.conn.ReadMessage()
			if err != nil {
				c.Reconnect()
				tlog.Printf("read message error, reconnecting: %v", err)
				return
			}

			var tradeEvent types.TradeEvent
			if err := json.Unmarshal(message, &tradeEvent); err != nil {
				tlog.Printf("unmarshal error: %v", err)
				tlog.Printf("Raw message: %s", string(message))
			} else {
				if tradeEvent.Op == "pong" {
					continue
				}
				tlog.Printf("TradeEvent: %+v", tradeEvent)
			}
		}
	}
}

func (c *TradeClient) Close() {
	close(*c.pongDone)
	close(c.done)
	c.conn.Close()
}

func Auth(conn *websocket.Conn, config *config.Config) {
	expires := time.Now().Unix()*1000 + 10000
	signature := utils.GenerateSignatureString(fmt.Sprintf("GET/realtime%d", expires), config.HMACSecret)

	conn.WriteJSON(types.WSRequest{
		Op:   "auth",
		Args: []interface{}{config.ApiKey, expires, signature},
	})
}

func (c *TradeClient) CreateOrder(params map[string]string) {
	tlog.Printf("place order: %v", params)

	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)

	request := types.WSTradeRequest{
		Op:     "order.create",
		Header: map[string]string{"X-BAPI-TIMESTAMP": timestamp},
		Args:   []interface{}{params},
	}

	c.conn.WriteJSON(request)
}

func (c *TradeClient) CreateMarketOrder(symbol string, side types.TradeSide, quantity float64) {
	params := map[string]string{
		"symbol":    symbol,
		"qty":       strconv.FormatFloat(quantity, 'f', -1, 64),
		"side":      string(side),
		"orderType": "Market",
		"category":  "linear",
	}

	c.CreateOrder(params)
}

func (c *TradeClient) PlaceReduceOnlyLimitOrder(symbol string, side types.TradeSide, quantity, price float64) {
	params := map[string]string{
		"symbol":     symbol,
		"side":       string(side),
		"price":      strconv.FormatFloat(price, 'f', -1, 64),
		"qty":        strconv.FormatFloat(quantity, 'f', -1, 64),
		"orderType":  "Limit",
		"reduceOnly": "true",
		"category":   "linear",
	}

	c.CreateOrder(params)
}

func (c *TradeClient) CreateStopOrder(symbol string, side types.TradeSide, quantity, stopPrice float64) {
	triggerDirection := "1"
	if side == types.TradeSellSide {
		triggerDirection = "2"
	}

	params := map[string]string{
		"symbol":           symbol,
		"side":             string(side),
		"orderType":        "Limit",
		"qty":              strconv.FormatFloat(quantity, 'f', -1, 64),
		"reduceOnly":       "true",
		"triggerPrice":     strconv.FormatFloat(stopPrice, 'f', -1, 64),
		"price":            strconv.FormatFloat(stopPrice, 'f', -1, 64),
		"triggerDirection": triggerDirection,
		"category":         "linear",
	}

	c.CreateOrder(params)
}

func (c *TradeClient) Reconnect() {
	c.conn.Close()
	c.conn = NewTradeWebsocketConn(c, c.config)
	c.lastConnTime = time.Now()
	go c.StartMessageHandler()
	tlog.Println("Trade client reconnected")
}

func (c *TradeClient) EnsureConnection() {
	if time.Since(c.lastConnTime) > 8*time.Hour {
		tlog.Println("lastConnTime is greater than 8 hours, establishing new connection")
		c.Reconnect()
	}
}
