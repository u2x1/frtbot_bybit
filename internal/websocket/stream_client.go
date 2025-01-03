package websocket

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"strconv"
	"time"

	"bybit-bot/config"
	"bybit-bot/internal/constant"
	"bybit-bot/internal/rest"
	"bybit-bot/internal/types"
	"bybit-bot/internal/utils"

	"github.com/gorilla/websocket"
)

var slog = log.New(os.Stdout, "[STREAM] ", log.Ldate|log.Lmicroseconds|log.Lmsgprefix)

type StreamClient struct {
	conn         *websocket.Conn
	config       *config.Config
	done         chan struct{}
	pongDone     *chan struct{}
	tradeClient  *TradeClient
	restClient   *rest.RestClient
	lastConnTime time.Time
}

func NewStreamWebsocketConn(streamClient *StreamClient, cfg *config.Config) *websocket.Conn {
	dialer := websocket.DefaultDialer

	dialer.WriteBufferSize = 0
	dialer.ReadBufferSize = 0

	baseURL := constant.WS_STREAM_URL
	if cfg.TestMode {
		baseURL = constant.TEST_WS_STREAM_URL
	}

	conn, _, err := dialer.Dial(baseURL, nil)
	if err != nil {
		slog.Fatalf("websocket dial error: %v", err)
	}

	Auth(conn, cfg)
	SubscribeOrderUpdates(conn)

	if streamClient != nil && streamClient.pongDone != nil {
		select {
		case <-*streamClient.pongDone:
			break
		default:
			close(*streamClient.pongDone)
		}
	}

	pongChan := make(chan struct{})
	streamClient.pongDone = &pongChan
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-*streamClient.pongDone:
				return
			case <-ticker.C:
				conn.WriteMessage(websocket.TextMessage, []byte(`{"op":"ping"}`))
			}
		}
	}()
	return conn
}

func NewStreamClient(tradeClient *TradeClient, restClient *rest.RestClient, cfg *config.Config) *StreamClient {

	slog.Println("stream client(websocket) initialized, listening for order updates")

	client := &StreamClient{
		config:       cfg,
		done:         make(chan struct{}),
		tradeClient:  tradeClient,
		restClient:   restClient,
		lastConnTime: time.Now(),
	}

	client.conn = NewStreamWebsocketConn(client, cfg)

	go client.messageHandler()

	return client
}

func SubscribeOrderUpdates(conn *websocket.Conn) {
	conn.WriteJSON(types.WSRequest{
		Op:   "subscribe",
		Args: []interface{}{"order.linear"},
	})
}

func (c *StreamClient) messageHandler() {
	defer func() {
		slog.Println("Message handler stopped")
	}()
	slog.Println("Message handler started")

	var (
		priceFloat                 float64
		stopPrice, takeProfitPrice float64
	)

	for {
		select {
		case <-c.done:
			return
		default:
			_, message, err := c.conn.ReadMessage()
			if err != nil {
				c.Reconnect()
				slog.Printf("websocket read message error, reconnecting: %v", err)
				return
			}

			var event types.EventData
			if err := json.Unmarshal(message, &event); err == nil {
				if event.Topic == "order.linear" {
					slog.Printf("event: %+v", event)
					if len(event.Data) == 0 {
						continue
					}
					order := event.Data[0]
					slog.Printf("data: %+v", order)
					if c.tradeClient.LastTrade == nil ||
						order.OrderType != "Market" ||
						order.OrderStatus != "Filled" {
						continue
					}
					lastTrade := c.tradeClient.LastTrade
					if lastTrade.Quantity != order.Qty {
						slog.Printf("tradeData.Quantity not equal to lastTrade.Quantity: %v != %v", order.Qty, lastTrade.Quantity)
						continue
					}
					priceFloat, _ = strconv.ParseFloat(order.AvgPrice, 64)
					if c.tradeClient.LastTrade.StopSide == types.TradeSellSide {
						stopPrice = utils.Truncate(priceFloat*(1-c.config.StopRatio), lastTrade.MinPrice)
						takeProfitPrice = utils.Truncate(priceFloat*(1+c.config.TakeProfitRatio), lastTrade.MinPrice)
					} else {
						stopPrice = utils.Truncate(priceFloat*(1+c.config.StopRatio), lastTrade.MinPrice)
						takeProfitPrice = utils.Truncate(priceFloat*(1-c.config.TakeProfitRatio), lastTrade.MinPrice)
					}
					go func() {
						c.tradeClient.PlaceReduceOnlyLimitOrder(
							lastTrade.Symbol,   // symbol
							lastTrade.StopSide, // side
							lastTrade.Quantity, // quantity
							takeProfitPrice,    // take profit price
						)
					}()
					go func() {
						c.tradeClient.CreateStopOrder(
							lastTrade.Symbol,   // symbol
							lastTrade.StopSide, // side
							lastTrade.Quantity, // quantity
							stopPrice,          // stop price
						)
					}()
					if c.config.BreakevenEnabled {
						go func() {
							tick := c.config.BreakevenWindowSize
							placeDuration := c.config.BreakevenPlaceDuration
							var delta float64
							if lastTrade.StopSide == types.TradeBuySide {
								delta = priceFloat * -c.config.BreakevenPercent
							} else {
								delta = priceFloat * c.config.BreakevenPercent
							}
							delta = utils.Truncate(delta, lastTrade.MinPrice)
							rawPrice := priceFloat
							cost := rawPrice + delta
							for i := 0; i < placeDuration; i++ {
								price := c.restClient.GetLatestPrice(lastTrade.Symbol)
								if price != -1 {
									if lastTrade.StopSide == types.TradeBuySide {
										if price < cost {
											tick--
											slog.Printf("current(%f) < cost(%f) + delta(%f), tick: %d", price, rawPrice, delta, tick)
											if tick == 0 {
												slog.Println("breakeven reached, creating stop order")
												c.tradeClient.CreateStopOrder(
													lastTrade.Symbol,   // symbol
													lastTrade.StopSide, // side
													lastTrade.Quantity, // quantity
													cost,               // stop price
												)
												break
											}
										} else {
											tick = c.config.BreakevenWindowSize
											slog.Printf("current(%f) >= cost(%f) + delta(%f), BAD, tick reset", price, rawPrice, delta)
										}
									} else {
										if price > cost {
											tick--
											slog.Printf("current(%f) > cost(%f) + delta(%f), tick: %d", price, rawPrice, delta, tick)
											if tick == 0 {
												slog.Println("breakeven reached, creating stop order")
												c.tradeClient.CreateStopOrder(
													lastTrade.Symbol,   // symbol
													lastTrade.StopSide, // side
													lastTrade.Quantity, // quantity
													cost,               // stop price
												)
												break
											}
										} else {
											tick = c.config.BreakevenWindowSize
											slog.Printf("current(%f) <= cost(%f) + delta(%f), BAD, tick reset", price, rawPrice, delta)
										}
									}
								}
								time.Sleep(1 * time.Second)
							}
						}()
					}
				} else if event.Op == "subscribe" {
					slog.Printf("subscribe event return: %+v", event.Success)
				} else if event.Op == "auth" {
					slog.Printf("auth event return: %+v", event.Success)
				} else if event.Op == "pong" {
					continue
				} else {
					slog.Printf("event: %+v", event)
				}
			} else {
				slog.Printf("error: %+v", err)
				var prettyJSON bytes.Buffer
				if err := json.Indent(&prettyJSON, message, "", "    "); err != nil {
					slog.Printf("Raw message: %s", string(message))
				} else {
					slog.Printf("收到消息:\n%s", prettyJSON.String())
				}
			}
		}
	}
}

func (c *StreamClient) Close() {
	close(*c.pongDone)
	close(c.done)
	c.conn.Close()
}

func (c *StreamClient) Reconnect() {
	c.conn.Close()
	c.conn = NewStreamWebsocketConn(c, c.config)
	c.lastConnTime = time.Now()
	go c.messageHandler()
	slog.Println("Stream client reconnected")
}

func (c *StreamClient) EnsureConnection() {
	if time.Since(c.lastConnTime) > 8*time.Hour {
		slog.Println("lastConnTime is greater than 8 hours, establishing new connection")
		c.Reconnect()
	}
}
