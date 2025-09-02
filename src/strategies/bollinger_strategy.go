package strategies

import (
	"context"
	"fmt"

	"tradingbot/src/cex"
	"tradingbot/src/executor"
	"tradingbot/src/indicators"
	"tradingbot/src/strategy"

	"github.com/shopspring/decimal"
	"github.com/xpwu/go-log/log"
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

	// 卖出策略参数
	SellStrategyName string `json:"sell_strategy_name"`

	// 内部状态
	bb             *indicators.BollingerBands
	priceHistory   []decimal.Decimal
	currentBar     int
	lastTradeBar   int
	lastTradePrice decimal.Decimal

	// 🔥 新增：跟踪持仓期间最高价格（移动止盈关键字段）
	highestPriceSinceBuy decimal.Decimal
	hasBought            bool

	// 卖出策略
	sellStrategy strategy.SellStrategy
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
func (s *BollingerBandsStrategy) GetParams() strategy.StrategyParams {
	return &strategy.BollingerBandsParams{
		Period:              s.Period,
		Multiplier:          s.Multiplier,
		PositionSizePercent: s.PositionSizePercent,
		MinTradeAmount:      s.MinTradeAmount,
		StopLossPercent:     s.StopLossPercent,
		TakeProfitPercent:   s.TakeProfitPercent,
		CooldownBars:        s.CooldownBars,
	}
}

// SetParams 设置策略参数
func (s *BollingerBandsStrategy) SetParams(params strategy.StrategyParams) error {
	if bollingerParams, ok := params.(*strategy.BollingerBandsParams); ok {
		s.Period = bollingerParams.Period
		s.Multiplier = bollingerParams.Multiplier
		s.PositionSizePercent = bollingerParams.PositionSizePercent
		s.MinTradeAmount = bollingerParams.MinTradeAmount
		s.StopLossPercent = bollingerParams.StopLossPercent
		s.TakeProfitPercent = bollingerParams.TakeProfitPercent
		s.CooldownBars = bollingerParams.CooldownBars

		// 设置卖出策略
		s.SellStrategyName = bollingerParams.SellStrategyName

		// 创建卖出策略实例，统一使用 CreateSellStrategyWithParams（支持预设名称和直接类型）
		sellStrategy, err := strategy.CreateSellStrategyWithParams(s.SellStrategyName, bollingerParams.SellStrategyParams)
		if err == nil {
			s.sellStrategy = sellStrategy
		}
	} else {
		return fmt.Errorf("invalid parameter type, expected *strategy.BollingerBandsParams")
	}

	// 重新创建布林道指标
	s.bb = indicators.NewBollingerBands(s.Period, s.Multiplier)
	return nil
}

// OnData 处理新的K线数据
func (s *BollingerBandsStrategy) OnData(ctx context.Context, kline *cex.KlineData, portfolio *executor.Portfolio) ([]*strategy.Signal, error) {
	ctx, logger := log.WithCtx(ctx)
	logger.PushPrefix("BollingerStrategy")

	s.currentBar++

	// 只在有持仓变化或重要节点时打印状态
	if s.currentBar == 1 || (s.currentBar%50 == 0 && !portfolio.Position.IsZero()) {
		logger.Info(fmt.Sprintf("📊 交易状态: Bar #%d, 价格 %s, 持仓 %s", 
			s.currentBar, kline.Close.String(), portfolio.Position.String()))
	}

	// 添加价格到历史数据
	s.priceHistory = append(s.priceHistory, kline.Close)

	// 保持历史数据长度
	maxHistory := s.Period + 10
	if len(s.priceHistory) > maxHistory {
		s.priceHistory = s.priceHistory[1:]
	}

	// 检查是否有足够的数据计算布林道
	if len(s.priceHistory) < s.Period {
		// 只在即将完成时打印一次
		if len(s.priceHistory) == s.Period-1 {
			logger.Info(fmt.Sprintf("⚡ 数据积累完成，准备开始交易分析"))
		}
		return nil, nil
	}

	// 计算布林道指标
	bbResult, err := s.bb.Calculate(s.priceHistory)
	if err != nil {
		logger.Error("❌ 布林带计算失败", "error", err)
		return nil, fmt.Errorf("failed to calculate Bollinger Bands: %w", err)
	}

	bbResult.Timestamp = kline.OpenTime.Unix() * 1000

	// 删除过于频繁的边界检测日志，在交易信号中会有更有意义的日志

	var signals []*strategy.Signal

	// 检查冷却期
	inCooldown := s.lastTradeBar >= 0 && s.currentBar-s.lastTradeBar < s.CooldownBars
	if inCooldown {
		cooldownRemaining := s.CooldownBars - (s.currentBar - s.lastTradeBar)
		logger.Info("⏳ 处于冷却期",
			"last_trade_bar", s.lastTradeBar,
			"current_bar", s.currentBar,
			"cooldown_remaining", cooldownRemaining)

		// 仍在冷却期，只检查止损止盈
		stopSignals := s.checkStopConditions(ctx, kline, portfolio)
		signals = append(signals, stopSignals...)
		return signals, nil
	}

	// 移除冗余的冷却期日志

	// 检查止损止盈条件
	stopSignals := s.checkStopConditions(ctx, kline, portfolio)
	signals = append(signals, stopSignals...)

	// 如果有止损止盈信号，不再生成新的开仓信号
	if len(stopSignals) > 0 {
		logger.Info(fmt.Sprintf("🛑 生成止损止盈信号，跳过开仓信号: stop_signals_count=%d", len(stopSignals)))
		return signals, nil
	}

	// 生成交易信号
	tradeSignals := s.generateTradeSignals(ctx, bbResult, kline, portfolio)
	signals = append(signals, tradeSignals...)

	// 信号详情已在生成时记录，此处无需重复

	return signals, nil
}

// generateTradeSignals 生成交易信号
func (s *BollingerBandsStrategy) generateTradeSignals(ctx context.Context, bb *indicators.BollingerBandsResult, kline *cex.KlineData, portfolio *executor.Portfolio) []*strategy.Signal {
	ctx, logger := log.WithCtx(ctx)
	var signals []*strategy.Signal

	currentPrice := kline.Close

	// 🔥 更新持仓期间最高价格
	if s.hasBought && currentPrice.GreaterThan(s.highestPriceSinceBuy) {
		s.highestPriceSinceBuy = currentPrice
	}

	// 简化买入条件分析日志（只在满足条件时打印）

	// 买入信号：价格触及下轨且无持仓
	if currentPrice.LessThanOrEqual(bb.LowerBand) && portfolio.Position.IsZero() {
		reason := fmt.Sprintf("price %.8f touched lower band %.8f", currentPrice.InexactFloat64(), bb.LowerBand.InexactFloat64())
		logger.Info("")  // 空行分隔
		logger.Info(fmt.Sprintf("✅ 买入条件满足: reason=%s, signal_strength=%.1f", reason, 0.8))

		signals = append(signals, &strategy.Signal{
			Type:      "BUY",
			Reason:    reason,
			Strength:  0.8,
			Timestamp: kline.OpenTime.Unix() * 1000,
		})

		s.lastTradeBar = s.currentBar
		s.lastTradePrice = currentPrice

		// 🔥 初始化移动止盈跟踪
		s.hasBought = true
		s.highestPriceSinceBuy = currentPrice

		// 交易状态已在买入信号中记录，此处无需重复日志
	} else {
		// 不买入时无需记录日志，避免噪音
	}

	// 卖出决策完全由SellStrategy处理，这里不再生成卖出信号

	return signals
}

// checkStopConditions 检查止损止盈条件（使用卖出策略）
func (s *BollingerBandsStrategy) checkStopConditions(ctx context.Context, kline *cex.KlineData, portfolio *executor.Portfolio) []*strategy.Signal {
	ctx, logger := log.WithCtx(ctx)
	var signals []*strategy.Signal

	// 只有持有仓位时才检查止损止盈
	if portfolio.Position.IsZero() || s.lastTradePrice.IsZero() {
		// 跳过止损止盈检查（无持仓或无价格记录）
		return signals
	}

	currentPrice := kline.Close
	pnl := currentPrice.Sub(s.lastTradePrice)
	pnlPercent := pnl.Div(s.lastTradePrice)

	// 简化盈亏日志 - 只在关键时刻打印
	stopLossThreshold := decimal.NewFromFloat(-s.StopLossPercent)
	willStopLoss := pnlPercent.LessThanOrEqual(stopLossThreshold)

	// 只在即将止损时打印详细信息
	if willStopLoss {
		logger.Info("💰 持仓分析",
			"entry", s.lastTradePrice.String(),
			"current", currentPrice.String(),
			"pnl%", pnlPercent.Mul(decimal.NewFromInt(100)).String())
	}

	if pnlPercent.LessThanOrEqual(stopLossThreshold) {
		reason := fmt.Sprintf("stop loss: %.2f%%", pnlPercent.Mul(decimal.NewFromInt(100)).InexactFloat64())
		logger.Info(fmt.Sprintf("🚨 触发止损: reason=%s", reason))

		signals = append(signals, &strategy.Signal{
			Type:      "SELL",
			Reason:    reason,
			Strength:  1.0,
			Timestamp: kline.OpenTime.Unix() * 1000,
		})
		s.resetTradeState()
		return signals
	}

	// 2. 使用卖出策略检查
	if s.sellStrategy != nil {
		// 创建交易信息
		tradeInfo := &strategy.TradeInfo{
			EntryPrice:   s.lastTradePrice,
			CurrentPrice: currentPrice,
			CurrentPnL:   pnlPercent,
			HighestPrice: s.highestPriceSinceBuy,
		}

		sellSignal := s.sellStrategy.ShouldSell(kline, tradeInfo)

		if sellSignal.ShouldSell {
			logger.Info("")  // 空行分隔
			logger.Info(fmt.Sprintf("✅ 卖出触发: reason=%s, strength=%.1f", sellSignal.Reason, sellSignal.Strength))

			signals = append(signals, &strategy.Signal{
				Type:      "SELL",
				Reason:    sellSignal.Reason,
				Strength:  sellSignal.Strength,
				Timestamp: kline.OpenTime.Unix() * 1000,
			})
			s.resetTradeState()
			return signals
		}
	} else {
		// 3. 兜底：基础止盈检查
		takeProfitThreshold := decimal.NewFromFloat(s.TakeProfitPercent)
		if pnlPercent.GreaterThanOrEqual(takeProfitThreshold) {
			reason := fmt.Sprintf("take profit: %.2f%%", pnlPercent.Mul(decimal.NewFromInt(100)).InexactFloat64())
			logger.Info(fmt.Sprintf("💎 触发基础止盈: reason=%s", reason))

			signals = append(signals, &strategy.Signal{
				Type:      "SELL",
				Reason:    reason,
				Strength:  1.0,
				Timestamp: kline.OpenTime.Unix() * 1000,
			})
			s.resetTradeState()
		}
	}

	return signals
}

// resetTradeState 重置交易状态
func (s *BollingerBandsStrategy) resetTradeState() {
	s.lastTradeBar = s.currentBar
	s.lastTradePrice = decimal.Zero

	// 🔥 重置移动止盈状态
	s.hasBought = false
	s.highestPriceSinceBuy = decimal.Zero

	if s.sellStrategy != nil {
		s.sellStrategy.Reset()
	}
}
