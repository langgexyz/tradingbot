package cex

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// TradingPair 标准化的交易对
type TradingPair struct {
	Base  string // 基础货币，如 BTC, ETH, PEPE
	Quote string // 计价货币，如 USDT, USDC, BTC
}

// String 返回标准化的交易对字符串表示
func (tp TradingPair) String() string {
	return tp.Base + "/" + tp.Quote
}

// KlineData 标准化的K线数据，基于 binance.KlineData 但添加了 TradingPair 字段
type KlineData struct {
	TradingPair         TradingPair     `json:"trading_pair"`
	OpenTime            time.Time       `json:"open_time"`              // 开盘时间
	Open                decimal.Decimal `json:"open"`                   // 开盘价
	High                decimal.Decimal `json:"high"`                   // 最高价
	Low                 decimal.Decimal `json:"low"`                    // 最低价
	Close               decimal.Decimal `json:"close"`                  // 收盘价
	Volume              decimal.Decimal `json:"volume"`                 // 成交量
	CloseTime           time.Time       `json:"close_time"`             // 收盘时间
	QuoteVolume         decimal.Decimal `json:"quote_volume"`           // 成交额
	TakerBuyVolume      decimal.Decimal `json:"taker_buy_volume"`       // 主动买入成交量
	TakerBuyQuoteVolume decimal.Decimal `json:"taker_buy_quote_volume"` // 主动买入成交额
}

// OrderSide 订单方向
type OrderSide string

const (
	OrderSideBuy  OrderSide = "BUY"
	OrderSideSell OrderSide = "SELL"
)

// OrderType 订单类型
type OrderType string

const (
	OrderTypeMarket OrderType = "MARKET"
	OrderTypeLimit  OrderType = "LIMIT"
)

// BuyOrderRequest 买入订单请求
type BuyOrderRequest struct {
	TradingPair TradingPair     `json:"trading_pair"`
	Type        OrderType       `json:"type"`
	Quantity    decimal.Decimal `json:"quantity"`
	Price       decimal.Decimal `json:"price,omitempty"` // 限价单时需要
}

// SellOrderRequest 卖出订单请求
type SellOrderRequest struct {
	TradingPair TradingPair     `json:"trading_pair"`
	Type        OrderType       `json:"type"`
	Quantity    decimal.Decimal `json:"quantity"`
	Price       decimal.Decimal `json:"price,omitempty"` // 限价单时需要
}

// OrderResult 订单结果
type OrderResult struct {
	TradingPair   TradingPair     `json:"trading_pair"`
	OrderID       string          `json:"order_id"`
	ClientOrderID string          `json:"client_order_id"`
	Price         decimal.Decimal `json:"price"`
	Quantity      decimal.Decimal `json:"quantity"`
	Side          OrderSide       `json:"side"`
	Status        string          `json:"status"`
	Type          OrderType       `json:"type"`
	TransactTime  time.Time       `json:"transact_time"`
}

// AccountBalance 账户余额
type AccountBalance struct {
	Asset  string          `json:"asset"`
	Free   decimal.Decimal `json:"free"`
	Locked decimal.Decimal `json:"locked"`
}

// CEXClient 中心化交易所客户端接口
type CEXClient interface {
	// GetName 获取交易所名称
	GetName() string

	// GetDatabase 获取数据库连接
	GetDatabase() interface{} // 返回数据库接口，具体类型由实现决定

	// GetTradingFee 获取交易手续费率
	GetTradingFee() float64

	// GetKlines 获取K线数据
	GetKlines(ctx context.Context, pair TradingPair, interval string, limit int) ([]*KlineData, error)

	// GetKlinesWithTimeRange 获取指定时间范围的K线数据
	GetKlinesWithTimeRange(ctx context.Context, pair TradingPair, interval string, startTime, endTime time.Time, limit int) ([]*KlineData, error)

	// Buy 买入
	Buy(ctx context.Context, order BuyOrderRequest) (*OrderResult, error)

	// Sell 卖出
	Sell(ctx context.Context, order SellOrderRequest) (*OrderResult, error)

	// GetAccount 获取账户信息
	GetAccount(ctx context.Context) ([]*AccountBalance, error)

	// Ping 测试连接
	Ping(ctx context.Context) error
}
