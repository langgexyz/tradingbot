package strategy

import (
	"fmt"
)

// BollingerBandsParams 布林道策略参数
type BollingerBandsParams struct {
	Period              int     // 计算周期，默认20
	Multiplier          float64 // 标准差倍数，默认2.0
	PositionSizePercent float64 // 仓位比例，默认0.95
	MinTradeAmount      float64 // 最小交易额，默认10
	StopLossPercent     float64 // 止损比例，默认1.0 (100%，即不止损)
	TakeProfitPercent   float64 // 止盈比例，默认0.2 (20%)
	CooldownBars        int     // 冷却期K线数，默认1
}

// GetDefaultBollingerBandsParams 获取默认的布林道策略参数
func GetDefaultBollingerBandsParams() *BollingerBandsParams {
	return &BollingerBandsParams{
		Period:              20,
		Multiplier:          2.0,
		PositionSizePercent: 0.95,
		MinTradeAmount:      10.0,
		StopLossPercent:     1.0, // 100% = 不止损
		TakeProfitPercent:   0.2, // 20%
		CooldownBars:        1,
	}
}

// Validate 验证参数有效性
func (p *BollingerBandsParams) Validate() error {
	if p.Period <= 0 {
		return fmt.Errorf("period must be positive, got %d", p.Period)
	}
	if p.Multiplier <= 0 {
		return fmt.Errorf("multiplier must be positive, got %f", p.Multiplier)
	}
	if p.PositionSizePercent <= 0 || p.PositionSizePercent > 1 {
		return fmt.Errorf("position_size_percent must be between 0 and 1, got %f", p.PositionSizePercent)
	}
	if p.MinTradeAmount < 0 {
		return fmt.Errorf("min_trade_amount must be non-negative, got %f", p.MinTradeAmount)
	}
	if p.CooldownBars < 0 {
		return fmt.Errorf("cooldown_bars must be non-negative, got %d", p.CooldownBars)
	}
	return nil
}
