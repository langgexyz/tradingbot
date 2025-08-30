package executor

import (
	"context"
	"time"

	"tradingbot/src/cex"

	"github.com/shopspring/decimal"
)

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

// BuyOrder 买入订单信息
type BuyOrder struct {
	ID          string          `json:"id"`
	TradingPair cex.TradingPair `json:"trading_pair"`
	Type        OrderType       `json:"type"`
	Quantity    decimal.Decimal `json:"quantity"`
	Price       decimal.Decimal `json:"price"` // 限价单价格，市价单可为空
	Timestamp   time.Time       `json:"timestamp"`
	Reason      string          `json:"reason"` // 交易原因
}

// SellOrder 卖出订单信息
type SellOrder struct {
	ID          string          `json:"id"`
	TradingPair cex.TradingPair `json:"trading_pair"`
	Type        OrderType       `json:"type"`
	Quantity    decimal.Decimal `json:"quantity"`
	Price       decimal.Decimal `json:"price"` // 限价单价格，市价单可为空
	Timestamp   time.Time       `json:"timestamp"`
	Reason      string          `json:"reason"` // 交易原因
}

// OrderResult 订单执行结果
type OrderResult struct {
	OrderID     string          `json:"order_id"`
	TradingPair cex.TradingPair `json:"trading_pair"`
	Side        OrderSide       `json:"side"`
	Quantity    decimal.Decimal `json:"quantity"`
	Price       decimal.Decimal `json:"price"`      // 实际成交价格
	Commission  decimal.Decimal `json:"commission"` // 手续费
	Timestamp   time.Time       `json:"timestamp"`
	Success     bool            `json:"success"`
	Error       string          `json:"error,omitempty"`
}

// Portfolio 投资组合状态
type Portfolio struct {
	Cash         decimal.Decimal `json:"cash"`          // 现金余额
	Position     decimal.Decimal `json:"position"`      // 持仓数量
	CurrentPrice decimal.Decimal `json:"current_price"` // 当前价格
	Portfolio    decimal.Decimal `json:"portfolio"`     // 总资产价值
	Timestamp    time.Time       `json:"timestamp"`
}

// Executor 交易执行器接口
type Executor interface {
	// Buy 执行买入订单
	Buy(ctx context.Context, order *BuyOrder) (*OrderResult, error)

	// Sell 执行卖出订单
	Sell(ctx context.Context, order *SellOrder) (*OrderResult, error)

	// GetPortfolio 获取当前投资组合状态
	GetPortfolio(ctx context.Context) (*Portfolio, error)

	// GetName 获取执行器名称
	GetName() string

	// Close 关闭执行器，清理资源
	Close() error
}
