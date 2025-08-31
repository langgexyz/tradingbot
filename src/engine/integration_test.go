package engine

import (
	"context"
	"testing"
	"time"

	"tradingbot/src/cex"
	"tradingbot/src/executor"
	"tradingbot/src/strategy"
	"tradingbot/src/timeframes"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// 集成测试：模拟完整的交易场景
// ============================================================================

// RealTradeScenarioStrategy 模拟真实交易策略
type RealTradeScenarioStrategy struct {
	signals   []*strategy.Signal
	callIndex int
}

func (s *RealTradeScenarioStrategy) OnData(ctx context.Context, kline *cex.KlineData, portfolio *executor.Portfolio) ([]*strategy.Signal, error) {
	if s.callIndex < len(s.signals) {
		signal := s.signals[s.callIndex]
		s.callIndex++
		if signal != nil {
			return []*strategy.Signal{signal}, nil
		}
	}
	return []*strategy.Signal{}, nil
}

func (s *RealTradeScenarioStrategy) GetName() string                                { return "RealTradeScenario" }
func (s *RealTradeScenarioStrategy) GetParams() strategy.StrategyParams             { return nil }
func (s *RealTradeScenarioStrategy) SetParams(params strategy.StrategyParams) error { return nil }

// TestRealTradingScenario_BullMarket 测试牛市场景
func TestRealTradingScenario_BullMarket(t *testing.T) {
	// 创建牛市K线数据：价格逐步上涨
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	prices := []float64{100, 95, 90, 95, 105, 110, 105, 115, 120, 125} // 先跌后涨
	klines := make([]*cex.KlineData, len(prices))

	for i, price := range prices {
		priceDecimal := decimal.NewFromFloat(price)
		// 为了确保挂单能成功执行，我们设置合理的High/Low范围
		highPrice := priceDecimal.Mul(decimal.NewFromFloat(1.05)) // +5%
		lowPrice := priceDecimal.Mul(decimal.NewFromFloat(0.95))  // -5%

		// 如果这是买入信号后的K线，确保Low足够低以触发买入挂单
		if i == 3 || i == 4 { // 第4、5个K线需要触发买入挂单执行
			lowPrice = decimal.NewFromFloat(88) // 确保能触发89.91的买入挂单
		}

		klines[i] = &cex.KlineData{
			OpenTime:  startTime.Add(time.Duration(i) * 4 * time.Hour),
			CloseTime: startTime.Add(time.Duration(i+1) * 4 * time.Hour),
			Open:      priceDecimal,
			High:      highPrice,
			Low:       lowPrice,
			Close:     priceDecimal,
			Volume:    decimal.NewFromInt(10000),
		}
	}

	// 创建交易策略：在低点买入，高点卖出
	strategy := &RealTradeScenarioStrategy{
		signals: []*strategy.Signal{
			// 第3个K线（90价格）买入 - 注意索引从0开始，所以第3个K线是index 2
			nil, nil, {Type: "BUY", Strength: 0.8, Reason: "价格跌到90，买入机会"},
			// 第8个K线（115价格）部分卖出
			nil, nil, nil, nil, {Type: "SELL", Strength: 0.5, Reason: "价格涨到115，部分获利了结"},
			// 第10个K线（125价格）全部卖出
			nil, {Type: "SELL", Strength: 1.0, Reason: "价格涨到125，全部卖出"},
		},
	}

	// 创建交易系统
	executor := newMockOrderExecutor(decimal.NewFromInt(10000), decimal.Zero) // $10,000初始资金
	dataFeed := NewBacktestDataFeed(klines)
	orderManager := NewBacktestOrderManager(executor)

	engine := NewTradingEngine(
		cex.TradingPair{Base: "BTC", Quote: "USDT"},
		timeframes.Timeframe4h,
		strategy,
		executor,
		&MockCEXClient{},
		dataFeed,
		orderManager,
	)

	engine.SetPositionSizePercent(0.9) // 使用90%资金
	engine.SetMinTradeAmount(100.0)    // 最小交易$100

	ctx := context.Background()
	err := engine.Run(ctx)
	require.NoError(t, err)

	// 验证交易结果
	orders := executor.GetOrders()
	assert.GreaterOrEqual(t, len(orders), 2, "应该至少有买入和卖出订单")

	// 验证盈利
	finalPortfolio, err := executor.GetPortfolio(ctx)
	require.NoError(t, err)

	t.Logf("🎯 牛市交易测试结果:")
	t.Logf("  📊 总订单数: %d", len(orders))
	t.Logf("  💰 最终现金: %s", finalPortfolio.Cash.String())
	t.Logf("  📈 剩余持仓: %s", finalPortfolio.Position.String())

	// 验证基本逻辑：在牛市中应该能盈利
	if finalPortfolio.Position.IsZero() {
		// 全部卖出，现金应该增加
		assert.True(t, finalPortfolio.Cash.GreaterThan(decimal.NewFromInt(10000)),
			"牛市全卖出应该盈利，最终现金: %s", finalPortfolio.Cash.String())
	}
}

// TestRealTradingScenario_BearMarket 测试熊市场景
func TestRealTradingScenario_BearMarket(t *testing.T) {
	// 创建熊市K线数据：价格持续下跌
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	prices := []float64{100, 95, 90, 85, 80, 85, 80, 75, 70, 65} // 持续下跌，偶尔反弹
	klines := make([]*cex.KlineData, len(prices))

	for i, price := range prices {
		priceDecimal := decimal.NewFromFloat(price)
		klines[i] = &cex.KlineData{
			OpenTime:  startTime.Add(time.Duration(i) * 4 * time.Hour),
			CloseTime: startTime.Add(time.Duration(i+1) * 4 * time.Hour),
			Open:      priceDecimal,
			High:      priceDecimal.Mul(decimal.NewFromFloat(1.03)), // +3%
			Low:       priceDecimal.Mul(decimal.NewFromFloat(0.97)), // -3%
			Close:     priceDecimal,
			Volume:    decimal.NewFromInt(8000),
		}
	}

	// 创建保守策略：少量买入，快速止损
	strategy := &RealTradeScenarioStrategy{
		signals: []*strategy.Signal{
			// 第4个K线（85价格）小量买入
			nil, nil, nil, {Type: "BUY", Strength: 0.3, Reason: "小量抄底"},
			// 第7个K线（80价格）止损卖出
			nil, nil, {Type: "SELL", Strength: 1.0, Reason: "继续下跌，止损"},
		},
	}

	// 创建交易系统
	executor := newMockOrderExecutor(decimal.NewFromInt(5000), decimal.Zero) // $5,000初始资金
	dataFeed := NewBacktestDataFeed(klines)
	orderManager := NewBacktestOrderManager(executor)

	engine := NewTradingEngine(
		cex.TradingPair{Base: "ETH", Quote: "USDT"},
		timeframes.Timeframe4h,
		strategy,
		executor,
		&MockCEXClient{},
		dataFeed,
		orderManager,
	)

	engine.SetPositionSizePercent(0.5) // 只使用50%资金，保守策略
	engine.SetMinTradeAmount(50.0)     // 最小交易$50

	ctx := context.Background()
	err := engine.Run(ctx)
	require.NoError(t, err)

	// 验证交易结果
	orders := executor.GetOrders()
	finalPortfolio, err := executor.GetPortfolio(ctx)
	require.NoError(t, err)

	t.Logf("🐻 熊市交易测试结果:")
	t.Logf("  📊 总订单数: %d", len(orders))
	t.Logf("  💰 最终现金: %s", finalPortfolio.Cash.String())
	t.Logf("  📈 剩余持仓: %s", finalPortfolio.Position.String())

	// 熊市中保守策略应该限制损失
	totalValue := finalPortfolio.Cash
	if !finalPortfolio.Position.IsZero() {
		// 如果有持仓，按最后价格估值
		lastPrice := decimal.NewFromFloat(prices[len(prices)-1])
		totalValue = totalValue.Add(finalPortfolio.Position.Mul(lastPrice))
	}

	lossPercent := decimal.NewFromInt(5000).Sub(totalValue).Div(decimal.NewFromInt(5000)).Mul(decimal.NewFromInt(100))
	t.Logf("  📉 损失百分比: %s%%", lossPercent.String())

	// 在熊市中，损失应该被控制在合理范围内
	assert.True(t, lossPercent.LessThan(decimal.NewFromFloat(30)),
		"熊市中损失应控制在30%以内，实际损失: %s%%", lossPercent.String())
}

// TestRealTradingScenario_SidewaysMarket 测试震荡市场景
func TestRealTradingScenario_SidewaysMarket(t *testing.T) {
	// 创建震荡市K线数据：在95-105之间震荡，额外添加K线让最后的卖单有机会执行
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	prices := []float64{100, 95, 100, 105, 100, 95, 100, 105, 100, 95, 100, 105, 106} // 规律震荡，最后一个设为106以触发卖单执行
	klines := make([]*cex.KlineData, len(prices))

	for i, price := range prices {
		priceDecimal := decimal.NewFromFloat(price)
		klines[i] = &cex.KlineData{
			OpenTime:  startTime.Add(time.Duration(i) * 4 * time.Hour),
			CloseTime: startTime.Add(time.Duration(i+1) * 4 * time.Hour),
			Open:      priceDecimal,
			High:      priceDecimal.Mul(decimal.NewFromFloat(1.02)), // +2%
			Low:       priceDecimal.Mul(decimal.NewFromFloat(0.98)), // -2%
			Close:     priceDecimal,
			Volume:    decimal.NewFromInt(12000),
		}
	}

	// 创建震荡策略：低买高卖
	strategy := &RealTradeScenarioStrategy{
		signals: []*strategy.Signal{
			// 在95价位买入，105价位卖出
			nil, {Type: "BUY", Strength: 0.7, Reason: "95低位买入"}, // 第2个K线
			nil, {Type: "SELL", Strength: 1.0, Reason: "105高位卖出"}, // 第4个K线
			nil, {Type: "BUY", Strength: 0.7, Reason: "95低位买入"}, // 第6个K线
			nil, {Type: "SELL", Strength: 1.0, Reason: "105高位卖出"}, // 第8个K线
			nil, {Type: "BUY", Strength: 0.7, Reason: "95低位买入"}, // 第10个K线
			nil, {Type: "SELL", Strength: 1.0, Reason: "105高位卖出"}, // 第12个K线
			nil, // 第13个K线，让最后的卖单有机会执行
		},
	}

	// 创建交易系统
	executor := newMockOrderExecutor(decimal.NewFromInt(8000), decimal.Zero) // $8,000初始资金
	dataFeed := NewBacktestDataFeed(klines)
	orderManager := NewBacktestOrderManager(executor)

	engine := NewTradingEngine(
		cex.TradingPair{Base: "ADA", Quote: "USDT"},
		timeframes.Timeframe4h,
		strategy,
		executor,
		&MockCEXClient{},
		dataFeed,
		orderManager,
	)

	engine.SetPositionSizePercent(0.8) // 使用80%资金
	engine.SetMinTradeAmount(100.0)    // 最小交易$100

	ctx := context.Background()
	err := engine.Run(ctx)
	require.NoError(t, err)

	// 验证交易结果
	orders := executor.GetOrders()
	finalPortfolio, err := executor.GetPortfolio(ctx)
	require.NoError(t, err)

	// 计算交易次数
	buyCount := 0
	sellCount := 0
	for _, order := range orders {
		if order.Side == "BUY" {
			buyCount++
		} else if order.Side == "SELL" {
			sellCount++
		}
	}

	t.Logf("📊 震荡市交易测试结果:")
	t.Logf("  📈 买入次数: %d", buyCount)
	t.Logf("  📉 卖出次数: %d", sellCount)
	t.Logf("  💰 最终现金: %s", finalPortfolio.Cash.String())
	t.Logf("  📊 剩余持仓: %s", finalPortfolio.Position.String())

	// 震荡市中应该有多次交易
	assert.GreaterOrEqual(t, buyCount, 2, "震荡市应该有多次买入")
	assert.GreaterOrEqual(t, sellCount, 2, "震荡市应该有多次卖出")

	// 震荡市中频繁交易应该能获得一些收益
	if finalPortfolio.Position.IsZero() {
		profitPercent := finalPortfolio.Cash.Sub(decimal.NewFromInt(8000)).Div(decimal.NewFromInt(8000)).Mul(decimal.NewFromInt(100))
		t.Logf("  💎 盈利百分比: %s%%", profitPercent.String())

		// 在理想的震荡市中，应该能获得一些收益
		assert.True(t, profitPercent.GreaterThan(decimal.NewFromFloat(-5)),
			"震荡市中不应该亏损太多，实际收益: %s%%", profitPercent.String())
	}
}

// TestRealTradingScenario_HighVolatility 测试高波动率场景
func TestRealTradingScenario_HighVolatility(t *testing.T) {
	// 创建高波动K线数据：剧烈波动
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	prices := []float64{100, 120, 80, 150, 60, 140, 70, 130, 90, 110} // 剧烈波动
	klines := make([]*cex.KlineData, len(prices))

	for i, price := range prices {
		priceDecimal := decimal.NewFromFloat(price)
		highVariation := decimal.NewFromFloat(price * 0.15) // ±15%的波动
		lowVariation := decimal.NewFromFloat(price * 0.15)

		klines[i] = &cex.KlineData{
			OpenTime:  startTime.Add(time.Duration(i) * 1 * time.Hour), // 1小时间隔，更高频
			CloseTime: startTime.Add(time.Duration(i+1) * 1 * time.Hour),
			Open:      priceDecimal,
			High:      priceDecimal.Add(highVariation), // +15%
			Low:       priceDecimal.Sub(lowVariation),  // -15%
			Close:     priceDecimal,
			Volume:    decimal.NewFromInt(20000), // 高成交量
		}
	}

	// 创建谨慎策略：在极端波动中保持谨慎
	strategy := &RealTradeScenarioStrategy{
		signals: []*strategy.Signal{
			// 只在相对稳定的时机交易
			nil, nil, {Type: "BUY", Strength: 0.2, Reason: "跌至80，小量抄底"}, // 第3个K线
			nil, nil, nil, nil, nil, {Type: "SELL", Strength: 1.0, Reason: "波动太大，清仓观望"}, // 第9个K线
		},
	}

	// 创建交易系统
	executor := newMockOrderExecutor(decimal.NewFromInt(3000), decimal.Zero) // $3,000初始资金
	dataFeed := NewBacktestDataFeed(klines)
	orderManager := NewBacktestOrderManager(executor)

	engine := NewTradingEngine(
		cex.TradingPair{Base: "DOGE", Quote: "USDT"},
		timeframes.Timeframe1h, // 1小时时间框架
		strategy,
		executor,
		&MockCEXClient{},
		dataFeed,
		orderManager,
	)

	engine.SetPositionSizePercent(0.3) // 只使用30%资金，高度谨慎
	engine.SetMinTradeAmount(50.0)     // 最小交易$50

	ctx := context.Background()
	err := engine.Run(ctx)
	require.NoError(t, err)

	// 验证交易结果
	orders := executor.GetOrders()
	finalPortfolio, err := executor.GetPortfolio(ctx)
	require.NoError(t, err)

	t.Logf("⚡ 高波动交易测试结果:")
	t.Logf("  📊 总订单数: %d", len(orders))
	t.Logf("  💰 最终现金: %s", finalPortfolio.Cash.String())
	t.Logf("  📈 剩余持仓: %s", finalPortfolio.Position.String())

	// 在高波动市场中，谨慎策略应该限制风险
	totalValue := finalPortfolio.Cash
	if !finalPortfolio.Position.IsZero() {
		// 如果有持仓，按最后价格估值
		lastPrice := decimal.NewFromFloat(prices[len(prices)-1])
		totalValue = totalValue.Add(finalPortfolio.Position.Mul(lastPrice))
	}

	// 资金应该得到保护
	assert.True(t, totalValue.GreaterThan(decimal.NewFromFloat(2000)),
		"高波动中应该保护资金，总价值: %s", totalValue.String())
}

// ============================================================================
// 压力测试：大量数据和高频交易
// ============================================================================

func TestStressTest_LargeDataSet(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过压力测试")
	}

	// 创建大量K线数据（1年的4小时数据 ≈ 2190条）
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	const klineCount = 2190
	klines := make([]*cex.KlineData, klineCount)

	basePrice := 50000.0
	for i := 0; i < klineCount; i++ {
		// 模拟随机价格波动
		variation := float64(i%100-50) * 0.01 // ±50% * 1% = ±0.5%
		price := basePrice * (1 + variation)
		priceDecimal := decimal.NewFromFloat(price)

		klines[i] = &cex.KlineData{
			OpenTime:  startTime.Add(time.Duration(i) * 4 * time.Hour),
			CloseTime: startTime.Add(time.Duration(i+1) * 4 * time.Hour),
			Open:      priceDecimal,
			High:      priceDecimal.Mul(decimal.NewFromFloat(1.02)),
			Low:       priceDecimal.Mul(decimal.NewFromFloat(0.98)),
			Close:     priceDecimal,
			Volume:    decimal.NewFromInt(1000),
		}
	}

	// 创建简单策略
	strategy := &RealTradeScenarioStrategy{
		signals: []*strategy.Signal{
			{Type: "BUY", Strength: 0.5, Reason: "定期买入"}, // 第1个信号
		},
	}

	// 创建交易系统
	executor := newMockOrderExecutor(decimal.NewFromInt(100000), decimal.Zero)
	dataFeed := NewBacktestDataFeed(klines)
	orderManager := NewBacktestOrderManager(executor)

	engine := NewTradingEngine(
		cex.TradingPair{Base: "BTC", Quote: "USDT"},
		timeframes.Timeframe4h,
		strategy,
		executor,
		&MockCEXClient{},
		dataFeed,
		orderManager,
	)

	start := time.Now()
	ctx := context.Background()
	err := engine.Run(ctx)
	elapsed := time.Since(start)

	require.NoError(t, err)

	t.Logf("🔥 压力测试结果:")
	t.Logf("  📊 处理K线数: %d", klineCount)
	t.Logf("  ⏱️  处理时间: %v", elapsed)
	t.Logf("  🚀 处理速度: %.0f K线/秒", float64(klineCount)/elapsed.Seconds())

	// 性能要求：处理速度应该足够快
	assert.Less(t, elapsed, 5*time.Second, "处理2190个K线应该在5秒内完成")
	assert.Equal(t, klineCount, len(engine.GetKlines()), "所有K线都应该被处理")
}

// ============================================================================
// 边界情况测试
// ============================================================================

func TestEdgeCase_ZeroInitialCapital(t *testing.T) {
	// 零资金启动
	executor := newMockOrderExecutor(decimal.Zero, decimal.Zero)
	dataFeed := NewBacktestDataFeed(CreateTestKlines(5, time.Now(), 4*time.Hour))
	orderManager := NewBacktestOrderManager(executor)

	strategy := &RealTradeScenarioStrategy{
		signals: []*strategy.Signal{
			{Type: "BUY", Strength: 1.0, Reason: "尝试买入"},
		},
	}

	engine := NewTradingEngine(
		cex.TradingPair{Base: "BTC", Quote: "USDT"},
		timeframes.Timeframe4h,
		strategy,
		executor,
		&MockCEXClient{},
		dataFeed,
		orderManager,
	)

	ctx := context.Background()
	err := engine.Run(ctx)

	// 应该正常完成，不会因为资金不足而崩溃
	assert.NoError(t, err)

	orders := executor.GetOrders()
	assert.Equal(t, 0, len(orders), "零资金不应该产生任何订单")
}

func TestEdgeCase_ExtremeSignalStrength(t *testing.T) {
	// 测试极端的信号强度
	executor := newMockOrderExecutor(decimal.NewFromInt(10000), decimal.NewFromInt(10))
	dataFeed := NewBacktestDataFeed(CreateTestKlines(5, time.Now(), 4*time.Hour))
	orderManager := NewBacktestOrderManager(executor)

	strategy := &RealTradeScenarioStrategy{
		signals: []*strategy.Signal{
			{Type: "SELL", Strength: -1.0, Reason: "负强度信号"},   // 负强度
			{Type: "SELL", Strength: 2.0, Reason: "超过100%强度"}, // 超过100%
			{Type: "SELL", Strength: 0.0001, Reason: "极小强度"},  // 极小值
		},
	}

	engine := NewTradingEngine(
		cex.TradingPair{Base: "ETH", Quote: "USDT"},
		timeframes.Timeframe4h,
		strategy,
		executor,
		&MockCEXClient{},
		dataFeed,
		orderManager,
	)

	ctx := context.Background()
	err := engine.Run(ctx)

	// 应该能处理极端值而不崩溃
	assert.NoError(t, err)

	t.Logf("处理极端信号强度测试完成")
}

// ============================================================================
// 并发安全测试
// ============================================================================

func TestConcurrency_MultipleEngines(t *testing.T) {
	const numEngines = 5
	results := make(chan error, numEngines)

	for i := 0; i < numEngines; i++ {
		go func(id int) {
			executor := newMockOrderExecutor(decimal.NewFromInt(1000), decimal.Zero)
			dataFeed := NewBacktestDataFeed(CreateTestKlines(10, time.Now(), 4*time.Hour))
			orderManager := NewBacktestOrderManager(executor)

			strategy := &RealTradeScenarioStrategy{
				signals: []*strategy.Signal{
					{Type: "BUY", Strength: 0.5, Reason: "并发测试买入"},
				},
			}

			engine := NewTradingEngine(
				cex.TradingPair{Base: "BTC", Quote: "USDT"},
				timeframes.Timeframe4h,
				strategy,
				executor,
				&MockCEXClient{},
				dataFeed,
				orderManager,
			)

			ctx := context.Background()
			err := engine.Run(ctx)
			results <- err
		}(i)
	}

	// 等待所有goroutine完成
	for i := 0; i < numEngines; i++ {
		err := <-results
		assert.NoError(t, err, "并发引擎 %d 应该成功完成", i)
	}

	t.Logf("✅ 并发安全测试：%d个引擎同时运行成功", numEngines)
}
