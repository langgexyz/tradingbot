package strategy

import (
	"context"

	"tradingbot/src/cex"
	"tradingbot/src/executor"
)

// Signal 交易信号
type Signal struct {
	Type      string  `json:"type"`      // "BUY", "SELL", "CLOSE"
	Reason    string  `json:"reason"`    // 信号原因
	Strength  float64 `json:"strength"`  // 信号强度 0-1
	Timestamp int64   `json:"timestamp"` // 信号时间戳
}

// StrategyParams 策略参数接口
type StrategyParams interface {
	// Validate 验证参数有效性
	Validate() error
}

// Strategy 交易策略接口
type Strategy interface {
	// OnData 处理新的K线数据，返回交易信号
	OnData(ctx context.Context, kline *cex.KlineData, portfolio *executor.Portfolio) ([]*Signal, error)

	// GetName 获取策略名称
	GetName() string

	// GetParams 获取策略参数
	GetParams() StrategyParams

	// SetParams 设置策略参数
	SetParams(params StrategyParams) error
}
