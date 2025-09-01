package executor

import (
	"context"
	"fmt"
	"testing"
	"time"

	"tradingbot/src/cex"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

// mockCEXClient 用于测试的模拟CEX客户端
type mockCEXClient struct{}

func (m *mockCEXClient) GetName() string {
	return "mock"
}

func (m *mockCEXClient) GetDatabase() interface{} {
	return nil
}

func (m *mockCEXClient) GetTradingFee() float64 {
	return 0.001 // 0.1%
}

func (m *mockCEXClient) GetKlines(ctx context.Context, pair cex.TradingPair, interval string, limit int) ([]*cex.KlineData, error) {
	return []*cex.KlineData{
		{
			TradingPair: pair,
			OpenTime:    time.Now().Add(-time.Hour),
			Close:       decimal.NewFromFloat(50000),
			CloseTime:   time.Now(),
		},
	}, nil
}

func (m *mockCEXClient) GetKlinesWithTimeRange(ctx context.Context, pair cex.TradingPair, interval string, startTime, endTime time.Time, limit int) ([]*cex.KlineData, error) {
	return nil, nil
}

func (m *mockCEXClient) Buy(ctx context.Context, order cex.BuyOrderRequest) (*cex.OrderResult, error) {
	// 模拟边界情况检查
	if order.Quantity.IsZero() {
		return nil, fmt.Errorf("quantity must be greater than 0")
	}
	if order.Price.IsNegative() {
		return nil, fmt.Errorf("price must be greater than 0")
	}

	return &cex.OrderResult{
		OrderID:      "mock_buy_123",
		TradingPair:  order.TradingPair,
		Price:        order.Price,
		Quantity:     order.Quantity,
		Side:         cex.OrderSideBuy,
		Status:       "FILLED",
		Type:         order.Type,
		TransactTime: time.Now(),
	}, nil
}

func (m *mockCEXClient) Sell(ctx context.Context, order cex.SellOrderRequest) (*cex.OrderResult, error) {
	// 模拟边界情况检查
	if order.Quantity.IsZero() {
		return nil, fmt.Errorf("quantity must be greater than 0")
	}
	if order.Price.IsNegative() {
		return nil, fmt.Errorf("price must be greater than 0")
	}

	return &cex.OrderResult{
		OrderID:      "mock_sell_123",
		TradingPair:  order.TradingPair,
		Price:        order.Price,
		Quantity:     order.Quantity,
		Side:         cex.OrderSideSell,
		Status:       "FILLED",
		Type:         order.Type,
		TransactTime: time.Now(),
	}, nil
}

func (m *mockCEXClient) GetAccount(ctx context.Context) ([]*cex.AccountBalance, error) {
	return []*cex.AccountBalance{
		{Asset: "USDT", Free: decimal.NewFromFloat(10000), Locked: decimal.Zero},
		{Asset: "BTC", Free: decimal.Zero, Locked: decimal.Zero},
	}, nil
}

func (m *mockCEXClient) Ping(ctx context.Context) error {
	return nil
}

func TestNewLiveExecutor(t *testing.T) {
	mockClient := &mockCEXClient{}
	pair := cex.TradingPair{Base: "BTC", Quote: "USDT"}

	executor := NewLiveExecutor(mockClient, pair)

	assert.NotNil(t, executor)
	assert.Equal(t, mockClient, executor.cexClient)
	assert.Equal(t, pair, executor.tradingPair)
}

func TestLiveExecutor_Buy(t *testing.T) {
	mockClient := &mockCEXClient{}
	pair := cex.TradingPair{Base: "BTC", Quote: "USDT"}
	executor := NewLiveExecutor(mockClient, pair)

	order := &BuyOrder{
		ID:          "test_buy",
		TradingPair: pair,
		Type:        OrderTypeMarket,
		Quantity:    decimal.NewFromFloat(0.1),
		Price:       decimal.NewFromFloat(50000),
		Timestamp:   time.Now(),
		Reason:      "test buy",
	}

	ctx := context.Background()
	result, err := executor.Buy(ctx, order)

	// LiveExecutor 现在应该成功执行买入
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, OrderSideBuy, result.Side)
	assert.Equal(t, order.Quantity, result.Quantity)
	assert.Equal(t, order.Price, result.Price)
	assert.Equal(t, "mock_buy_123", result.OrderID)
}

func TestLiveExecutor_Sell(t *testing.T) {
	mockClient := &mockCEXClient{}
	pair := cex.TradingPair{Base: "BTC", Quote: "USDT"}
	executor := NewLiveExecutor(mockClient, pair)

	order := &SellOrder{
		ID:          "test_sell",
		TradingPair: pair,
		Type:        OrderTypeMarket,
		Quantity:    decimal.NewFromFloat(0.1),
		Price:       decimal.NewFromFloat(55000),
		Timestamp:   time.Now(),
		Reason:      "test sell",
	}

	ctx := context.Background()
	result, err := executor.Sell(ctx, order)

	// LiveExecutor 现在应该成功执行卖出
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, OrderSideSell, result.Side)
	assert.Equal(t, order.Quantity, result.Quantity)
	assert.Equal(t, order.Price, result.Price)
	assert.Equal(t, "mock_sell_123", result.OrderID)
}

func TestLiveExecutor_GetPortfolio(t *testing.T) {
	mockClient := &mockCEXClient{}
	pair := cex.TradingPair{Base: "BTC", Quote: "USDT"}
	executor := NewLiveExecutor(mockClient, pair)

	ctx := context.Background()
	portfolio, err := executor.GetPortfolio(ctx)

	// GetPortfolio 现在应该基于真实账户数据计算投资组合
	assert.NoError(t, err)
	assert.NotNil(t, portfolio)
	assert.True(t, decimal.NewFromFloat(10000).Equal(portfolio.Cash))         // USDT余额
	assert.True(t, portfolio.Position.IsZero())                               // BTC余额为0
	assert.True(t, decimal.NewFromFloat(50000).Equal(portfolio.CurrentPrice)) // 当前BTC价格
	assert.True(t, decimal.NewFromFloat(10000).Equal(portfolio.Portfolio))    // 总价值 = 10000 USDT + 0 BTC
	assert.WithinDuration(t, time.Now(), portfolio.Timestamp, time.Second)
}

func TestLiveExecutor_GetName(t *testing.T) {
	mockClient := &mockCEXClient{}
	pair := cex.TradingPair{Base: "BTC", Quote: "USDT"}
	executor := NewLiveExecutor(mockClient, pair)

	name := executor.GetName()
	assert.Equal(t, "LiveExecutor", name)
}

func TestLiveExecutor_Close(t *testing.T) {
	mockClient := &mockCEXClient{}
	pair := cex.TradingPair{Base: "BTC", Quote: "USDT"}
	executor := NewLiveExecutor(mockClient, pair)

	err := executor.Close()
	assert.NoError(t, err)
}

// 测试边界情况和异常处理
func TestLiveExecutor_EdgeCases(t *testing.T) {
	mockClient := &mockCEXClient{}
	pair := cex.TradingPair{Base: "BTC", Quote: "USDT"}
	executor := NewLiveExecutor(mockClient, pair)

	t.Run("nil order for Buy", func(t *testing.T) {
		ctx := context.Background()
		// LiveExecutor目前不处理nil订单，会panic，这是预期行为
		assert.Panics(t, func() {
			_, _ = executor.Buy(ctx, nil)
		})
	})

	t.Run("nil order for Sell", func(t *testing.T) {
		ctx := context.Background()
		// LiveExecutor目前不处理nil订单，会panic，这是预期行为
		assert.Panics(t, func() {
			_, _ = executor.Sell(ctx, nil)
		})
	})

	t.Run("zero quantity order", func(t *testing.T) {
		order := &BuyOrder{
			ID:          "test_zero",
			TradingPair: pair,
			Quantity:    decimal.Zero,
			Price:       decimal.NewFromFloat(50000),
			Timestamp:   time.Now(),
			Reason:      "test zero quantity",
		}

		ctx := context.Background()
		result, err := executor.Buy(ctx, order)

		// 即使数量为零，也应该返回一致的错误
		assert.Error(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.Success)
	})

	t.Run("negative price order", func(t *testing.T) {
		order := &SellOrder{
			ID:          "test_negative",
			TradingPair: pair,
			Quantity:    decimal.NewFromFloat(0.1),
			Price:       decimal.NewFromFloat(-1000),
			Timestamp:   time.Now(),
			Reason:      "test negative price",
		}

		ctx := context.Background()
		result, err := executor.Sell(ctx, order)

		// 即使价格为负，也应该返回一致的错误
		assert.Error(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.Success)
	})
}

// 基准测试
func BenchmarkLiveExecutor_Buy(b *testing.B) {
	mockClient := &mockCEXClient{}
	pair := cex.TradingPair{Base: "BTC", Quote: "USDT"}
	executor := NewLiveExecutor(mockClient, pair)

	order := &BuyOrder{
		ID:          "bench_buy",
		TradingPair: pair,
		Quantity:    decimal.NewFromFloat(0.1),
		Price:       decimal.NewFromFloat(50000),
		Timestamp:   time.Now(),
		Reason:      "benchmark buy",
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = executor.Buy(ctx, order)
	}
}

func BenchmarkLiveExecutor_GetPortfolio(b *testing.B) {
	mockClient := &mockCEXClient{}
	pair := cex.TradingPair{Base: "BTC", Quote: "USDT"}
	executor := NewLiveExecutor(mockClient, pair)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = executor.GetPortfolio(ctx)
	}
}
