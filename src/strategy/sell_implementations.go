package strategy

import (
	"fmt"
	"tradingbot/src/cex"

	"github.com/shopspring/decimal"
)

// FixedSellStrategy 固定止盈策略
type FixedSellStrategy struct {
	TakeProfitPercent float64
}

func NewFixedSellStrategy(takeProfitPercent float64) *FixedSellStrategy {
	return &FixedSellStrategy{
		TakeProfitPercent: takeProfitPercent,
	}
}

func (s *FixedSellStrategy) ShouldSell(kline *cex.KlineData, tradeInfo *TradeInfo) *SellSignal {
	threshold := decimal.NewFromFloat(s.TakeProfitPercent)
	if tradeInfo.CurrentPnL.GreaterThanOrEqual(threshold) {
		return &SellSignal{
			ShouldSell: true,
			Reason:     fmt.Sprintf("fixed take profit: %.2f%%", tradeInfo.CurrentPnL.Mul(decimal.NewFromInt(100)).InexactFloat64()),
			Strength:   1.0,
		}
	}
	return &SellSignal{ShouldSell: false}
}

func (s *FixedSellStrategy) GetName() string {
	return fmt.Sprintf("Fixed(%.1f%%)", s.TakeProfitPercent*100)
}

func (s *FixedSellStrategy) Reset() {}

// TrailingSellStrategy 移动止盈策略
type TrailingSellStrategy struct {
	TrailingPercent      float64
	MinProfitForTrailing float64
}

func NewTrailingSellStrategy(trailingPercent, minProfitForTrailing float64) *TrailingSellStrategy {
	return &TrailingSellStrategy{
		TrailingPercent:      trailingPercent,
		MinProfitForTrailing: minProfitForTrailing,
	}
}

func (s *TrailingSellStrategy) ShouldSell(kline *cex.KlineData, tradeInfo *TradeInfo) *SellSignal {
	// 必须先达到最小盈利才启用移动止盈
	minProfit := decimal.NewFromFloat(s.MinProfitForTrailing)
	if tradeInfo.CurrentPnL.LessThan(minProfit) {
		return &SellSignal{ShouldSell: false}
	}

	// 从最高点回撤超过设定比例
	if tradeInfo.HighestPrice.GreaterThan(decimal.Zero) {
		drawdown := tradeInfo.HighestPrice.Sub(tradeInfo.CurrentPrice).Div(tradeInfo.HighestPrice)
		trailingThreshold := decimal.NewFromFloat(s.TrailingPercent)

		if drawdown.GreaterThanOrEqual(trailingThreshold) {
			highestPnL := tradeInfo.HighestPrice.Sub(tradeInfo.EntryPrice).Div(tradeInfo.EntryPrice)
			return &SellSignal{
				ShouldSell: true,
				Reason: fmt.Sprintf("trailing stop: %.2f%% (peak: %.2f%%)",
					tradeInfo.CurrentPnL.Mul(decimal.NewFromInt(100)).InexactFloat64(),
					highestPnL.Mul(decimal.NewFromInt(100)).InexactFloat64()),
				Strength: 1.0,
			}
		}
	}

	return &SellSignal{ShouldSell: false}
}

func (s *TrailingSellStrategy) GetName() string {
	return fmt.Sprintf("Trailing(%.1f%% after %.1f%%)", s.TrailingPercent*100, s.MinProfitForTrailing*100)
}

func (s *TrailingSellStrategy) Reset() {}

// TechnicalSellStrategy 技术指标止盈策略
type TechnicalSellStrategy struct {
	MinProfitForTechnical float64
}

func NewTechnicalSellStrategy() *TechnicalSellStrategy {
	return &TechnicalSellStrategy{
		MinProfitForTechnical: 0.10, // 10%
	}
}

func (s *TechnicalSellStrategy) ShouldSell(kline *cex.KlineData, tradeInfo *TradeInfo) *SellSignal {
	// 必须先有基础盈利才考虑技术卖出
	minProfit := decimal.NewFromFloat(s.MinProfitForTechnical)
	if tradeInfo.CurrentPnL.LessThan(minProfit) {
		return &SellSignal{ShouldSell: false}
	}

	// 简化的技术判断：当盈利超过最小阈值时，根据价格动量决定是否卖出
	// 如果盈利超过15%，则认为是技术性卖出时机
	technicalThreshold := decimal.NewFromFloat(0.15) // 15%
	if tradeInfo.CurrentPnL.GreaterThanOrEqual(technicalThreshold) {
		return &SellSignal{
			ShouldSell: true,
			Reason: fmt.Sprintf("technical sell: %.2f%% profit reached technical threshold",
				tradeInfo.CurrentPnL.Mul(decimal.NewFromInt(100)).InexactFloat64()),
			Strength: 1.0,
		}
	}

	return &SellSignal{ShouldSell: false}
}

func (s *TechnicalSellStrategy) GetName() string {
	return "Technical"
}

func (s *TechnicalSellStrategy) Reset() {}

// ComboSellStrategy 组合止盈策略
type ComboSellStrategy struct {
	FixedStrategy    *FixedSellStrategy
	TrailingStrategy *TrailingSellStrategy
	MaxHoldingDays   int
}

func NewComboSellStrategy(config *SellStrategyConfig) *ComboSellStrategy {
	return &ComboSellStrategy{
		FixedStrategy:    NewFixedSellStrategy(config.FixedTakeProfit),
		TrailingStrategy: NewTrailingSellStrategy(config.TrailingPercent, config.MinProfitForTrailing),
		MaxHoldingDays:   config.MaxHoldingDays,
	}
}

func (s *ComboSellStrategy) ShouldSell(kline *cex.KlineData, tradeInfo *TradeInfo) *SellSignal {
	// 1. 检查最大持仓时间
	if s.MaxHoldingDays > 0 && tradeInfo.HoldingDays >= s.MaxHoldingDays {
		return &SellSignal{
			ShouldSell: true,
			Reason:     fmt.Sprintf("max holding time: %d days", s.MaxHoldingDays),
			Strength:   1.0,
		}
	}

	// 2. 检查移动止盈
	trailingSignal := s.TrailingStrategy.ShouldSell(kline, tradeInfo)
	if trailingSignal.ShouldSell {
		return trailingSignal
	}

	// 3. 检查固定止盈（提高阈值，给移动止盈更多空间）
	enhancedThreshold := s.FixedStrategy.TakeProfitPercent * 1.5 // 提高50%
	enhancedFixed := NewFixedSellStrategy(enhancedThreshold)
	fixedSignal := enhancedFixed.ShouldSell(kline, tradeInfo)
	if fixedSignal.ShouldSell {
		fixedSignal.Reason = fmt.Sprintf("enhanced %s", fixedSignal.Reason)
		return fixedSignal
	}

	return &SellSignal{ShouldSell: false}
}

func (s *ComboSellStrategy) GetName() string {
	return fmt.Sprintf("Combo(%s + %s)", s.TrailingStrategy.GetName(), s.FixedStrategy.GetName())
}

func (s *ComboSellStrategy) Reset() {
	s.FixedStrategy.Reset()
	s.TrailingStrategy.Reset()
}

// PartialSellStrategy 分批止盈策略
type PartialSellStrategy struct {
	Levels        []PartialLevel
	ExecutedLevel int // 已执行的级别
}

func NewPartialSellStrategy(levels []PartialLevel) *PartialSellStrategy {
	return &PartialSellStrategy{
		Levels:        levels,
		ExecutedLevel: -1,
	}
}

func (s *PartialSellStrategy) ShouldSell(kline *cex.KlineData, tradeInfo *TradeInfo) *SellSignal {
	// 检查是否达到下一个分批止盈级别
	nextLevel := s.ExecutedLevel + 1
	if nextLevel < len(s.Levels) {
		level := s.Levels[nextLevel]
		threshold := decimal.NewFromFloat(level.ProfitPercent)

		if tradeInfo.CurrentPnL.GreaterThanOrEqual(threshold) {
			s.ExecutedLevel = nextLevel
			return &SellSignal{
				ShouldSell: true,
				Reason: fmt.Sprintf("partial sell level %d: %.2f%% (sell %.0f%%)",
					nextLevel+1,
					tradeInfo.CurrentPnL.Mul(decimal.NewFromInt(100)).InexactFloat64(),
					level.SellPercent*100),
				Strength: level.SellPercent, // 使用卖出比例作为强度
			}
		}
	}

	return &SellSignal{ShouldSell: false}
}

func (s *PartialSellStrategy) GetName() string {
	return fmt.Sprintf("Partial(%d levels)", len(s.Levels))
}

func (s *PartialSellStrategy) Reset() {
	s.ExecutedLevel = -1
}
