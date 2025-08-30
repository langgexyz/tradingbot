package trading

import (
	"github.com/xpwu/go-config/configs"
)

// TradingConfig 交易配置
type TradingConfig struct {
	Timeframe           string  `json:"timeframe"`             // K线周期
	MaxPositions        int     `json:"max_positions"`         // 最大持仓数
	PositionSizePercent float64 `json:"position_size_percent"` // 仓位比例
	MinTradeAmount      float64 `json:"min_trade_amount"`      // 最小交易额
}

// TradingConfigValue 交易配置实例
var TradingConfigValue = TradingConfig{
	Timeframe:           "4h",
	MaxPositions:        1,
	PositionSizePercent: 0.95,
	MinTradeAmount:      10.0,
}

func init() {
	configs.Unmarshal(&TradingConfigValue)
}
