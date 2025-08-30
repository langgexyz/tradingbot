package strategy

import (
	"fmt"
	"time"

	"tradingbot/src/cex"

	"github.com/shopspring/decimal"
)

// SellSignal 卖出信号
type SellSignal struct {
	ShouldSell bool
	Reason     string
	Strength   float64
}

// SellStrategy 卖出策略接口
type SellStrategy interface {
	// ShouldSell 判断是否应该卖出
	ShouldSell(kline *cex.KlineData, tradeInfo *TradeInfo) *SellSignal

	// GetName 获取策略名称
	GetName() string

	// Reset 重置策略状态
	Reset()
}

// TradeInfo 交易信息
type TradeInfo struct {
	EntryPrice   decimal.Decimal // 开仓价格
	EntryTime    time.Time       // 开仓时间
	HighestPrice decimal.Decimal // 持仓期间最高价格
	CurrentPrice decimal.Decimal // 当前价格
	CurrentPnL   decimal.Decimal // 当前盈亏百分比
	HoldingDays  int             // 持仓天数
}

// SellStrategyType 卖出策略类型
type SellStrategyType string

const (
	SellStrategyFixed     SellStrategyType = "fixed"     // 固定止盈
	SellStrategyTrailing  SellStrategyType = "trailing"  // 移动止盈
	SellStrategyTechnical SellStrategyType = "technical" // 技术指标
	SellStrategyCombo     SellStrategyType = "combo"     // 组合策略
	SellStrategyPartial   SellStrategyType = "partial"   // 分批止盈
)

// SellStrategyConfig 卖出策略配置
type SellStrategyConfig struct {
	Type                 SellStrategyType `json:"type"`
	FixedTakeProfit      float64          `json:"fixed_take_profit"`       // 固定止盈比例
	TrailingPercent      float64          `json:"trailing_percent"`        // 移动止盈回撤比例
	MinProfitForTrailing float64          `json:"min_profit_for_trailing"` // 启用移动止盈的最小盈利
	MaxHoldingDays       int              `json:"max_holding_days"`        // 最大持仓天数
	PartialLevels        []PartialLevel   `json:"partial_levels"`          // 分批止盈配置
}

// PartialLevel 分批止盈配置
type PartialLevel struct {
	ProfitPercent float64 `json:"profit_percent"` // 盈利百分比
	SellPercent   float64 `json:"sell_percent"`   // 卖出仓位百分比
}

// CreateSellStrategy 创建卖出策略
func CreateSellStrategy(config *SellStrategyConfig) (SellStrategy, error) {
	switch config.Type {
	case SellStrategyFixed:
		return NewFixedSellStrategy(config.FixedTakeProfit), nil
	case SellStrategyTrailing:
		return NewTrailingSellStrategy(config.TrailingPercent, config.MinProfitForTrailing), nil
	case SellStrategyTechnical:
		return NewTechnicalSellStrategy(), nil
	case SellStrategyCombo:
		return NewComboSellStrategy(config), nil
	case SellStrategyPartial:
		return NewPartialSellStrategy(config.PartialLevels), nil
	default:
		return nil, fmt.Errorf("unknown sell strategy type: %s", config.Type)
	}
}

// GetDefaultSellStrategyConfigs 获取预设的卖出策略配置
func GetDefaultSellStrategyConfigs() map[string]*SellStrategyConfig {
	return map[string]*SellStrategyConfig{
		"conservative": {
			Type:            SellStrategyFixed,
			FixedTakeProfit: 0.15, // 15%
		},
		"moderate": {
			Type:            SellStrategyFixed,
			FixedTakeProfit: 0.20, // 20%
		},
		"aggressive": {
			Type:            SellStrategyFixed,
			FixedTakeProfit: 0.30, // 30%
		},
		"trailing_5": {
			Type:                 SellStrategyTrailing,
			TrailingPercent:      0.05, // 5%回撤
			MinProfitForTrailing: 0.15, // 15%后启用
		},
		"trailing_10": {
			Type:                 SellStrategyTrailing,
			TrailingPercent:      0.10, // 10%回撤
			MinProfitForTrailing: 0.20, // 20%后启用
		},
		"combo_smart": {
			Type:                 SellStrategyCombo,
			FixedTakeProfit:      0.25, // 25%兜底
			TrailingPercent:      0.08, // 8%回撤
			MinProfitForTrailing: 0.18, // 18%后启用移动止盈
			MaxHoldingDays:       180,  // 最大持仓半年
		},
		"partial_pyramid": {
			Type: SellStrategyPartial,
			PartialLevels: []PartialLevel{
				{ProfitPercent: 0.20, SellPercent: 0.30}, // 20%盈利卖30%
				{ProfitPercent: 0.40, SellPercent: 0.40}, // 40%盈利再卖40%
				{ProfitPercent: 0.60, SellPercent: 1.00}, // 60%盈利全卖
			},
		},
	}
}
