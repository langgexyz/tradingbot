package strategies

import (
	"context"
	"fmt"

	"go-build-stream-gateway-go-server-main/src/binance"
	"go-build-stream-gateway-go-server-main/src/executor"
	"go-build-stream-gateway-go-server-main/src/indicators"
	"go-build-stream-gateway-go-server-main/src/strategy"

	"github.com/shopspring/decimal"
)

// BollingerBandsStrategy 布林道策略
type BollingerBandsStrategy struct {
	// 策略参数
	Period              int     `json:"period"`
	Multiplier          float64 `json:"multiplier"`
	PositionSizePercent float64 `json:"position_size_percent"`
	MinTradeAmount      float64 `json:"min_trade_amount"`
	StopLossPercent     float64 `json:"stop_loss_percent"`
	TakeProfitPercent   float64 `json:"take_profit_percent"`
	CooldownBars        int     `json:"cooldown_bars"`

	// 内部状态
	bb             *indicators.BollingerBands
	priceHistory   []decimal.Decimal
	currentBar     int
	lastTradeBar   int
	lastTradePrice decimal.Decimal
}

// NewBollingerBandsStrategy 创建布林道策略
func NewBollingerBandsStrategy() *BollingerBandsStrategy {
	return &BollingerBandsStrategy{
		Period:              20,
		Multiplier:          2.0,
		PositionSizePercent: 0.95,
		MinTradeAmount:      10.0,
		StopLossPercent:     1.0, // 100%止损 = 永不止损
		TakeProfitPercent:   0.5, // 50%止盈
		CooldownBars:        3,
		lastTradeBar:        -1,
		priceHistory:        make([]decimal.Decimal, 0),
	}
}

// GetName 获取策略名称
func (s *BollingerBandsStrategy) GetName() string {
	return "Bollinger Bands Strategy"
}

// GetParams 获取策略参数
func (s *BollingerBandsStrategy) GetParams() map[string]interface{} {
	return map[string]interface{}{
		"period":                s.Period,
		"multiplier":            s.Multiplier,
		"position_size_percent": s.PositionSizePercent,
		"min_trade_amount":      s.MinTradeAmount,
		"stop_loss_percent":     s.StopLossPercent,
		"take_profit_percent":   s.TakeProfitPercent,
		"cooldown_bars":         s.CooldownBars,
	}
}

// SetParams 设置策略参数
func (s *BollingerBandsStrategy) SetParams(params map[string]interface{}) error {
	if period, ok := params["period"].(int); ok {
		s.Period = period
	}
	if multiplier, ok := params["multiplier"].(float64); ok {
		s.Multiplier = multiplier
	}
	if positionSizePercent, ok := params["position_size_percent"].(float64); ok {
		s.PositionSizePercent = positionSizePercent
	}
	if minTradeAmount, ok := params["min_trade_amount"].(float64); ok {
		s.MinTradeAmount = minTradeAmount
	}
	if stopLossPercent, ok := params["stop_loss_percent"].(float64); ok {
		s.StopLossPercent = stopLossPercent
	}
	if takeProfitPercent, ok := params["take_profit_percent"].(float64); ok {
		s.TakeProfitPercent = takeProfitPercent
	}
	if cooldownBars, ok := params["cooldown_bars"].(int); ok {
		s.CooldownBars = cooldownBars
	}

	// 重新创建布林道指标
	s.bb = indicators.NewBollingerBands(s.Period, s.Multiplier)
	return nil
}

// OnData 处理新的K线数据
func (s *BollingerBandsStrategy) OnData(ctx context.Context, kline *binance.KlineData, portfolio *executor.Portfolio) ([]*strategy.Signal, error) {
	s.currentBar++

	// 添加价格到历史数据
	s.priceHistory = append(s.priceHistory, kline.Close)

	// 保持历史数据长度
	maxHistory := s.Period + 10
	if len(s.priceHistory) > maxHistory {
		s.priceHistory = s.priceHistory[1:]
	}

	// 检查是否有足够的数据计算布林道
	if len(s.priceHistory) < s.Period {
		return nil, nil
	}

	// 计算布林道指标
	bbResult, err := s.bb.Calculate(s.priceHistory)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate Bollinger Bands: %w", err)
	}

	bbResult.Timestamp = kline.OpenTime

	var signals []*strategy.Signal

	// 检查冷却期
	if s.lastTradeBar >= 0 && s.currentBar-s.lastTradeBar < s.CooldownBars {
		// 仍在冷却期，只检查止损止盈
		stopSignals := s.checkStopConditions(kline, portfolio)
		signals = append(signals, stopSignals...)
		return signals, nil
	}

	// 检查止损止盈条件
	stopSignals := s.checkStopConditions(kline, portfolio)
	signals = append(signals, stopSignals...)

	// 如果有止损止盈信号，不再生成新的开仓信号
	if len(stopSignals) > 0 {
		return signals, nil
	}

	// 生成交易信号
	tradeSignals := s.generateTradeSignals(bbResult, kline, portfolio)
	signals = append(signals, tradeSignals...)

	return signals, nil
}

// generateTradeSignals 生成交易信号
func (s *BollingerBandsStrategy) generateTradeSignals(bb *indicators.BollingerBandsResult, kline *binance.KlineData, portfolio *executor.Portfolio) []*strategy.Signal {
	var signals []*strategy.Signal

	currentPrice := kline.Close

	// 买入信号：价格触及下轨且无持仓
	if currentPrice.LessThanOrEqual(bb.LowerBand) && portfolio.Position.IsZero() {
		signals = append(signals, &strategy.Signal{
			Type:      "BUY",
			Reason:    fmt.Sprintf("price %.4f touched lower band %.4f", currentPrice.InexactFloat64(), bb.LowerBand.InexactFloat64()),
			Strength:  0.8,
			Timestamp: kline.OpenTime,
		})

		s.lastTradeBar = s.currentBar
		s.lastTradePrice = currentPrice
	}

	// 卖出信号：价格触及上轨且有持仓
	if currentPrice.GreaterThanOrEqual(bb.UpperBand) && !portfolio.Position.IsZero() {
		signals = append(signals, &strategy.Signal{
			Type:      "SELL",
			Reason:    fmt.Sprintf("price %.4f touched upper band %.4f", currentPrice.InexactFloat64(), bb.UpperBand.InexactFloat64()),
			Strength:  0.8,
			Timestamp: kline.OpenTime,
		})

		s.lastTradeBar = s.currentBar
		s.lastTradePrice = decimal.Zero
	}

	return signals
}

// checkStopConditions 检查止损止盈条件
func (s *BollingerBandsStrategy) checkStopConditions(kline *binance.KlineData, portfolio *executor.Portfolio) []*strategy.Signal {
	var signals []*strategy.Signal

	// 只有持有仓位时才检查止损止盈
	if portfolio.Position.IsZero() || s.lastTradePrice.IsZero() {
		return signals
	}

	currentPrice := kline.Close
	pnl := currentPrice.Sub(s.lastTradePrice)
	pnlPercent := pnl.Div(s.lastTradePrice)

	// 止损检查
	stopLossThreshold := decimal.NewFromFloat(-s.StopLossPercent)
	if pnlPercent.LessThanOrEqual(stopLossThreshold) {
		signals = append(signals, &strategy.Signal{
			Type:      "SELL",
			Reason:    fmt.Sprintf("stop loss: %.2f%%", pnlPercent.Mul(decimal.NewFromInt(100)).InexactFloat64()),
			Strength:  1.0,
			Timestamp: kline.OpenTime,
		})

		s.lastTradeBar = s.currentBar
		s.lastTradePrice = decimal.Zero
		return signals
	}

	// 止盈检查
	takeProfitThreshold := decimal.NewFromFloat(s.TakeProfitPercent)
	if pnlPercent.GreaterThanOrEqual(takeProfitThreshold) {
		signals = append(signals, &strategy.Signal{
			Type:      "SELL",
			Reason:    fmt.Sprintf("take profit: %.2f%%", pnlPercent.Mul(decimal.NewFromInt(100)).InexactFloat64()),
			Strength:  1.0,
			Timestamp: kline.OpenTime,
		})

		s.lastTradeBar = s.currentBar
		s.lastTradePrice = decimal.Zero
	}

	return signals
}
