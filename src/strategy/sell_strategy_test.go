package strategy

import (
	"testing"
	"time"

	"tradingbot/src/cex"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create test KlineData
func createTestKline(closePrice float64) *cex.KlineData {
	return &cex.KlineData{
		TradingPair: cex.TradingPair{Base: "BTC", Quote: "USDT"},
		OpenTime:    time.Now(),
		Close:       decimal.NewFromFloat(closePrice),
		CloseTime:   time.Now().Add(time.Hour),
		Open:        decimal.NewFromFloat(closePrice * 0.99),
		High:        decimal.NewFromFloat(closePrice * 1.01),
		Low:         decimal.NewFromFloat(closePrice * 0.98),
		Volume:      decimal.NewFromFloat(100),
		QuoteVolume: decimal.NewFromFloat(closePrice * 100),
	}
}

// Helper function to create test TradeInfo
func createTestTradeInfo(buyPrice, currentPrice, quantity float64) *TradeInfo {
	buyPriceDecimal := decimal.NewFromFloat(buyPrice)
	currentPriceDecimal := decimal.NewFromFloat(currentPrice)

	pnl := currentPriceDecimal.Sub(buyPriceDecimal).Div(buyPriceDecimal)

	return &TradeInfo{
		EntryPrice:   buyPriceDecimal,
		CurrentPrice: currentPriceDecimal,
		EntryTime:    time.Now().Add(-24 * time.Hour), // 1天前买入
		CurrentPnL:   pnl,
		HoldingDays:  1,                   // 1天
		HighestPrice: currentPriceDecimal, // 简化，假设当前价格就是最高价
	}
}

// Test FixedSellStrategy
func TestFixedSellStrategy_ShouldSell(t *testing.T) {
	strategy := NewFixedSellStrategy(0.2) // 20% 止盈

	t.Run("profit above threshold", func(t *testing.T) {
		kline := createTestKline(60000)                   // 当前价格
		tradeInfo := createTestTradeInfo(50000, 60000, 1) // 买入价50000，当前60000，涨幅20%

		signal := strategy.ShouldSell(kline, tradeInfo)
		assert.True(t, signal.ShouldSell)
		assert.Contains(t, signal.Reason, "fixed take profit")
		assert.Equal(t, 1.0, signal.Strength)
	})

	t.Run("profit below threshold", func(t *testing.T) {
		kline := createTestKline(55000)                   // 当前价格
		tradeInfo := createTestTradeInfo(50000, 55000, 1) // 买入价50000，当前55000，涨幅10%

		signal := strategy.ShouldSell(kline, tradeInfo)
		assert.False(t, signal.ShouldSell)
	})

	t.Run("loss position", func(t *testing.T) {
		kline := createTestKline(45000)                   // 当前价格
		tradeInfo := createTestTradeInfo(50000, 45000, 1) // 买入价50000，当前45000，亏损10%

		signal := strategy.ShouldSell(kline, tradeInfo)
		assert.False(t, signal.ShouldSell)
	})

	t.Run("exactly at threshold", func(t *testing.T) {
		kline := createTestKline(60000)                   // 当前价格
		tradeInfo := createTestTradeInfo(50000, 60000, 1) // 买入价50000，当前60000，涨幅恰好20%

		signal := strategy.ShouldSell(kline, tradeInfo)
		assert.True(t, signal.ShouldSell) // GreaterThanOrEqual
	})
}

func TestFixedSellStrategy_GetName(t *testing.T) {
	strategy := NewFixedSellStrategy(0.25)
	assert.Equal(t, "Fixed(25.0%)", strategy.GetName())
}

func TestFixedSellStrategy_Reset(t *testing.T) {
	strategy := NewFixedSellStrategy(0.2)
	// Reset should not panic and should work without errors
	assert.NotPanics(t, func() {
		strategy.Reset()
	})
}

// Test TrailingSellStrategy
func TestTrailingSellStrategy_ShouldSell(t *testing.T) {
	// 5% trailing stop after 15% minimum profit
	strategy := NewTrailingSellStrategy(0.05, 0.15)

	t.Run("profit below minimum threshold", func(t *testing.T) {
		kline := createTestKline(55000) // 10% profit
		tradeInfo := createTestTradeInfo(50000, 55000, 1)

		signal := strategy.ShouldSell(kline, tradeInfo)
		assert.False(t, signal.ShouldSell)
	})

	t.Run("first time above minimum threshold", func(t *testing.T) {
		kline := createTestKline(58000) // 16% profit
		tradeInfo := createTestTradeInfo(50000, 58000, 1)

		signal := strategy.ShouldSell(kline, tradeInfo)
		assert.False(t, signal.ShouldSell) // 第一次达到不会立即触发
	})

	// Note: Testing full trailing behavior would require state management
	// across multiple calls which would be more complex
}

func TestTrailingSellStrategy_GetName(t *testing.T) {
	strategy := NewTrailingSellStrategy(0.05, 0.15)
	expected := "Trailing(5.0% after 15.0%)"
	assert.Equal(t, expected, strategy.GetName())
}

// Test TechnicalSellStrategy
func TestTechnicalSellStrategy_GetName(t *testing.T) {
	strategy := NewTechnicalSellStrategy()
	assert.Equal(t, "Technical", strategy.GetName())
}

// Test ComboSellStrategy
func TestComboSellStrategy_ShouldSell(t *testing.T) {
	config := &SellStrategyConfig{
		Type:                 SellStrategyCombo,
		FixedTakeProfit:      0.3,
		TrailingPercent:      0.05,
		MinProfitForTrailing: 0.15,
		MaxHoldingDays:       30,
	}
	comboStrategy := NewComboSellStrategy(config)

	t.Run("enhanced fixed strategy triggers", func(t *testing.T) {
		kline := createTestKline(72500) // 45% profit (30% * 1.5 threshold)
		tradeInfo := createTestTradeInfo(50000, 72500, 1)

		signal := comboStrategy.ShouldSell(kline, tradeInfo)
		assert.True(t, signal.ShouldSell)
		assert.Contains(t, signal.Reason, "enhanced")
		assert.Contains(t, signal.Reason, "fixed take profit")
	})

	t.Run("max holding time reached", func(t *testing.T) {
		kline := createTestKline(52000) // 4% profit
		tradeInfo := createTestTradeInfo(50000, 52000, 1)
		// Override holding days to 31 days
		tradeInfo.HoldingDays = 31
		tradeInfo.EntryTime = time.Now().Add(-31 * 24 * time.Hour)

		signal := comboStrategy.ShouldSell(kline, tradeInfo)
		assert.True(t, signal.ShouldSell)
		assert.Contains(t, signal.Reason, "max holding time")
	})

	t.Run("no sell conditions met", func(t *testing.T) {
		kline := createTestKline(52000) // 4% profit, not enough for any strategy
		tradeInfo := createTestTradeInfo(50000, 52000, 1)

		signal := comboStrategy.ShouldSell(kline, tradeInfo)
		assert.False(t, signal.ShouldSell)
	})
}

func TestComboSellStrategy_GetName(t *testing.T) {
	config := &SellStrategyConfig{
		Type:                 SellStrategyCombo,
		FixedTakeProfit:      0.3,
		TrailingPercent:      0.05,
		MinProfitForTrailing: 0.15,
		MaxHoldingDays:       30,
	}
	strategy := NewComboSellStrategy(config)

	expected := "Combo(Trailing(5.0% after 15.0%) + Fixed(30.0%))"
	assert.Equal(t, expected, strategy.GetName())
}

// Test PartialSellStrategy
func TestPartialSellStrategy_ShouldSell(t *testing.T) {
	levels := []PartialLevel{
		{ProfitPercent: 0.2, SellPercent: 0.3}, // 20%涨幅时卖出30%
		{ProfitPercent: 0.4, SellPercent: 0.5}, // 40%涨幅时再卖出50%
		{ProfitPercent: 0.8, SellPercent: 1.0}, // 80%涨幅时全部卖出
	}
	strategy := NewPartialSellStrategy(levels)

	t.Run("first level triggered", func(t *testing.T) {
		kline := createTestKline(60000) // 20% profit
		tradeInfo := createTestTradeInfo(50000, 60000, 1)

		signal := strategy.ShouldSell(kline, tradeInfo)
		assert.True(t, signal.ShouldSell)
		assert.Contains(t, signal.Reason, "partial")
		assert.Equal(t, 0.3, signal.Strength) // 卖出30%
	})

	t.Run("profit below first level", func(t *testing.T) {
		kline := createTestKline(58000) // 16% profit
		tradeInfo := createTestTradeInfo(50000, 58000, 1)

		signal := strategy.ShouldSell(kline, tradeInfo)
		assert.False(t, signal.ShouldSell)
	})

	t.Run("second level triggered", func(t *testing.T) {
		// 创建新策略实例，执行level为1（已执行第一级）
		levels := []PartialLevel{
			{ProfitPercent: 0.2, SellPercent: 0.3}, // 20%涨幅时卖出30%
			{ProfitPercent: 0.4, SellPercent: 0.5}, // 40%涨幅时再卖出50%
			{ProfitPercent: 0.8, SellPercent: 1.0}, // 80%涨幅时全部卖出
		}
		strategy := NewPartialSellStrategy(levels)
		strategy.ExecutedLevel = 0 // 已执行第一级（从0开始计数）

		kline := createTestKline(70000) // 40% profit
		tradeInfo := createTestTradeInfo(50000, 70000, 1)

		signal := strategy.ShouldSell(kline, tradeInfo)
		assert.True(t, signal.ShouldSell)
		assert.Equal(t, 0.5, signal.Strength) // 卖出50%
	})
}

func TestPartialSellStrategy_GetName(t *testing.T) {
	levels := []PartialLevel{
		{ProfitPercent: 0.2, SellPercent: 0.3},
		{ProfitPercent: 0.4, SellPercent: 0.5},
	}
	strategy := NewPartialSellStrategy(levels)
	assert.Equal(t, "Partial(2 levels)", strategy.GetName())
}

// Test CreateSellStrategy function
func TestCreateSellStrategy(t *testing.T) {
	t.Run("create fixed strategy", func(t *testing.T) {
		config := &SellStrategyConfig{
			Type:            SellStrategyFixed,
			FixedTakeProfit: 0.25,
		}

		strategy, err := CreateSellStrategy(config)
		require.NoError(t, err)
		assert.NotNil(t, strategy)

		fixedStrategy, ok := strategy.(*FixedSellStrategy)
		require.True(t, ok)
		assert.Equal(t, 0.25, fixedStrategy.TakeProfitPercent)
	})

	t.Run("create trailing strategy", func(t *testing.T) {
		config := &SellStrategyConfig{
			Type:                 SellStrategyTrailing,
			TrailingPercent:      0.05,
			MinProfitForTrailing: 0.15,
		}

		strategy, err := CreateSellStrategy(config)
		require.NoError(t, err)
		assert.NotNil(t, strategy)

		trailingStrategy, ok := strategy.(*TrailingSellStrategy)
		require.True(t, ok)
		assert.Equal(t, 0.05, trailingStrategy.TrailingPercent)
		assert.Equal(t, 0.15, trailingStrategy.MinProfitForTrailing)
	})

	t.Run("create combo strategy", func(t *testing.T) {
		config := &SellStrategyConfig{
			Type:                 SellStrategyCombo,
			FixedTakeProfit:      0.3,
			TrailingPercent:      0.05,
			MinProfitForTrailing: 0.15,
			MaxHoldingDays:       30,
		}

		strategy, err := CreateSellStrategy(config)
		require.NoError(t, err)
		assert.NotNil(t, strategy)

		comboStrategy, ok := strategy.(*ComboSellStrategy)
		require.True(t, ok)
		assert.Equal(t, 30, comboStrategy.MaxHoldingDays)
	})

	t.Run("create partial strategy", func(t *testing.T) {
		levels := []PartialLevel{
			{ProfitPercent: 0.2, SellPercent: 0.3},
			{ProfitPercent: 0.4, SellPercent: 0.7},
		}
		config := &SellStrategyConfig{
			Type:          SellStrategyPartial,
			PartialLevels: levels,
		}

		strategy, err := CreateSellStrategy(config)
		require.NoError(t, err)
		assert.NotNil(t, strategy)

		partialStrategy, ok := strategy.(*PartialSellStrategy)
		require.True(t, ok)
		assert.Len(t, partialStrategy.Levels, 2)
	})

	t.Run("invalid strategy type", func(t *testing.T) {
		config := &SellStrategyConfig{
			Type: "invalid",
		}

		strategy, err := CreateSellStrategy(config)
		assert.Error(t, err)
		assert.Nil(t, strategy)
		assert.Contains(t, err.Error(), "unknown sell strategy type")
	})
}

// Test GetDefaultSellStrategyConfigs
func TestGetDefaultSellStrategyConfigs(t *testing.T) {
	configs := GetDefaultSellStrategyConfigs()

	// 验证预期的策略配置存在
	expectedStrategies := []string{
		"conservative", "moderate", "aggressive",
		"trailing_5", "trailing_10",
		"combo_smart", "partial_pyramid",
	}

	for _, name := range expectedStrategies {
		config, exists := configs[name]
		assert.True(t, exists, "Strategy %s should exist", name)
		assert.NotNil(t, config, "Config for %s should not be nil", name)
		assert.NotEmpty(t, config.Type, "Type for %s should not be empty", name)
	}

	// 测试一些具体配置的值
	conservativeConfig := configs["conservative"]
	assert.Equal(t, SellStrategyFixed, conservativeConfig.Type)
	assert.Equal(t, 0.15, conservativeConfig.FixedTakeProfit)

	aggressiveConfig := configs["aggressive"]
	assert.Equal(t, SellStrategyFixed, aggressiveConfig.Type)
	assert.Equal(t, 0.30, aggressiveConfig.FixedTakeProfit)
}

// Test ParseSellStrategyParams
func TestParseSellStrategyParams(t *testing.T) {
	t.Run("valid params", func(t *testing.T) {
		params, err := ParseSellStrategyParams("take_profit=0.25,trailing_percent=0.05")
		require.NoError(t, err)

		assert.Equal(t, 0.25, params["take_profit"])
		assert.Equal(t, 0.05, params["trailing_percent"])
	})

	t.Run("empty params", func(t *testing.T) {
		params, err := ParseSellStrategyParams("")
		require.NoError(t, err)
		assert.Empty(t, params)
	})

	t.Run("single param", func(t *testing.T) {
		params, err := ParseSellStrategyParams("take_profit=0.20")
		require.NoError(t, err)
		assert.Equal(t, 0.20, params["take_profit"])
	})

	t.Run("invalid format - missing equals", func(t *testing.T) {
		params, err := ParseSellStrategyParams("take_profit")
		assert.Error(t, err)
		assert.Nil(t, params)
		assert.Contains(t, err.Error(), "invalid parameter format")
	})

	t.Run("invalid format - invalid number", func(t *testing.T) {
		params, err := ParseSellStrategyParams("take_profit=abc")
		assert.Error(t, err)
		assert.Nil(t, params)
		assert.Contains(t, err.Error(), "invalid parameter value")
	})

	t.Run("whitespace handling", func(t *testing.T) {
		params, err := ParseSellStrategyParams(" take_profit = 0.25 , trailing_percent = 0.05 ")
		require.NoError(t, err)

		assert.Equal(t, 0.25, params["take_profit"])
		assert.Equal(t, 0.05, params["trailing_percent"])
	})
}

// Test CreateSellStrategyWithParams
func TestCreateSellStrategyWithParams(t *testing.T) {
	t.Run("override fixed strategy params", func(t *testing.T) {
		userParams := map[string]float64{
			"take_profit": 0.35, // Override 20% default to 35%
		}

		strategy, err := CreateSellStrategyWithParams("conservative", userParams)
		require.NoError(t, err)
		assert.NotNil(t, strategy)

		fixedStrategy, ok := strategy.(*FixedSellStrategy)
		require.True(t, ok)
		assert.Equal(t, 0.35, fixedStrategy.TakeProfitPercent) // 应该使用用户参数
	})

	t.Run("override trailing strategy params", func(t *testing.T) {
		userParams := map[string]float64{
			"trailing_percent": 0.08,
			"min_profit":       0.18,
		}

		strategy, err := CreateSellStrategyWithParams("trailing_5", userParams)
		require.NoError(t, err)
		assert.NotNil(t, strategy)

		trailingStrategy, ok := strategy.(*TrailingSellStrategy)
		require.True(t, ok)
		assert.Equal(t, 0.08, trailingStrategy.TrailingPercent)
		assert.Equal(t, 0.18, trailingStrategy.MinProfitForTrailing)
	})

	t.Run("use default when no user params", func(t *testing.T) {
		strategy, err := CreateSellStrategyWithParams("moderate", map[string]float64{})
		require.NoError(t, err)

		fixedStrategy, ok := strategy.(*FixedSellStrategy)
		require.True(t, ok)
		assert.Equal(t, 0.20, fixedStrategy.TakeProfitPercent) // 默认值
	})

	t.Run("unknown strategy", func(t *testing.T) {
		strategy, err := CreateSellStrategyWithParams("unknown", map[string]float64{})
		assert.Error(t, err)
		assert.Nil(t, strategy)
		assert.Contains(t, err.Error(), "unknown sell strategy")
	})
}

// Benchmark tests
func BenchmarkFixedSellStrategy_ShouldSell(b *testing.B) {
	strategy := NewFixedSellStrategy(0.2)
	kline := createTestKline(60000)
	tradeInfo := createTestTradeInfo(50000, 60000, 1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		strategy.ShouldSell(kline, tradeInfo)
	}
}

func BenchmarkTrailingSellStrategy_ShouldSell(b *testing.B) {
	strategy := NewTrailingSellStrategy(0.05, 0.15)
	kline := createTestKline(60000)
	tradeInfo := createTestTradeInfo(50000, 60000, 1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		strategy.ShouldSell(kline, tradeInfo)
	}
}

// Test Reset methods for all strategies
func TestSellStrategies_Reset(t *testing.T) {
	t.Run("FixedSellStrategy Reset", func(t *testing.T) {
		strategy := NewFixedSellStrategy(0.2)

		// Reset should not panic
		assert.NotPanics(t, func() {
			strategy.Reset()
		})
	})

	t.Run("TrailingSellStrategy Reset", func(t *testing.T) {
		strategy := NewTrailingSellStrategy(0.05, 0.15)

		// Test that reset clears internal state
		kline := createTestKline(60000) // 20% profit
		tradeInfo := createTestTradeInfo(50000, 60000, 1)

		// First call to establish peak
		strategy.ShouldSell(kline, tradeInfo)

		// Reset should clear state
		assert.NotPanics(t, func() {
			strategy.Reset()
		})

		// After reset, should work normally
		signal := strategy.ShouldSell(kline, tradeInfo)
		assert.NotNil(t, signal)
	})

	t.Run("TechnicalSellStrategy Reset", func(t *testing.T) {
		strategy := NewTechnicalSellStrategy()

		assert.NotPanics(t, func() {
			strategy.Reset()
		})
	})

	t.Run("ComboSellStrategy Reset", func(t *testing.T) {
		config := &SellStrategyConfig{
			Type:                 SellStrategyCombo,
			FixedTakeProfit:      0.3,
			TrailingPercent:      0.05,
			MinProfitForTrailing: 0.15,
			MaxHoldingDays:       30,
		}
		strategy := NewComboSellStrategy(config)

		assert.NotPanics(t, func() {
			strategy.Reset()
		})
	})

	t.Run("PartialSellStrategy Reset", func(t *testing.T) {
		levels := []PartialLevel{
			{ProfitPercent: 0.2, SellPercent: 0.3},
		}
		strategy := NewPartialSellStrategy(levels)

		// Execute a level first
		kline := createTestKline(60000)
		tradeInfo := createTestTradeInfo(50000, 60000, 1)
		strategy.ShouldSell(kline, tradeInfo)

		// Reset should clear executed level
		assert.NotPanics(t, func() {
			strategy.Reset()
		})

		// Should be able to execute again after reset
		signal := strategy.ShouldSell(kline, tradeInfo)
		assert.True(t, signal.ShouldSell)
	})
}

// Test TechnicalSellStrategy ShouldSell
func TestTechnicalSellStrategy_ShouldSell(t *testing.T) {
	strategy := NewTechnicalSellStrategy()

	t.Run("profitable position with technical signals", func(t *testing.T) {
		kline := createTestKline(60000) // 20% profit
		tradeInfo := createTestTradeInfo(50000, 60000, 1)

		signal := strategy.ShouldSell(kline, tradeInfo)

		// Should sell when profit exceeds minimum threshold
		assert.True(t, signal.ShouldSell)
		assert.Contains(t, signal.Reason, "technical")
		assert.Equal(t, 1.0, signal.Strength)
	})

	t.Run("loss position", func(t *testing.T) {
		kline := createTestKline(40000) // -20% loss
		tradeInfo := createTestTradeInfo(50000, 40000, 1)

		signal := strategy.ShouldSell(kline, tradeInfo)

		// Should not sell when losing
		assert.False(t, signal.ShouldSell)
	})

	t.Run("small profit below threshold", func(t *testing.T) {
		kline := createTestKline(51000) // 2% profit
		tradeInfo := createTestTradeInfo(50000, 51000, 1)

		signal := strategy.ShouldSell(kline, tradeInfo)

		// Should not sell when profit is too small
		assert.False(t, signal.ShouldSell)
	})
}

// Test TrailingSellStrategy more thoroughly for better coverage
func TestTrailingSellStrategy_ComprehensiveCoverage(t *testing.T) {
	strategy := NewTrailingSellStrategy(0.05, 0.15) // 5% trailing after 15% min profit

	t.Run("profit progression with trailing", func(t *testing.T) {
		// Reset strategy for clean test
		strategy.Reset()

		// Start with low profit - should not sell
		kline1 := createTestKline(52500) // 5% profit
		tradeInfo1 := createTestTradeInfo(50000, 52500, 1)
		signal1 := strategy.ShouldSell(kline1, tradeInfo1)
		assert.False(t, signal1.ShouldSell)

		// Reach minimum profit threshold - should not sell yet (first time)
		kline2 := createTestKline(57500) // 15% profit
		tradeInfo2 := createTestTradeInfo(50000, 57500, 1)
		signal2 := strategy.ShouldSell(kline2, tradeInfo2)
		assert.False(t, signal2.ShouldSell) // First time reaching threshold

		// Go higher - should not sell (establishing new peak)
		kline3 := createTestKline(60000) // 20% profit
		tradeInfo3 := createTestTradeInfo(50000, 60000, 1)
		signal3 := strategy.ShouldSell(kline3, tradeInfo3)
		assert.False(t, signal3.ShouldSell)

		// Drop by trailing amount - should sell
		// 重要：设置正确的最高价来模拟回撤
		kline4 := createTestKline(57000) // Dropped 5% from peak of 60000
		tradeInfo4 := &TradeInfo{
			EntryPrice:   decimal.NewFromFloat(50000),
			CurrentPrice: decimal.NewFromFloat(57000),
			HighestPrice: decimal.NewFromFloat(60000), // 明确设置峰值为60000
			CurrentPnL:   decimal.NewFromFloat(0.16),  // 16% profit (高于15%最小阈值)
			EntryTime:    time.Now().Add(-24 * time.Hour),
			HoldingDays:  1,
		}
		signal4 := strategy.ShouldSell(kline4, tradeInfo4)
		assert.True(t, signal4.ShouldSell)
		assert.Contains(t, signal4.Reason, "trailing stop")
	})

	t.Run("peak tracking accuracy", func(t *testing.T) {
		strategy.Reset()

		// Establish initial peak above minimum
		kline1 := createTestKline(58000) // 16% profit
		tradeInfo1 := createTestTradeInfo(50000, 58000, 1)
		strategy.ShouldSell(kline1, tradeInfo1)

		// Go even higher
		kline2 := createTestKline(62000) // 24% profit
		tradeInfo2 := createTestTradeInfo(50000, 62000, 1)
		signal2 := strategy.ShouldSell(kline2, tradeInfo2)
		assert.False(t, signal2.ShouldSell) // New peak, don't sell

		// Small drop, not enough to trigger
		kline3 := createTestKline(60000) // 20% profit, 3.2% drop from peak
		tradeInfo3 := &TradeInfo{
			EntryPrice:   decimal.NewFromFloat(50000),
			CurrentPrice: decimal.NewFromFloat(60000),
			HighestPrice: decimal.NewFromFloat(62000), // 峰值为62000
			CurrentPnL:   decimal.NewFromFloat(0.20),  // 20% profit
			EntryTime:    time.Now().Add(-24 * time.Hour),
			HoldingDays:  1,
		}
		signal3 := strategy.ShouldSell(kline3, tradeInfo3)
		assert.False(t, signal3.ShouldSell) // Drop less than 5%

		// Larger drop, should trigger
		kline4 := createTestKline(58900) // 17.8% profit, 5% drop from 62000 peak
		tradeInfo4 := &TradeInfo{
			EntryPrice:   decimal.NewFromFloat(50000),
			CurrentPrice: decimal.NewFromFloat(58900),
			HighestPrice: decimal.NewFromFloat(62000), // 峰值为62000
			CurrentPnL:   decimal.NewFromFloat(0.178), // 17.8% profit
			EntryTime:    time.Now().Add(-24 * time.Hour),
			HoldingDays:  1,
		}
		signal4 := strategy.ShouldSell(kline4, tradeInfo4)
		assert.True(t, signal4.ShouldSell)
	})
}

// Test Validate method for BollingerBandsParams (skip if not available)
func TestBollingerBandsParams_Validate(t *testing.T) {
	// Note: This test requires BollingerBandsParams to be imported properly
	t.Skip("Skipping BollingerBandsParams test - type not available in this package")
}

// Test GetDefaultBollingerBandsParams
func TestGetDefaultBollingerBandsParams(t *testing.T) {
	// Note: This test requires GetDefaultBollingerBandsParams to be imported properly
	t.Skip("Skipping GetDefaultBollingerBandsParams test - function not available in this package")
}
