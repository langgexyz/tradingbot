// checkStopConditions 检查止损止盈条件（使用卖出策略）
func (s *BollingerBandsStrategy) checkStopConditions(kline *cex.KlineData, portfolio *executor.Portfolio) []*strategy.Signal {
	var signals []*strategy.Signal

	// 只有持有仓位时才检查止损止盈
	if portfolio.Position.IsZero() || s.lastTradePrice.IsZero() {
		return signals
	}

	currentPrice := kline.Close
	pnl := currentPrice.Sub(s.lastTradePrice)
	pnlPercent := pnl.Div(s.lastTradePrice)

	// 1. 止损检查（优先级最高）
	stopLossThreshold := decimal.NewFromFloat(-s.StopLossPercent)
	if pnlPercent.LessThanOrEqual(stopLossThreshold) {
		signals = append(signals, &strategy.Signal{
			Type:      "SELL",
			Reason:    fmt.Sprintf("stop loss: %.2f%%", pnlPercent.Mul(decimal.NewFromInt(100)).InexactFloat64()),
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
			// TODO: 添加其他必要信息
		}

		sellSignal := s.sellStrategy.ShouldSell(kline, tradeInfo)
		if sellSignal.ShouldSell {
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
			signals = append(signals, &strategy.Signal{
				Type:      "SELL",
				Reason:    fmt.Sprintf("take profit: %.2f%%", pnlPercent.Mul(decimal.NewFromInt(100)).InexactFloat64()),
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
	if s.sellStrategy != nil {
		s.sellStrategy.Reset()
	}
}
