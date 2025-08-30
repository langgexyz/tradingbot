package executor

import (
	"context"
	"fmt"
	"testing"
	"time"

	"tradingbot/src/cex"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function for tests
func mustParseDecimal(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		panic(err)
	}
	return d
}

func TestNewBacktestExecutor(t *testing.T) {
	pair := cex.TradingPair{Base: "BTC", Quote: "USDT"}
	initialCapital := decimal.NewFromFloat(10000)

	executor := NewBacktestExecutor(pair, initialCapital)

	assert.NotNil(t, executor)
	assert.Equal(t, "BacktestExecutor", executor.GetName())
	assert.Equal(t, pair, executor.tradingPair)
	assert.Equal(t, initialCapital, executor.initialCapital)
	assert.Equal(t, initialCapital, executor.cash)
	assert.True(t, executor.position.IsZero())
	assert.Equal(t, decimal.NewFromFloat(0.001), executor.commission) // 默认0.1%
	assert.Empty(t, executor.orders)
}

func TestBacktestExecutor_SetCommission(t *testing.T) {
	pair := cex.TradingPair{Base: "BTC", Quote: "USDT"}
	executor := NewBacktestExecutor(pair, decimal.NewFromFloat(10000))

	executor.SetCommission(0.002) // 0.2%
	assert.Equal(t, decimal.NewFromFloat(0.002), executor.commission)
}

func TestBacktestExecutor_Buy_Success(t *testing.T) {
	pair := cex.TradingPair{Base: "BTC", Quote: "USDT"}
	initialCapital := decimal.NewFromFloat(10000)
	executor := NewBacktestExecutor(pair, initialCapital)
	ctx := context.Background()

	// 买入订单
	buyOrder := &BuyOrder{
		ID:          "test_buy_1",
		TradingPair: pair,
		Quantity:    decimal.NewFromFloat(0.1),   // 0.1 BTC
		Price:       decimal.NewFromFloat(50000), // 50000 USDT per BTC
		Timestamp:   time.Now(),
		Reason:      "test buy",
	}

	// 预期成本: 0.1 * 50000 = 5000 USDT + 手续费 5000 * 0.001 = 5 USDT = 5005 USDT
	expectedCost := decimal.NewFromFloat(5005)
	expectedRemainingCash := initialCapital.Sub(expectedCost)

	result, err := executor.Buy(ctx, buyOrder)

	// 验证返回结果
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, OrderSideBuy, result.Side)
	assert.Equal(t, buyOrder.Quantity, result.Quantity)
	assert.Equal(t, buyOrder.Price, result.Price)
	assert.Equal(t, decimal.NewFromFloat(5), result.Commission)

	// 验证执行器状态
	assert.Equal(t, expectedRemainingCash, executor.cash)
	assert.Equal(t, buyOrder.Quantity, executor.position)
	assert.Equal(t, 1, executor.totalTrades)
	assert.Len(t, executor.orders, 1)
}

func TestBacktestExecutor_Buy_InsufficientCash(t *testing.T) {
	pair := cex.TradingPair{Base: "BTC", Quote: "USDT"}
	executor := NewBacktestExecutor(pair, decimal.NewFromFloat(1000)) // 只有 1000 USDT
	ctx := context.Background()

	// 尝试买入过多
	buyOrder := &BuyOrder{
		ID:          "test_buy_fail",
		TradingPair: pair,
		Quantity:    decimal.NewFromFloat(1),     // 1 BTC
		Price:       decimal.NewFromFloat(50000), // 50000 USDT per BTC
		Timestamp:   time.Now(),
		Reason:      "test insufficient cash",
	}

	result, err := executor.Buy(ctx, buyOrder)

	// 应该返回错误
	assert.Error(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Equal(t, "insufficient cash", result.Error)

	// 执行器状态不应改变
	assert.Equal(t, decimal.NewFromFloat(1000), executor.cash)
	assert.True(t, executor.position.IsZero())
	assert.Equal(t, 0, executor.totalTrades)
}

func TestBacktestExecutor_Sell_Success(t *testing.T) {
	pair := cex.TradingPair{Base: "BTC", Quote: "USDT"}
	initialCapital := decimal.NewFromFloat(12000) // 增加初始资金
	executor := NewBacktestExecutor(pair, initialCapital)
	ctx := context.Background()

	// 首先买入建立持仓
	buyOrder := &BuyOrder{
		ID:          "test_buy",
		TradingPair: pair,
		Quantity:    decimal.NewFromFloat(0.2),   // 0.2 BTC
		Price:       decimal.NewFromFloat(50000), // 50000 USDT per BTC
		Timestamp:   time.Now(),
		Reason:      "setup position",
	}
	_, err := executor.Buy(ctx, buyOrder)
	require.NoError(t, err)

	// 现在卖出部分持仓
	sellOrder := &SellOrder{
		ID:          "test_sell",
		TradingPair: pair,
		Quantity:    decimal.NewFromFloat(0.1),   // 卖出 0.1 BTC
		Price:       decimal.NewFromFloat(55000), // 55000 USDT per BTC (盈利)
		Timestamp:   time.Now().Add(time.Hour),
		Reason:      "take profit",
	}

	// 记录卖出前的现金
	cashBeforeSell := executor.cash

	result, err := executor.Sell(ctx, sellOrder)

	// 验证返回结果
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, OrderSideSell, result.Side)
	assert.Equal(t, sellOrder.Quantity, result.Quantity)
	assert.Equal(t, sellOrder.Price, result.Price)

	// 预期收入: 0.1 * 55000 = 5500 USDT - 手续费 5500 * 0.001 = 5.5 USDT = 5494.5 USDT
	expectedRevenue := decimal.NewFromFloat(5494.5)
	expectedCash := cashBeforeSell.Add(expectedRevenue)

	// 验证执行器状态
	assert.Equal(t, expectedCash, executor.cash)
	assert.Equal(t, decimal.NewFromFloat(0.1), executor.position) // 剩余 0.1 BTC
	assert.Equal(t, 2, executor.totalTrades)                      // 买入+卖出
	assert.Len(t, executor.orders, 2)
}

func TestBacktestExecutor_Sell_InsufficientPosition(t *testing.T) {
	pair := cex.TradingPair{Base: "BTC", Quote: "USDT"}
	executor := NewBacktestExecutor(pair, decimal.NewFromFloat(10000))
	ctx := context.Background()

	// 尝试卖出但没有持仓
	sellOrder := &SellOrder{
		ID:          "test_sell_fail",
		TradingPair: pair,
		Quantity:    decimal.NewFromFloat(0.1),
		Price:       decimal.NewFromFloat(50000),
		Timestamp:   time.Now(),
		Reason:      "test insufficient position",
	}

	result, err := executor.Sell(ctx, sellOrder)

	// 应该返回错误
	assert.Error(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Equal(t, "insufficient position", result.Error)

	// 执行器状态不应改变
	assert.Equal(t, decimal.NewFromFloat(10000), executor.cash)
	assert.True(t, executor.position.IsZero())
	assert.Equal(t, 0, executor.totalTrades)
}

func TestBacktestExecutor_GetPortfolio(t *testing.T) {
	pair := cex.TradingPair{Base: "BTC", Quote: "USDT"}
	initialCapital := decimal.NewFromFloat(10000)
	executor := NewBacktestExecutor(pair, initialCapital)
	ctx := context.Background()

	// 初始状态
	portfolio, err := executor.GetPortfolio(ctx)
	require.NoError(t, err)
	assert.Equal(t, initialCapital, portfolio.Cash)
	assert.True(t, portfolio.Position.IsZero())

	// 买入后
	buyOrder := &BuyOrder{
		ID:          "test_buy",
		TradingPair: pair,
		Quantity:    decimal.NewFromFloat(0.1),
		Price:       decimal.NewFromFloat(50000),
		Timestamp:   time.Now(),
		Reason:      "test portfolio",
	}
	_, err = executor.Buy(ctx, buyOrder)
	require.NoError(t, err)

	portfolio, err = executor.GetPortfolio(ctx)
	require.NoError(t, err)
	assert.Equal(t, decimal.NewFromFloat(4995), portfolio.Cash) // 10000 - 5005 = 4995
	assert.Equal(t, decimal.NewFromFloat(0.1), portfolio.Position)
}

func TestBacktestExecutor_GetStatistics(t *testing.T) {
	pair := cex.TradingPair{Base: "BTC", Quote: "USDT"}
	initialCapital := decimal.NewFromFloat(10000)
	executor := NewBacktestExecutor(pair, initialCapital)
	ctx := context.Background()

	// 执行一次盈利交易
	buyOrder := &BuyOrder{
		ID:          "buy1",
		TradingPair: pair,
		Quantity:    decimal.NewFromFloat(0.1),
		Price:       decimal.NewFromFloat(50000),
		Timestamp:   time.Now(),
		Reason:      "test stats",
	}
	_, err := executor.Buy(ctx, buyOrder)
	require.NoError(t, err)

	sellOrder := &SellOrder{
		ID:          "sell1",
		TradingPair: pair,
		Quantity:    decimal.NewFromFloat(0.1),
		Price:       decimal.NewFromFloat(55000), // 盈利 10%
		Timestamp:   time.Now().Add(time.Hour),
		Reason:      "profit",
	}
	_, err = executor.Sell(ctx, sellOrder)
	require.NoError(t, err)

	stats := executor.GetStatistics()
	assert.Equal(t, 2, stats["total_trades"])

	// 验证统计结果不为空
	assert.NotNil(t, stats)
	assert.Contains(t, stats, "total_trades")
	assert.Contains(t, stats, "total_commission")
}

func TestBacktestExecutor_ComplexScenario(t *testing.T) {
	// 测试复杂交易场景：多次买入卖出
	pair := cex.TradingPair{Base: "BTC", Quote: "USDT"}
	initialCapital := decimal.NewFromFloat(20000)
	executor := NewBacktestExecutor(pair, initialCapital)
	ctx := context.Background()

	// 第一次买入
	_, err := executor.Buy(ctx, &BuyOrder{
		ID:          "buy1",
		TradingPair: pair,
		Quantity:    decimal.NewFromFloat(0.2),
		Price:       decimal.NewFromFloat(50000),
		Timestamp:   time.Now(),
		Reason:      "first buy",
	})
	require.NoError(t, err)

	// 第二次买入
	_, err = executor.Buy(ctx, &BuyOrder{
		ID:          "buy2",
		TradingPair: pair,
		Quantity:    decimal.NewFromFloat(0.1),
		Price:       decimal.NewFromFloat(52000),
		Timestamp:   time.Now().Add(time.Hour),
		Reason:      "second buy",
	})
	require.NoError(t, err)

	// 部分卖出
	_, err = executor.Sell(ctx, &SellOrder{
		ID:          "sell1",
		TradingPair: pair,
		Quantity:    decimal.NewFromFloat(0.15),
		Price:       decimal.NewFromFloat(55000),
		Timestamp:   time.Now().Add(2 * time.Hour),
		Reason:      "partial sell",
	})
	require.NoError(t, err)

	// 验证最终状态
	assert.Equal(t, decimal.NewFromFloat(0.15), executor.position) // 0.3 - 0.15 = 0.15
	assert.Equal(t, 3, executor.totalTrades)
	assert.Len(t, executor.orders, 3)

	// 验证现金计算
	// 初始: 20000
	// 买入1: -10010 (0.2 * 50000 * 1.001)
	// 买入2: -5205.2 (0.1 * 52000 * 1.001)
	// 卖出1: +8241.75 (0.15 * 55000 * 0.999)
	// 剩余现金应该 > 0
	assert.True(t, executor.cash.IsPositive())

	portfolio, err := executor.GetPortfolio(ctx)
	require.NoError(t, err)
	assert.True(t, portfolio.Cash.IsPositive())
	assert.Equal(t, decimal.NewFromFloat(0.15), portfolio.Position)
}

func TestBacktestExecutor_PrecisionHandling(t *testing.T) {
	// 测试高精度计算
	pair := cex.TradingPair{Base: "BTC", Quote: "USDT"}
	initialCapital, err := decimal.NewFromString("10000.123456789")
	require.NoError(t, err)
	executor := NewBacktestExecutor(pair, initialCapital)
	ctx := context.Background()

	buyOrder := &BuyOrder{
		ID:          "precision_test",
		TradingPair: pair,
		Quantity:    mustParseDecimal("0.123456789"),
		Price:       mustParseDecimal("50000.987654321"),
		Timestamp:   time.Now(),
		Reason:      "precision test",
	}

	result, err := executor.Buy(ctx, buyOrder)
	require.NoError(t, err)
	assert.True(t, result.Success)

	// 验证精度保持
	assert.Equal(t, buyOrder.Quantity, result.Quantity)
	assert.Equal(t, buyOrder.Price, result.Price)

	// 验证手续费计算精度
	expectedNotional := buyOrder.Quantity.Mul(buyOrder.Price)
	expectedCommission := expectedNotional.Mul(executor.commission)
	assert.Equal(t, expectedCommission, result.Commission)
}

// 基准测试
func BenchmarkBacktestExecutor_Buy(b *testing.B) {
	pair := cex.TradingPair{Base: "BTC", Quote: "USDT"}
	executor := NewBacktestExecutor(pair, decimal.NewFromFloat(1000000)) // 足够的资金
	ctx := context.Background()

	buyOrder := &BuyOrder{
		ID:          "benchmark",
		TradingPair: pair,
		Quantity:    decimal.NewFromFloat(0.01),
		Price:       decimal.NewFromFloat(50000),
		Timestamp:   time.Now(),
		Reason:      "benchmark",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buyOrder.ID = fmt.Sprintf("benchmark_%d", i)
		executor.Buy(ctx, buyOrder)
	}
}

func BenchmarkBacktestExecutor_Sell(b *testing.B) {
	pair := cex.TradingPair{Base: "BTC", Quote: "USDT"}
	executor := NewBacktestExecutor(pair, decimal.NewFromFloat(1000000))
	ctx := context.Background()

	// 先建立大量持仓
	executor.position = decimal.NewFromFloat(float64(b.N) * 0.01)

	sellOrder := &SellOrder{
		ID:          "benchmark",
		TradingPair: pair,
		Quantity:    decimal.NewFromFloat(0.01),
		Price:       decimal.NewFromFloat(50000),
		Timestamp:   time.Now(),
		Reason:      "benchmark",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sellOrder.ID = fmt.Sprintf("benchmark_%d", i)
		executor.Sell(ctx, sellOrder)
	}
}

// Test Close method
func TestBacktestExecutor_Close(t *testing.T) {
	pair := cex.TradingPair{Base: "BTC", Quote: "USDT"}
	executor := NewBacktestExecutor(pair, decimal.NewFromFloat(10000))

	// Close should not panic and should complete successfully
	assert.NotPanics(t, func() {
		err := executor.Close()
		assert.NoError(t, err)
	})
}

// Test GetOrders method
func TestBacktestExecutor_GetOrders(t *testing.T) {
	pair := cex.TradingPair{Base: "BTC", Quote: "USDT"}
	executor := NewBacktestExecutor(pair, decimal.NewFromFloat(10000))
	ctx := context.Background()

	// Initially should have no orders
	orders := executor.GetOrders()
	assert.Empty(t, orders)

	// Execute a buy order
	buyOrder := &BuyOrder{
		ID:          "test_buy",
		TradingPair: pair,
		Quantity:    decimal.NewFromFloat(0.1),
		Price:       decimal.NewFromFloat(50000),
		Timestamp:   time.Now(),
		Reason:      "test",
	}

	_, err := executor.Buy(ctx, buyOrder)
	require.NoError(t, err)

	// Should have one order now
	orders = executor.GetOrders()
	assert.Len(t, orders, 1)
	assert.Equal(t, OrderSideBuy, orders[0].Side)
	assert.Equal(t, buyOrder.Quantity, orders[0].Quantity)
	assert.Equal(t, buyOrder.Price, orders[0].Price)

	// Execute a sell order
	sellOrder := &SellOrder{
		ID:          "test_sell",
		TradingPair: pair,
		Quantity:    decimal.NewFromFloat(0.05),
		Price:       decimal.NewFromFloat(55000),
		Timestamp:   time.Now().Add(time.Hour),
		Reason:      "test sell",
	}

	_, err = executor.Sell(ctx, sellOrder)
	require.NoError(t, err)

	// Should have two orders now
	orders = executor.GetOrders()
	assert.Len(t, orders, 2)

	// Verify order details
	buyOrderResult := orders[0]
	sellOrderResult := orders[1]

	assert.Equal(t, OrderSideBuy, buyOrderResult.Side)
	assert.Equal(t, OrderSideSell, sellOrderResult.Side)
	assert.Equal(t, buyOrder.Quantity, buyOrderResult.Quantity)
	assert.Equal(t, sellOrder.Quantity, sellOrderResult.Quantity)
}
