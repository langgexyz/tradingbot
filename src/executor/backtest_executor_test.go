package executor

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBacktestExecutor(t *testing.T) {
	symbol := "BTCUSDT"
	initialCapital := decimal.NewFromFloat(10000)

	executor := NewBacktestExecutor(symbol, initialCapital)

	assert.NotNil(t, executor)
	assert.Equal(t, "BacktestExecutor", executor.GetName())
	assert.Equal(t, symbol, executor.symbol)
	assert.Equal(t, initialCapital, executor.initialCapital)
	assert.Equal(t, decimal.NewFromFloat(0.001), executor.commission) // 默认0.1%

	ctx := context.Background()
	portfolio, err := executor.GetPortfolio(ctx)
	require.NoError(t, err)
	assert.Equal(t, initialCapital, portfolio.Cash)
	assert.True(t, portfolio.Position.IsZero())
}

func TestBacktestExecutor_SetCommission(t *testing.T) {
	executor := NewBacktestExecutor("BTCUSDT", decimal.NewFromFloat(10000))

	executor.SetCommission(0.002) // 0.2%
	assert.Equal(t, decimal.NewFromFloat(0.002), executor.commission)
}



func TestBacktestExecutor_ExecuteOrder_Buy(t *testing.T) {
	initialCapital := decimal.NewFromFloat(10000)
	executor := NewBacktestExecutor("BTCUSDT", initialCapital)
	ctx := context.Background()

	t.Run("successful buy order", func(t *testing.T) {
		order := &Order{
			Side:      OrderSideBuy,
			Quantity:  decimal.NewFromFloat(0.1),
			Price:     decimal.NewFromFloat(50000),
			Timestamp: time.Now(),
		}

		result, err := executor.ExecuteOrder(ctx, order)

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, OrderSideBuy, result.Side)
		assert.Equal(t, order.Quantity, result.Quantity)
		assert.True(t, result.Price.GreaterThan(decimal.Zero))
		assert.True(t, result.Success)

		// 验证投资组合状态
		portfolio, err := executor.GetPortfolio(ctx)
		require.NoError(t, err)

		expectedCost := order.Quantity.Mul(result.Price).Add(result.Commission)
		expectedCash := initialCapital.Sub(expectedCost)
		assert.True(t, portfolio.Cash.Equal(expectedCash))
		assert.Equal(t, order.Quantity, portfolio.Position)

		// 验证手续费计算
		expectedCommission := order.Quantity.Mul(result.Price).Mul(executor.commission)
		assert.True(t, result.Commission.Equal(expectedCommission))
	})

	t.Run("insufficient cash", func(t *testing.T) {
		// 尝试买入超过现金的数量
		order := &Order{
			Side:      OrderSideBuy,
			Quantity:  decimal.NewFromFloat(1), // 需要约50000，但现金不足
			Price:     decimal.NewFromFloat(50000),
			Timestamp: time.Now(),
		}

		result, err := executor.ExecuteOrder(ctx, order)

		assert.Error(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.Success)
		assert.Contains(t, result.Error, "insufficient cash")
		assert.Contains(t, err.Error(), "insufficient cash")
	})
}

func TestBacktestExecutor_ExecuteOrder_Sell(t *testing.T) {
	initialCapital := decimal.NewFromFloat(10000)
	executor := NewBacktestExecutor("BTCUSDT", initialCapital)
	ctx := context.Background()

	// 先买入建立仓位
	buyOrder := &Order{
		Side:      OrderSideBuy,
		Quantity:  decimal.NewFromFloat(0.15), // 减少数量以确保有足够现金
		Price:     decimal.NewFromFloat(50000),
		Timestamp: time.Now(),
	}
	buyResult, err := executor.ExecuteOrder(ctx, buyOrder)
	require.NoError(t, err)
	require.True(t, buyResult.Success)

	t.Run("successful sell order", func(t *testing.T) {
		sellOrder := &Order{
			Side:      OrderSideSell,
			Quantity:  decimal.NewFromFloat(0.1),
			Price:     decimal.NewFromFloat(52000),
			Timestamp: time.Now(),
		}

		result, err := executor.ExecuteOrder(ctx, sellOrder)

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, OrderSideSell, result.Side)
		assert.Equal(t, sellOrder.Quantity, result.Quantity)
		assert.True(t, result.Success)

		// 验证投资组合状态
		portfolio, err := executor.GetPortfolio(ctx)
		require.NoError(t, err)

		expectedPosition := decimal.NewFromFloat(0.05) // 0.15 - 0.1
		assert.True(t, portfolio.Position.Equal(expectedPosition))
	})

	t.Run("insufficient position", func(t *testing.T) {
		sellOrder := &Order{
			Side:      OrderSideSell,
			Quantity:  decimal.NewFromFloat(1), // 超过持仓
			Price:     decimal.NewFromFloat(52000),
			Timestamp: time.Now(),
		}

		result, err := executor.ExecuteOrder(ctx, sellOrder)

		assert.Error(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.Success)
		assert.Contains(t, result.Error, "insufficient position")
		assert.Contains(t, err.Error(), "insufficient position")
	})

	t.Run("sell all position", func(t *testing.T) {
		// 卖出剩余全部仓位
		portfolio, err := executor.GetPortfolio(ctx)
		require.NoError(t, err)

		sellOrder := &Order{
			Side:      OrderSideSell,
			Quantity:  portfolio.Position,
			Price:     decimal.NewFromFloat(55000),
			Timestamp: time.Now(),
		}

		_, err = executor.ExecuteOrder(ctx, sellOrder)

		require.NoError(t, err)

		newPortfolio, err := executor.GetPortfolio(ctx)
		require.NoError(t, err)
		assert.True(t, newPortfolio.Position.IsZero())
	})
}

func TestBacktestExecutor_GetPortfolio(t *testing.T) {
	initialCapital := decimal.NewFromFloat(10000)
	executor := NewBacktestExecutor("BTCUSDT", initialCapital)
	ctx := context.Background()

	portfolio, err := executor.GetPortfolio(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, portfolio)
	assert.Equal(t, initialCapital, portfolio.Cash)
	assert.True(t, portfolio.Position.IsZero())
}

func TestBacktestExecutor_GetName(t *testing.T) {
	executor := NewBacktestExecutor("BTCUSDT", decimal.NewFromFloat(10000))
	assert.Equal(t, "BacktestExecutor", executor.GetName())
}

func TestBacktestExecutor_Statistics(t *testing.T) {
	initialCapital := decimal.NewFromFloat(10000)
	executor := NewBacktestExecutor("BTCUSDT", initialCapital)
	ctx := context.Background()

	// 执行一系列交易
	trades := []struct {
		side     OrderSide
		quantity float64
		price    float64
	}{
		{OrderSideBuy, 0.1, 50000},  // 买入
		{OrderSideSell, 0.1, 52000}, // 盈利卖出
		{OrderSideBuy, 0.1, 51000},  // 再次买入
		{OrderSideSell, 0.1, 49000}, // 亏损卖出
	}

	for _, trade := range trades {
		order := &Order{
			Side:      trade.side,
			Quantity:  decimal.NewFromFloat(trade.quantity),
			Price:     decimal.NewFromFloat(trade.price),
			Timestamp: time.Now(),
		}
		executor.ExecuteOrder(ctx, order)
	}

	t.Run("verify statistics", func(t *testing.T) {
		assert.Equal(t, 4, executor.totalTrades)
		assert.True(t, executor.totalCommission.GreaterThan(decimal.Zero)) // 有手续费
		assert.Len(t, executor.orders, 4)                                  // 4笔交易记录
	})
}

func TestBacktestExecutor_PriceSlippage(t *testing.T) {
	initialCapital := decimal.NewFromFloat(10000)
	executor := NewBacktestExecutor("BTCUSDT", initialCapital)
	executor.SetSlippage(0.001) // 设置0.1%滑点
	ctx := context.Background()

	t.Run("buy order slippage", func(t *testing.T) {
		order := &Order{
			Side:      OrderSideBuy,
			Quantity:  decimal.NewFromFloat(0.1),
			Price:     decimal.NewFromFloat(50000),
			Timestamp: time.Now(),
		}

		result, err := executor.ExecuteOrder(ctx, order)

		require.NoError(t, err)
		// 买入价格应该略高于市场价格（滑点）
		expectedPrice := order.Price.Mul(decimal.NewFromFloat(1.001)) // 加上滑点
		assert.True(t, result.Price.Equal(expectedPrice))
	})

	t.Run("sell order slippage", func(t *testing.T) {
		// 先建立仓位
		buyOrder := &Order{
			Side:      OrderSideBuy,
			Quantity:  decimal.NewFromFloat(0.1),
			Price:     decimal.NewFromFloat(50000),
			Timestamp: time.Now(),
		}
		executor.ExecuteOrder(ctx, buyOrder)

		sellOrder := &Order{
			Side:      OrderSideSell,
			Quantity:  decimal.NewFromFloat(0.1),
			Price:     decimal.NewFromFloat(52000),
			Timestamp: time.Now(),
		}

		result, err := executor.ExecuteOrder(ctx, sellOrder)

		require.NoError(t, err)
		// 卖出价格应该略低于市场价格（滑点）
		expectedPrice := sellOrder.Price.Mul(decimal.NewFromFloat(0.999)) // 减去滑点
		assert.True(t, result.Price.Equal(expectedPrice))
	})
}

// 基准测试
func BenchmarkBacktestExecutor_ExecuteOrder(b *testing.B) {
	initialCapital := decimal.NewFromFloat(10000)
	executor := NewBacktestExecutor("BTCUSDT", initialCapital)
	ctx := context.Background()

	// 先建立一些仓位
	buyOrder := &Order{
		Side:      OrderSideBuy,
		Quantity:  decimal.NewFromFloat(0.5),
		Price:     decimal.NewFromFloat(50000),
		Timestamp: time.Now(),
	}
	executor.ExecuteOrder(ctx, buyOrder)

	orders := []*Order{
		{
			Side:      OrderSideBuy,
			Quantity:  decimal.NewFromFloat(0.01),
			Price:     decimal.NewFromFloat(50000),
			Timestamp: time.Now(),
		},
		{
			Side:      OrderSideSell,
			Quantity:  decimal.NewFromFloat(0.01),
			Price:     decimal.NewFromFloat(52000),
			Timestamp: time.Now(),
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		order := orders[i%len(orders)]
		executor.ExecuteOrder(ctx, order)
	}
}
