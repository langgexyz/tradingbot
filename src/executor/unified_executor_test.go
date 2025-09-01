package executor

import (
	"context"
	"testing"
	"time"

	"tradingbot/src/cex"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUnifiedExecutor_WithBacktestOrderExecutor 测试统一执行器与回测订单执行器
func TestUnifiedExecutor_WithBacktestOrderExecutor(t *testing.T) {
	pair := cex.TradingPair{Base: "BTC", Quote: "USDT"}
	initialCapital := decimal.NewFromFloat(10000)

	// 创建回测订单执行器
	orderExecutor := NewBacktestOrderExecutor(pair)

	// 创建统一执行器
	executor := NewUnifiedExecutor(pair, initialCapital, orderExecutor)

	ctx := context.Background()

	// 测试买入订单
	buyOrder := &BuyOrder{
		ID:          "buy1",
		TradingPair: pair,
		Quantity:    decimal.NewFromFloat(0.1),
		Price:       decimal.NewFromFloat(50000),
		Timestamp:   time.Now(),
		Reason:      "test buy",
	}

	result, err := executor.Buy(ctx, buyOrder)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, OrderSideBuy, result.Side)

	// 验证状态更新
	stats := executor.GetStatistics()
	assert.Equal(t, 0, stats["total_trades"]) // 还没有完整交易对

	// 验证资金和持仓
	portfolio, err := executor.GetPortfolio(ctx)
	require.NoError(t, err)
	assert.True(t, portfolio.Cash.LessThan(initialCapital))      // 现金减少
	assert.True(t, portfolio.Position.GreaterThan(decimal.Zero)) // 持仓增加

	// 测试卖出订单
	sellOrder := &SellOrder{
		ID:          "sell1",
		TradingPair: pair,
		Quantity:    decimal.NewFromFloat(0.1),
		Price:       decimal.NewFromFloat(55000), // 盈利10%
		Timestamp:   time.Now().Add(time.Hour),
		Reason:      "test sell",
	}

	result, err = executor.Sell(ctx, sellOrder)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, OrderSideSell, result.Side)

	// 验证统计更新
	stats = executor.GetStatistics()
	assert.Equal(t, 1, stats["total_trades"])   // 1个完整交易对
	assert.Equal(t, 1, stats["winning_trades"]) // 1个盈利交易
	assert.Equal(t, 0, stats["losing_trades"])  // 0个亏损交易

	// 验证订单记录
	orders := executor.GetOrders()
	assert.Equal(t, 2, len(orders)) // 2个订单
}

// TestUnifiedExecutor_InsufficientCash 测试资金不足
func TestUnifiedExecutor_InsufficientCash(t *testing.T) {
	pair := cex.TradingPair{Base: "BTC", Quote: "USDT"}
	initialCapital := decimal.NewFromFloat(1000) // 很少的资金

	orderExecutor := NewBacktestOrderExecutor(pair)
	executor := NewUnifiedExecutor(pair, initialCapital, orderExecutor)

	ctx := context.Background()

	// 尝试买入超出资金的订单
	buyOrder := &BuyOrder{
		ID:          "buy1",
		TradingPair: pair,
		Quantity:    decimal.NewFromFloat(1), // 50000 USDT > 1000 USDT
		Price:       decimal.NewFromFloat(50000),
		Timestamp:   time.Now(),
		Reason:      "test insufficient cash",
	}

	result, err := executor.Buy(ctx, buyOrder)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient cash")
	assert.False(t, result.Success)
}

// TestUnifiedExecutor_InsufficientPosition 测试持仓不足
func TestUnifiedExecutor_InsufficientPosition(t *testing.T) {
	pair := cex.TradingPair{Base: "BTC", Quote: "USDT"}
	initialCapital := decimal.NewFromFloat(10000)

	orderExecutor := NewBacktestOrderExecutor(pair)
	executor := NewUnifiedExecutor(pair, initialCapital, orderExecutor)

	ctx := context.Background()

	// 没有持仓就尝试卖出
	sellOrder := &SellOrder{
		ID:          "sell1",
		TradingPair: pair,
		Quantity:    decimal.NewFromFloat(0.1),
		Price:       decimal.NewFromFloat(50000),
		Timestamp:   time.Now(),
		Reason:      "test insufficient position",
	}

	result, err := executor.Sell(ctx, sellOrder)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient position")
	assert.False(t, result.Success)
}

// TestUnifiedExecutor_WinRateCalculation 测试胜率计算
func TestUnifiedExecutor_WinRateCalculation(t *testing.T) {
	pair := cex.TradingPair{Base: "BTC", Quote: "USDT"}
	initialCapital := decimal.NewFromFloat(100000)

	orderExecutor := NewBacktestOrderExecutor(pair)
	executor := NewUnifiedExecutor(pair, initialCapital, orderExecutor)

	ctx := context.Background()

	// 第1个交易对：盈利
	_, err := executor.Buy(ctx, &BuyOrder{
		ID: "buy1", TradingPair: pair, Quantity: decimal.NewFromFloat(1),
		Price: decimal.NewFromFloat(50000), Timestamp: time.Now(), Reason: "test",
	})
	require.NoError(t, err)

	_, err = executor.Sell(ctx, &SellOrder{
		ID: "sell1", TradingPair: pair, Quantity: decimal.NewFromFloat(1),
		Price: decimal.NewFromFloat(55000), Timestamp: time.Now().Add(time.Hour), Reason: "profit",
	})
	require.NoError(t, err)

	// 第2个交易对：亏损
	_, err = executor.Buy(ctx, &BuyOrder{
		ID: "buy2", TradingPair: pair, Quantity: decimal.NewFromFloat(1),
		Price: decimal.NewFromFloat(55000), Timestamp: time.Now().Add(2 * time.Hour), Reason: "test",
	})
	require.NoError(t, err)

	_, err = executor.Sell(ctx, &SellOrder{
		ID: "sell2", TradingPair: pair, Quantity: decimal.NewFromFloat(1),
		Price: decimal.NewFromFloat(52000), Timestamp: time.Now().Add(3 * time.Hour), Reason: "loss",
	})
	require.NoError(t, err)

	stats := executor.GetStatistics()

	// 验证胜率计算：1盈利/2总交易 = 50%
	assert.Equal(t, 2, stats["total_trades"])   // 2个完整交易对
	assert.Equal(t, 1, stats["winning_trades"]) // 1个盈利交易对
	assert.Equal(t, 1, stats["losing_trades"])  // 1个亏损交易对

	// 计算胜率：1/2 = 50%
	winRate := float64(stats["winning_trades"].(int)) / float64(stats["total_trades"].(int)) * 100
	assert.Equal(t, 50.0, winRate)

	// 验证订单总数
	orders := executor.GetOrders()
	assert.Equal(t, 4, len(orders)) // 总共4个订单（2买+2卖）
}
