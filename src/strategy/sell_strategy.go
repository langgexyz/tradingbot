package strategy

import (
	"fmt"
	"strconv"
	"strings"
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

// PartialLevel 分批止盈配置
type PartialLevel struct {
	ProfitPercent float64 `json:"profit_percent"` // 盈利百分比
	SellPercent   float64 `json:"sell_percent"`   // 卖出仓位百分比
}

// SellStrategyConfig 卖出策略配置
type SellStrategyConfig struct {
	Type                 SellStrategyType `json:"type"`
	FixedTakeProfit      float64          `json:"fixed_take_profit"`       // 固定止盈比例
	TrailingPercent      float64          `json:"trailing_percent"`        // 移动止盈回撤比例
	MinProfitForTrailing float64          `json:"min_profit_for_trailing"` // 启用移动止盈的最小盈利
	MaxHoldingDays       int              `json:"max_holding_days"`        // 最大持仓天数
	PartialLevels        []PartialLevel   `json:"partial_levels"`          // 分批止盈配置
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

// ParseSellStrategyParams 解析卖出策略参数字符串
func ParseSellStrategyParams(paramsStr string) (map[string]float64, error) {
	params := make(map[string]float64)

	if paramsStr == "" {
		return params, nil
	}

	// 解析格式: "key1=value1,key2=value2"
	pairs := strings.Split(paramsStr, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		parts := strings.Split(pair, "=")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid parameter format: %s (expected key=value)", pair)
		}

		key := strings.TrimSpace(parts[0])
		valueStr := strings.TrimSpace(parts[1])

		value, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid parameter value for %s: %s", key, valueStr)
		}

		params[key] = value
	}

	return params, nil
}

// CreateSellStrategyWithParams 使用用户参数创建卖出策略
func CreateSellStrategyWithParams(strategyName string, userParams map[string]float64) (SellStrategy, error) {
	// 首先尝试预设配置
	defaultConfigs := GetDefaultSellStrategyConfigs()
	if config, exists := defaultConfigs[strategyName]; exists {
		// 复制配置以避免修改默认值
		configCopy := *config

		// 根据用户参数覆盖默认值
		if takeProfit, ok := userParams["take_profit"]; ok {
			configCopy.FixedTakeProfit = takeProfit
		}
		if trailingPercent, ok := userParams["trailing_percent"]; ok {
			configCopy.TrailingPercent = trailingPercent
		}
		if minProfit, ok := userParams["min_profit"]; ok {
			configCopy.MinProfitForTrailing = minProfit
		}

		return CreateSellStrategy(&configCopy)
	}

	// 如果不是预设配置，尝试直接策略类型
	config := &SellStrategyConfig{}

	switch strategyName {
	case "fixed":
		config.Type = SellStrategyFixed
		config.FixedTakeProfit = 0.20 // 默认20%

		// 应用用户参数
		if takeProfit, ok := userParams["take_profit"]; ok {
			config.FixedTakeProfit = takeProfit
		}

	case "trailing":
		config.Type = SellStrategyTrailing
		config.TrailingPercent = 0.05      // 默认5%回撤
		config.MinProfitForTrailing = 0.15 // 默认15%后启用

		// 应用用户参数
		if trailingPercent, ok := userParams["trailing_percent"]; ok {
			config.TrailingPercent = trailingPercent
		}
		if minProfit, ok := userParams["min_profit"]; ok {
			config.MinProfitForTrailing = minProfit
		}

	case "combo":
		config.Type = SellStrategyCombo
		config.FixedTakeProfit = 0.25      // 默认25%兜底
		config.TrailingPercent = 0.08      // 默认8%回撤
		config.MinProfitForTrailing = 0.18 // 默认18%后启用
		config.MaxHoldingDays = 180        // 默认180天

		// 应用用户参数
		if takeProfit, ok := userParams["take_profit"]; ok {
			config.FixedTakeProfit = takeProfit
		}
		if trailingPercent, ok := userParams["trailing_percent"]; ok {
			config.TrailingPercent = trailingPercent
		}
		if minProfit, ok := userParams["min_profit"]; ok {
			config.MinProfitForTrailing = minProfit
		}
		if maxDays, ok := userParams["max_holding_days"]; ok {
			config.MaxHoldingDays = int(maxDays)
		}

	case "partial":
		config.Type = SellStrategyPartial
		// 使用默认分批配置
		config.PartialLevels = []PartialLevel{
			{ProfitPercent: 0.20, SellPercent: 0.30}, // 20%盈利卖30%
			{ProfitPercent: 0.40, SellPercent: 0.40}, // 40%盈利再卖40%
			{ProfitPercent: 0.60, SellPercent: 1.00}, // 60%盈利全卖
		}
		// 注意：partial策略的参数比较复杂，暂时使用默认配置

	default:
		return nil, fmt.Errorf("unknown sell strategy: %s", strategyName)
	}

	return CreateSellStrategy(config)
}
