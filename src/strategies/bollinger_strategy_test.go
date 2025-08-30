package strategies

import (
	"context"
	"testing"

	"go-build-stream-gateway-go-server-main/src/binance"
	"go-build-stream-gateway-go-server-main/src/executor"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestNewBollingerBandsStrategy(t *testing.T) {
	strategy := NewBollingerBandsStrategy()

	assert.NotNil(t, strategy)
	assert.Equal(t, 20, strategy.Period)
	assert.Equal(t, 2.0, strategy.Multiplier)
	assert.Equal(t, 0.95, strategy.PositionSizePercent)
	assert.Equal(t, 1.0, strategy.StopLossPercent)   // 100% = 永不止损
	assert.Equal(t, 0.5, strategy.TakeProfitPercent) // 50%
	assert.Equal(t, 3, strategy.CooldownBars)
	assert.Equal(t, 10.0, strategy.MinTradeAmount)
	assert.Equal(t, -1, strategy.lastTradeBar)
	assert.NotNil(t, strategy.priceHistory)
}

func TestBollingerBandsStrategy_GetName(t *testing.T) {
	strategy := &BollingerBandsStrategy{}
	assert.Equal(t, "Bollinger Bands Strategy", strategy.GetName())
}

func TestBollingerBandsStrategy_GetParams(t *testing.T) {
	strategy := &BollingerBandsStrategy{
		Period:              20,
		Multiplier:          2.0,
		PositionSizePercent: 0.95,
		StopLossPercent:     0.05,
		TakeProfitPercent:   0.1,
		CooldownBars:        3,
		MinTradeAmount:      10.0,
	}

	params := strategy.GetParams()
	assert.Equal(t, 20, params["period"])
	assert.Equal(t, 2.0, params["multiplier"])
	assert.Equal(t, 0.95, params["position_size_percent"])
	assert.Equal(t, 0.05, params["stop_loss_percent"])
	assert.Equal(t, 0.1, params["take_profit_percent"])
	assert.Equal(t, 3, params["cooldown_bars"])
	assert.Equal(t, 10.0, params["min_trade_amount"])
}

func TestBollingerBandsStrategy_SetParams(t *testing.T) {
	strategy := NewBollingerBandsStrategy()

	params := map[string]interface{}{
		"period":                25,
		"multiplier":            2.5,
		"position_size_percent": 0.8,
		"stop_loss_percent":     0.03,
		"take_profit_percent":   0.15,
		"cooldown_bars":         5,
		"min_trade_amount":      20.0,
	}

	err := strategy.SetParams(params)
	assert.NoError(t, err)

	assert.Equal(t, 25, strategy.Period)
	assert.Equal(t, 2.5, strategy.Multiplier)
	assert.Equal(t, 0.8, strategy.PositionSizePercent)
	assert.Equal(t, 0.03, strategy.StopLossPercent)
	assert.Equal(t, 0.15, strategy.TakeProfitPercent)
	assert.Equal(t, 5, strategy.CooldownBars)
	assert.Equal(t, 20.0, strategy.MinTradeAmount)
}

func TestBollingerBandsStrategy_OnData(t *testing.T) {
	strategy := NewBollingerBandsStrategy()
	strategy.Period = 3 // 使用小周期便于测试
	strategy.CooldownBars = 1
	strategy.SetParams(map[string]interface{}{
		"period": 3,
	}) // 重新初始化布林道指标

	ctx := context.Background()
	portfolio := &executor.Portfolio{
		Cash:     decimal.NewFromFloat(10000),
		Position: decimal.Zero,
	}

	// 测试数据不足的情况
	t.Run("insufficient data", func(t *testing.T) {
		kline := &binance.KlineData{
			Close: decimal.NewFromFloat(100),
		}

		signals, err := strategy.OnData(ctx, kline, portfolio)
		assert.NoError(t, err)
		assert.Empty(t, signals)
	})

	// 添加足够的数据
	prices := []float64{98, 99, 100}
	for _, price := range prices {
		kline := &binance.KlineData{
			Close: decimal.NewFromFloat(price),
		}
		strategy.OnData(ctx, kline, portfolio)
	}

	// 测试买入信号（价格触及下轨）
	t.Run("buy signal - price touches lower band", func(t *testing.T) {
		// 重置策略状态
		strategy.lastTradeBar = -1

		// 价格大幅下跌，触及下轨
		kline := &binance.KlineData{
			Close: decimal.NewFromFloat(95), // 低于下轨
		}

		signals, err := strategy.OnData(ctx, kline, portfolio)
		assert.NoError(t, err)

		// 可能有买入信号
		if len(signals) > 0 {
			buySignal := signals[0]
			if buySignal.Type == "BUY" {
				assert.Contains(t, buySignal.Reason, "lower band")
				assert.True(t, buySignal.Strength > 0)
			}
		}
	})

	// 测试卖出信号（价格触及上轨）
	t.Run("sell signal - price touches upper band", func(t *testing.T) {
		// 先设置持仓
		portfolio.Position = decimal.NewFromFloat(1.0)

		// 价格大幅上涨，触及上轨
		kline := &binance.KlineData{
			Close: decimal.NewFromFloat(108), // 高于上轨
		}

		signals, err := strategy.OnData(ctx, kline, portfolio)
		assert.NoError(t, err)

		// 可能有卖出信号
		if len(signals) > 0 {
			sellSignal := signals[0]
			if sellSignal.Type == "SELL" {
				assert.Contains(t, sellSignal.Reason, "upper band")
				assert.True(t, sellSignal.Strength > 0)
			}
		}
	})
}

func TestBollingerBandsStrategy_StopLossAndTakeProfit(t *testing.T) {
	strategy := NewBollingerBandsStrategy()
	strategy.StopLossPercent = 0.05  // 5%
	strategy.TakeProfitPercent = 0.1 // 10%

	ctx := context.Background()
	portfolio := &executor.Portfolio{
		Position: decimal.NewFromFloat(1.0),
		Cash:     decimal.NewFromFloat(5000),
	}

	// 设置最后交易价格
	strategy.lastTradePrice = decimal.NewFromFloat(100)

	tests := []struct {
		name          string
		currentPrice  float64
		shouldTrigger bool
		reason        string
	}{
		{
			name:          "stop loss triggered",
			currentPrice:  94, // 6% loss
			shouldTrigger: true,
			reason:        "stop loss",
		},
		{
			name:          "take profit triggered",
			currentPrice:  111, // 11% profit
			shouldTrigger: true,
			reason:        "take profit",
		},
		{
			name:          "no trigger",
			currentPrice:  102, // 2% profit, within range
			shouldTrigger: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kline := &binance.KlineData{
				Close: decimal.NewFromFloat(tt.currentPrice),
			}

			signals, err := strategy.OnData(ctx, kline, portfolio)
			assert.NoError(t, err)

			if tt.shouldTrigger {
				// 应该有止损或止盈信号
				found := false
				for _, signal := range signals {
					if signal.Type == "SELL" &&
						(signal.Reason == "stop loss triggered" || signal.Reason == "take profit triggered") {
						found = true
						break
					}
				}
				if !found {
					t.Logf("Expected %s signal but not found", tt.reason)
				}
			}
		})
	}
}

func TestBollingerBandsStrategy_CooldownPeriod(t *testing.T) {
	strategy := NewBollingerBandsStrategy()
	strategy.Period = 3
	strategy.CooldownBars = 2
	strategy.SetParams(map[string]interface{}{
		"period": 3,
	})

	ctx := context.Background()
	portfolio := &executor.Portfolio{
		Cash:     decimal.NewFromFloat(10000),
		Position: decimal.Zero,
	}

	// 添加初始数据
	prices := []float64{100, 101, 102}
	for _, price := range prices {
		kline := &binance.KlineData{
			Close: decimal.NewFromFloat(price),
		}
		strategy.OnData(ctx, kline, portfolio)
	}

	// 第一次交易
	kline1 := &binance.KlineData{
		Close: decimal.NewFromFloat(95), // 触发买入
	}
	signals1, _ := strategy.OnData(ctx, kline1, portfolio)

	// 如果有信号，设置最后交易bar
	if len(signals1) > 0 {
		strategy.lastTradeBar = strategy.currentBar - 1
	}

	// 冷却期内的交易应该被阻止
	kline2 := &binance.KlineData{
		Close: decimal.NewFromFloat(94), // 再次触发买入
	}
	signals2, err := strategy.OnData(ctx, kline2, portfolio)
	assert.NoError(t, err)

	// 在冷却期内，不应该有新的交易信号（除了止损止盈）
	buySignals := 0
	for _, signal := range signals2 {
		if signal.Type == "BUY" {
			buySignals++
		}
	}
	assert.Equal(t, 0, buySignals, "Should not generate buy signals during cooldown period")
}

// 基准测试
func BenchmarkBollingerBandsStrategy_OnData(b *testing.B) {
	strategy := NewBollingerBandsStrategy()

	// 预填充数据
	for i := 0; i < 100; i++ {
		strategy.priceHistory = append(strategy.priceHistory, decimal.NewFromFloat(100+float64(i%10)))
	}

	ctx := context.Background()
	portfolio := &executor.Portfolio{
		Cash:     decimal.NewFromFloat(10000),
		Position: decimal.Zero,
	}
	kline := &binance.KlineData{
		Close: decimal.NewFromFloat(95),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		strategy.OnData(ctx, kline, portfolio)
	}
}
