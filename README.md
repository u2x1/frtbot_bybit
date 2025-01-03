# frtbot_bybit

## Usage

```bash
go run main.go
```

## Config

| Key | Type | Description |
| --- | --- | --- |
| api_key | string | HMAC API key |
| hmac_secret | string | HMAC secret |
| test_mode | bool | 是否使用模拟盘 |
| test_api_key | string | 模拟盘 API key |
| test_hmac_secret | string | 模拟盘 HMAC secret |
| margin | float64 | 保证金(USDT) |
| first_order_time_offset_ms | int64 | 下单时间偏移(ms)，如果资金费率在8:00:00结算，偏移设置为-100，则会在7:59:59.900下单 |
| min_funding_rate_percent | float64 | 最低资金费率绝对值(%) |
| stop_percent | float64 | 止损百分比(%) |
| take_profit_percent | float64 | 止盈百分比(%) |
| margin_type | string | 账户保证金类型，可选值：ISOLATED(逐仓), REGULAR(全仓), PORTFOLIO(组合保证金) |
| leverage | int | 杠杆倍数 |
| breakeven_enabled | bool | 是否启用保本止损 |
| breakeven_percent | float64 | 保本止损百分比(%)，0% 表示成本价(不计算手续费) |
| breakeven_window_size | int | 保本止损窗口大小(秒) |
| breakeven_place_duration | int | 在下单后，检测"是否进行保本止损"的持续时间(秒) |
