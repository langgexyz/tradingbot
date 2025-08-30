package strategies

import (
	"context"
	"fmt"
	"time"

	"go-build-stream-gateway-go-server-main/src/backtest"
	"go-build-stream-gateway-go-server-main/src/binance"
	"go-build-stream-gateway-go-server-main/src/indicators"

	"github.com/shopspring/decimal"
)

// BollingerBandsStrategy 布林道交易策略
type BollingerBandsStrategy struct {
	// 策略参数
	Period              int             `json:"period"`                // 布林道周期，默认20
	Multiplier          float64         `json:"multiplier"`            // 标准差倍数，默认2.0
	PositionSizePercent float64         `json:"position_size_percent"` // 仓位大小百分比，默认0.95
	MinTradeAmount      decimal.Decimal `json:"min_trade_amount"`      // 最小交易金额
	StopLossPercent     float64         `json:"stop_loss_percent"`     // 止损百分比，默认0.05 (5%)
	TakeProfitPercent   float64         `json:"take_profit_percent"`   // 止盈百分比，默认0.10 (10%)

	// 内部状态
	bb             *indicators.BollingerBands
	priceHistory   []decimal.Decimal
	lastSignal     string // "buy", "sell", "none"
	lastTradePrice decimal.Decimal

	// 冷却期设置（避免频繁交易）
	CooldownBars int `json:"cooldown_bars"` // 交易后冷却K线数量
	lastTradeBar int // 最后交易的K线索引
	currentBar   int // 当前K线索引
}

// NewBollingerBandsStrategy 创建新的布林道策略
func NewBollingerBandsStrategy() *BollingerBandsStrategy {
	strategy := &BollingerBandsStrategy{
		Period:              20,
		Multiplier:          2.0,
		PositionSizePercent: 0.95,
		MinTradeAmount:      decimal.NewFromFloat(10.0), // 最小10 USDT
		StopLossPercent:     0.05,                       // 5% 止损
		TakeProfitPercent:   0.10,                       // 10% 止盈
		CooldownBars:        3,                          // 3根K线冷却期
		lastSignal:          "none",
		priceHistory:        make([]decimal.Decimal, 0),
		lastTradeBar:        -1,
		currentBar:          0,
	}

	strategy.bb = indicators.NewBollingerBands(strategy.Period, strategy.Multiplier)
	return strategy
}

// SetParams 设置策略参数
func (s *BollingerBandsStrategy) SetParams(params map[string]interface{}) error {
	if period, ok := params["period"].(int); ok {
		s.Period = period
	}
	if multiplier, ok := params["multiplier"].(float64); ok {
		s.Multiplier = multiplier
	}
	if positionSize, ok := params["position_size_percent"].(float64); ok {
		s.PositionSizePercent = positionSize
	}
	if minAmount, ok := params["min_trade_amount"].(float64); ok {
		s.MinTradeAmount = decimal.NewFromFloat(minAmount)
	}
	if stopLoss, ok := params["stop_loss_percent"].(float64); ok {
		s.StopLossPercent = stopLoss
	}
	if takeProfit, ok := params["take_profit_percent"].(float64); ok {
		s.TakeProfitPercent = takeProfit
	}
	if cooldown, ok := params["cooldown_bars"].(int); ok {
		s.CooldownBars = cooldown
	}

	// 重新创建布林道指标
	s.bb = indicators.NewBollingerBands(s.Period, s.Multiplier)
	return nil
}

// OnData 处理新的K线数据
func (s *BollingerBandsStrategy) OnData(ctx context.Context, kline *binance.KlineData, portfolio *backtest.Portfolio) ([]*backtest.Signal, error) {
	s.currentBar++

	// 添加价格到历史数据
	s.priceHistory = append(s.priceHistory, kline.Close)

	// 保持历史数据长度
	maxHistory := s.Period + 10 // 保留多一些数据用于指标计算
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

	var signals []*backtest.Signal

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
func (s *BollingerBandsStrategy) generateTradeSignals(bbResult *indicators.BollingerBandsResult, kline *binance.KlineData, portfolio *backtest.Portfolio) []*backtest.Signal {
	var signals []*backtest.Signal
	currentPrice := kline.Close

	// 买入信号：价格触及或跌破下轨
	if bbResult.IsLowerBreakout() && portfolio.Position.IsZero() && s.lastSignal != "buy" {
		// 计算买入数量
		availableCash := portfolio.Cash.Mul(decimal.NewFromFloat(s.PositionSizePercent))

		if availableCash.GreaterThan(s.MinTradeAmount) {
			quantity := availableCash.Div(currentPrice)

			currentPriceFloat, _ := currentPrice.Float64()
			lowerBandFloat, _ := bbResult.LowerBand.Float64()

			signal := &backtest.Signal{
				Type:      "buy",
				Symbol:    kline.Symbol,
				Quantity:  quantity,
				OrderType: "market",
				Reason:    fmt.Sprintf("价格 %.4f 跌破布林道下轨 %.4f", currentPriceFloat, lowerBandFloat),
				Timestamp: time.Unix(kline.OpenTime/1000, 0),
			}

			signals = append(signals, signal)
			s.lastSignal = "buy"
			s.lastTradePrice = currentPrice
			s.lastTradeBar = s.currentBar
		}
	}

	// 卖出信号：价格触及或突破上轨
	if bbResult.IsUpperBreakout() && portfolio.Position.GreaterThan(decimal.Zero) && s.lastSignal != "sell" {
		currentPriceFloat, _ := currentPrice.Float64()
		upperBandFloat, _ := bbResult.UpperBand.Float64()

		signal := &backtest.Signal{
			Type:      "sell",
			Symbol:    kline.Symbol,
			Quantity:  portfolio.Position,
			OrderType: "market",
			Reason:    fmt.Sprintf("价格 %.4f 突破布林道上轨 %.4f", currentPriceFloat, upperBandFloat),
			Timestamp: time.Unix(kline.OpenTime/1000, 0),
		}

		signals = append(signals, signal)
		s.lastSignal = "sell"
		s.lastTradePrice = currentPrice
		s.lastTradeBar = s.currentBar
	}

	return signals
}

// checkStopConditions 检查止损止盈条件
func (s *BollingerBandsStrategy) checkStopConditions(kline *binance.KlineData, portfolio *backtest.Portfolio) []*backtest.Signal {
	var signals []*backtest.Signal

	// 只有持有仓位时才检查止损止盈
	if portfolio.Position.IsZero() || s.lastTradePrice.IsZero() {
		return signals
	}

	currentPrice := kline.Close

	// 计算盈亏百分比
	if s.lastSignal == "buy" {
		// 做多仓位
		pnlPercent := currentPrice.Sub(s.lastTradePrice).Div(s.lastTradePrice)
		pnlPercentFloat, _ := pnlPercent.Float64()

		// 止损：亏损超过设定百分比
		if pnlPercentFloat <= -s.StopLossPercent {
			signal := &backtest.Signal{
				Type:      "sell",
				Symbol:    kline.Symbol,
				Quantity:  portfolio.Position,
				OrderType: "market",
				Reason:    fmt.Sprintf("止损：亏损 %.2f%%", pnlPercentFloat*100),
				Timestamp: time.Unix(kline.OpenTime/1000, 0),
			}
			signals = append(signals, signal)
			s.lastSignal = "sell"
			s.lastTradePrice = currentPrice
			s.lastTradeBar = s.currentBar
		}

		// 止盈：盈利超过设定百分比
		if pnlPercentFloat >= s.TakeProfitPercent {
			signal := &backtest.Signal{
				Type:      "sell",
				Symbol:    kline.Symbol,
				Quantity:  portfolio.Position,
				OrderType: "market",
				Reason:    fmt.Sprintf("止盈：盈利 %.2f%%", pnlPercentFloat*100),
				Timestamp: time.Unix(kline.OpenTime/1000, 0),
			}
			signals = append(signals, signal)
			s.lastSignal = "sell"
			s.lastTradePrice = currentPrice
			s.lastTradeBar = s.currentBar
		}
	}

	return signals
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

// GetCurrentBBResult 获取当前布林道指标结果（用于分析）
func (s *BollingerBandsStrategy) GetCurrentBBResult() (*indicators.BollingerBandsResult, error) {
	if len(s.priceHistory) < s.Period {
		return nil, fmt.Errorf("insufficient data")
	}

	return s.bb.Calculate(s.priceHistory)
}

// Reset 重置策略状态
func (s *BollingerBandsStrategy) Reset() {
	s.priceHistory = make([]decimal.Decimal, 0)
	s.lastSignal = "none"
	s.lastTradePrice = decimal.Zero
	s.lastTradeBar = -1
	s.currentBar = 0
}

// GetSignalHistory 获取信号历史（用于分析）
func (s *BollingerBandsStrategy) GetSignalHistory() map[string]interface{} {
	return map[string]interface{}{
		"last_signal":          s.lastSignal,
		"last_trade_price":     s.lastTradePrice,
		"last_trade_bar":       s.lastTradeBar,
		"current_bar":          s.currentBar,
		"price_history_length": len(s.priceHistory),
	}
}
