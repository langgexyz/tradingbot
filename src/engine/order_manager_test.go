package engine

import (
	"context"
	"fmt"
	"testing"
	"time"

	"tradingbot/src/cex"
	"tradingbot/src/executor"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestError 已在 datafeed_test.go 中定义

// CreateTestKlineWithPrices 创建指定价格的测试K线
func CreateTestKlineWithPrices(openTime time.Time, open, high, low, close decimal.Decimal) *cex.KlineData {
	return &cex.KlineData{
		OpenTime:  openTime,
		CloseTime: openTime.Add(4 * time.Hour),
		Open:      open,
		High:      high,
		Low:       low,
		Close:     close,
		Volume:    decimal.NewFromInt(1000),
	}
}

// CreateTestPendingOrder 创建测试挂单
func CreateTestPendingOrder(orderType PendingOrderType, id string, price decimal.Decimal) *PendingOrder {
	return &PendingOrder{
		ID:           id,
		Type:         orderType,
		TradingPair:  cex.TradingPair{Base: "BTC", Quote: "USDT"},
		Quantity:     decimal.NewFromInt(1),
		Price:        price,
		CreateTime:   time.Now(),
		ExpireTime:   nil,
		Reason:       "test order",
		OriginSignal: "BUY",
	}
}

// MockExecutor for testing
type mockOrderExecutor struct {
	cash           decimal.Decimal
	position       decimal.Decimal
	buyResults     []*executor.OrderResult
	sellResults    []*executor.OrderResult
	shouldFailBuy  bool
	shouldFailSell bool
	buyCallCount   int
	sellCallCount  int
}

func newMockOrderExecutor(cash, position decimal.Decimal) *mockOrderExecutor {
	return &mockOrderExecutor{
		cash:     cash,
		position: position,
	}
}

func (m *mockOrderExecutor) Buy(ctx context.Context, order *executor.BuyOrder) (*executor.OrderResult, error) {
	m.buyCallCount++

	if m.shouldFailBuy {
		return nil, assert.AnError
	}

	// 模拟成功买入
	cost := order.Quantity.Mul(order.Price)
	if cost.GreaterThan(m.cash) {
		return &executor.OrderResult{
			Success: false,
			Error:   "insufficient cash",
		}, nil
	}

	m.cash = m.cash.Sub(cost)
	m.position = m.position.Add(order.Quantity)

	result := &executor.OrderResult{
		Success:   true,
		OrderID:   order.ID,
		Quantity:  order.Quantity,
		Price:     order.Price,
		Timestamp: order.Timestamp,
		Side:      "BUY",
	}

	m.buyResults = append(m.buyResults, result)
	return result, nil
}

func (m *mockOrderExecutor) Sell(ctx context.Context, order *executor.SellOrder) (*executor.OrderResult, error) {
	m.sellCallCount++

	if m.shouldFailSell {
		return nil, TestError
	}

	// 模拟成功卖出
	if order.Quantity.GreaterThan(m.position) {
		return &executor.OrderResult{
			Success: false,
			Error:   "insufficient position",
		}, nil
	}

	revenue := order.Quantity.Mul(order.Price)
	m.cash = m.cash.Add(revenue)
	m.position = m.position.Sub(order.Quantity)

	result := &executor.OrderResult{
		Success:   true,
		OrderID:   order.ID,
		Quantity:  order.Quantity,
		Price:     order.Price,
		Timestamp: order.Timestamp,
		Side:      "SELL",
	}

	m.sellResults = append(m.sellResults, result)
	return result, nil
}

func (m *mockOrderExecutor) GetPortfolio(ctx context.Context) (*executor.Portfolio, error) {
	return &executor.Portfolio{
		Cash:     m.cash,
		Position: m.position,
	}, nil
}

func (m *mockOrderExecutor) GetStatistics() map[string]interface{} {
	return map[string]interface{}{
		"cash":     m.cash,
		"position": m.position,
	}
}

func (m *mockOrderExecutor) GetOrders() []executor.OrderResult {
	var allOrders []executor.OrderResult
	for _, result := range m.buyResults {
		allOrders = append(allOrders, *result)
	}
	for _, result := range m.sellResults {
		allOrders = append(allOrders, *result)
	}
	return allOrders
}

func (m *mockOrderExecutor) GetName() string {
	return "MockOrderExecutor"
}

func (m *mockOrderExecutor) Close() error {
	return nil
}

// ============================================================================
// BacktestOrderManager 测试
// ============================================================================

func TestBacktestOrderManager_NewBacktestOrderManager(t *testing.T) {
	mockExec := newMockOrderExecutor(decimal.NewFromInt(1000), decimal.Zero)
	manager := NewBacktestOrderManager(mockExec)

	assert.NotNil(t, manager)
	assert.Equal(t, mockExec, manager.executor)
	assert.NotNil(t, manager.pendingOrders)
	assert.Equal(t, 0, len(manager.pendingOrders))
	assert.WithinDuration(t, time.Now(), manager.currentTime, 1*time.Second)
}

func TestBacktestOrderManager_PlaceOrder(t *testing.T) {
	mockExec := newMockOrderExecutor(decimal.NewFromInt(1000), decimal.Zero)
	manager := NewBacktestOrderManager(mockExec)

	order := CreateTestPendingOrder(PendingOrderTypeBuyLimit, "test_buy_1", decimal.NewFromFloat(50000))

	ctx := context.Background()
	err := manager.PlaceOrder(ctx, order)

	assert.NoError(t, err)
	assert.Equal(t, 1, manager.GetOrderCount())

	pendingOrders := manager.GetPendingOrders()
	require.Len(t, pendingOrders, 1)
	assert.Equal(t, order.ID, pendingOrders[0].ID)
	assert.Equal(t, order.Type, pendingOrders[0].Type)
	assert.Equal(t, order.Price, pendingOrders[0].Price)
}

func TestBacktestOrderManager_CancelOrder(t *testing.T) {
	mockExec := newMockOrderExecutor(decimal.NewFromInt(1000), decimal.Zero)
	manager := NewBacktestOrderManager(mockExec)

	order := CreateTestPendingOrder(PendingOrderTypeBuyLimit, "test_buy_1", decimal.NewFromFloat(50000))

	ctx := context.Background()

	// 下挂单
	err := manager.PlaceOrder(ctx, order)
	require.NoError(t, err)
	assert.Equal(t, 1, manager.GetOrderCount())

	// 取消挂单
	err = manager.CancelOrder(ctx, order.ID)
	assert.NoError(t, err)
	assert.Equal(t, 0, manager.GetOrderCount())

	// 取消不存在的挂单应该返回错误
	err = manager.CancelOrder(ctx, "non_existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "挂单不存在")
}

func TestBacktestOrderManager_CancelAllOrders(t *testing.T) {
	mockExec := newMockOrderExecutor(decimal.NewFromInt(1000), decimal.Zero)
	manager := NewBacktestOrderManager(mockExec)

	ctx := context.Background()

	// 下多个挂单
	order1 := CreateTestPendingOrder(PendingOrderTypeBuyLimit, "buy_1", decimal.NewFromFloat(50000))
	order2 := CreateTestPendingOrder(PendingOrderTypeSellLimit, "sell_1", decimal.NewFromFloat(51000))

	err := manager.PlaceOrder(ctx, order1)
	require.NoError(t, err)
	err = manager.PlaceOrder(ctx, order2)
	require.NoError(t, err)

	assert.Equal(t, 2, manager.GetOrderCount())

	// 取消所有挂单
	err = manager.CancelAllOrders(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 0, manager.GetOrderCount())
}

func TestBacktestOrderManager_CheckAndExecuteOrders_BuyLimit(t *testing.T) {
	mockExec := newMockOrderExecutor(decimal.NewFromInt(100000), decimal.Zero) // 增加初始资金
	manager := NewBacktestOrderManager(mockExec)

	ctx := context.Background()
	orderPrice := decimal.NewFromFloat(50000)

	// 下买入限价单
	order := CreateTestPendingOrder(PendingOrderTypeBuyLimit, "buy_1", orderPrice)
	err := manager.PlaceOrder(ctx, order)
	require.NoError(t, err)

	tests := []struct {
		name           string
		klineHigh      decimal.Decimal
		klineLow       decimal.Decimal
		klineOpen      decimal.Decimal
		shouldExecute  bool
		executionPrice decimal.Decimal
	}{
		{
			name:          "价格高于挂单价，不执行",
			klineHigh:     decimal.NewFromFloat(51000),
			klineLow:      decimal.NewFromFloat(50100), // Low > 挂单价
			klineOpen:     decimal.NewFromFloat(50500),
			shouldExecute: false,
		},
		{
			name:           "价格触及挂单价，执行",
			klineHigh:      decimal.NewFromFloat(51000),
			klineLow:       decimal.NewFromFloat(49900), // Low <= 挂单价
			klineOpen:      decimal.NewFromFloat(50500), // Open > 挂单价
			shouldExecute:  true,
			executionPrice: orderPrice, // 使用挂单价执行
		},
		{
			name:           "开盘价低于挂单价，用开盘价执行",
			klineHigh:      decimal.NewFromFloat(51000),
			klineLow:       decimal.NewFromFloat(49000),
			klineOpen:      decimal.NewFromFloat(49500), // Open < 挂单价
			shouldExecute:  true,
			executionPrice: decimal.NewFromFloat(49500), // 使用开盘价执行
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 重置挂单状态
			manager.pendingOrders = map[string]*PendingOrder{
				order.ID: order,
			}
			mockExec.buyCallCount = 0

			kline := CreateTestKlineWithPrices(
				time.Now(),
				tt.klineOpen,
				tt.klineHigh,
				tt.klineLow,
				tt.klineOpen,
			)

			results, err := manager.CheckAndExecuteOrders(ctx, kline)
			require.NoError(t, err)

			if tt.shouldExecute {
				assert.Len(t, results, 1)
				assert.True(t, results[0].Success)
				assert.Equal(t, order.ID, results[0].OrderID)
				assert.True(t, results[0].Price.Equal(tt.executionPrice))
				assert.Equal(t, 1, mockExec.buyCallCount)
				assert.Equal(t, 0, manager.GetOrderCount()) // 挂单已执行，应被移除
			} else {
				assert.Len(t, results, 0)
				assert.Equal(t, 0, mockExec.buyCallCount)
				assert.Equal(t, 1, manager.GetOrderCount()) // 挂单未执行，仍存在
			}
		})
	}
}

func TestBacktestOrderManager_CheckAndExecuteOrders_SellLimit(t *testing.T) {
	mockExec := newMockOrderExecutor(decimal.NewFromInt(1000), decimal.NewFromInt(2)) // 有持仓
	manager := NewBacktestOrderManager(mockExec)

	ctx := context.Background()
	orderPrice := decimal.NewFromFloat(51000)

	// 下卖出限价单
	order := CreateTestPendingOrder(PendingOrderTypeSellLimit, "sell_1", orderPrice)
	err := manager.PlaceOrder(ctx, order)
	require.NoError(t, err)

	tests := []struct {
		name           string
		klineHigh      decimal.Decimal
		klineLow       decimal.Decimal
		klineOpen      decimal.Decimal
		shouldExecute  bool
		executionPrice decimal.Decimal
	}{
		{
			name:          "价格低于挂单价，不执行",
			klineHigh:     decimal.NewFromFloat(50900), // High < 挂单价
			klineLow:      decimal.NewFromFloat(50000),
			klineOpen:     decimal.NewFromFloat(50500),
			shouldExecute: false,
		},
		{
			name:           "价格触及挂单价，执行",
			klineHigh:      decimal.NewFromFloat(51100), // High >= 挂单价
			klineLow:       decimal.NewFromFloat(50000),
			klineOpen:      decimal.NewFromFloat(50500), // Open < 挂单价
			shouldExecute:  true,
			executionPrice: orderPrice, // 使用挂单价执行
		},
		{
			name:           "开盘价高于挂单价，用开盘价执行",
			klineHigh:      decimal.NewFromFloat(52000),
			klineLow:       decimal.NewFromFloat(50000),
			klineOpen:      decimal.NewFromFloat(51500), // Open > 挂单价
			shouldExecute:  true,
			executionPrice: decimal.NewFromFloat(51500), // 使用开盘价执行
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 重置挂单状态和执行器状态
			manager.pendingOrders = map[string]*PendingOrder{
				order.ID: order,
			}
			mockExec.sellCallCount = 0
			mockExec.position = decimal.NewFromInt(2) // 重置持仓

			kline := CreateTestKlineWithPrices(
				time.Now(),
				tt.klineOpen,
				tt.klineHigh,
				tt.klineLow,
				tt.klineOpen,
			)

			results, err := manager.CheckAndExecuteOrders(ctx, kline)
			require.NoError(t, err)

			if tt.shouldExecute {
				assert.Len(t, results, 1)
				assert.True(t, results[0].Success)
				assert.Equal(t, order.ID, results[0].OrderID)
				assert.True(t, results[0].Price.Equal(tt.executionPrice))
				assert.Equal(t, 1, mockExec.sellCallCount)
				assert.Equal(t, 0, manager.GetOrderCount()) // 挂单已执行，应被移除
			} else {
				assert.Len(t, results, 0)
				assert.Equal(t, 0, mockExec.sellCallCount)
				assert.Equal(t, 1, manager.GetOrderCount()) // 挂单未执行，仍存在
			}
		})
	}
}

func TestBacktestOrderManager_CheckAndExecuteOrders_OrderExpiry(t *testing.T) {
	mockExec := newMockOrderExecutor(decimal.NewFromInt(1000), decimal.Zero)
	manager := NewBacktestOrderManager(mockExec)

	ctx := context.Background()

	// 创建已过期的挂单
	expireTime := time.Now().Add(-1 * time.Hour) // 1小时前过期
	order := CreateTestPendingOrder(PendingOrderTypeBuyLimit, "buy_expired", decimal.NewFromFloat(50000))
	order.ExpireTime = &expireTime

	err := manager.PlaceOrder(ctx, order)
	require.NoError(t, err)
	assert.Equal(t, 1, manager.GetOrderCount())

	// 创建当前时间的K线
	kline := CreateTestKlineWithPrices(
		time.Now(),
		decimal.NewFromFloat(49000), // 价格触发条件
		decimal.NewFromFloat(50000),
		decimal.NewFromFloat(48000),
		decimal.NewFromFloat(49500),
	)

	results, err := manager.CheckAndExecuteOrders(ctx, kline)
	require.NoError(t, err)

	// 挂单应该被过期移除，不应该执行
	assert.Len(t, results, 0)
	assert.Equal(t, 0, manager.GetOrderCount())
	assert.Equal(t, 0, mockExec.buyCallCount)
}

func TestBacktestOrderManager_CheckAndExecuteOrders_ExecutionFailure(t *testing.T) {
	mockExec := newMockOrderExecutor(decimal.NewFromInt(10), decimal.Zero) // 资金不足
	manager := NewBacktestOrderManager(mockExec)

	ctx := context.Background()

	// 下一个需要大量资金的买入单
	order := CreateTestPendingOrder(PendingOrderTypeBuyLimit, "buy_expensive", decimal.NewFromFloat(50000))
	order.Quantity = decimal.NewFromInt(1) // 需要50000资金，但只有10

	err := manager.PlaceOrder(ctx, order)
	require.NoError(t, err)

	// 价格触及挂单条件
	kline := CreateTestKlineWithPrices(
		time.Now(),
		decimal.NewFromFloat(49000),
		decimal.NewFromFloat(51000),
		decimal.NewFromFloat(48000), // Low < 挂单价，应该触发
		decimal.NewFromFloat(49500),
	)

	results, err := manager.CheckAndExecuteOrders(ctx, kline)
	require.NoError(t, err)

	// 执行失败，挂单应该保留
	assert.Len(t, results, 0)
	assert.Equal(t, 1, manager.GetOrderCount()) // 挂单仍存在
	assert.Equal(t, 1, mockExec.buyCallCount)   // 尝试了执行
}

func TestBacktestOrderManager_CheckAndExecuteOrders_MultipleOrders(t *testing.T) {
	mockExec := newMockOrderExecutor(decimal.NewFromInt(100000), decimal.NewFromInt(2))
	manager := NewBacktestOrderManager(mockExec)

	ctx := context.Background()

	// 下多个挂单
	buyOrder := CreateTestPendingOrder(PendingOrderTypeBuyLimit, "buy_1", decimal.NewFromFloat(49000))
	sellOrder := CreateTestPendingOrder(PendingOrderTypeSellLimit, "sell_1", decimal.NewFromFloat(51000))

	err := manager.PlaceOrder(ctx, buyOrder)
	require.NoError(t, err)
	err = manager.PlaceOrder(ctx, sellOrder)
	require.NoError(t, err)

	assert.Equal(t, 2, manager.GetOrderCount())

	// K线同时触发两个挂单
	kline := CreateTestKlineWithPrices(
		time.Now(),
		decimal.NewFromFloat(50000),
		decimal.NewFromFloat(51500), // High >= 卖出挂单价
		decimal.NewFromFloat(48500), // Low <= 买入挂单价
		decimal.NewFromFloat(50500),
	)

	results, err := manager.CheckAndExecuteOrders(ctx, kline)
	require.NoError(t, err)

	// 两个挂单都应该执行
	assert.Len(t, results, 2)
	assert.Equal(t, 0, manager.GetOrderCount()) // 所有挂单都已执行
	assert.Equal(t, 1, mockExec.buyCallCount)
	assert.Equal(t, 1, mockExec.sellCallCount)
}

func TestBacktestOrderManager_GetPendingOrders(t *testing.T) {
	mockExec := newMockOrderExecutor(decimal.NewFromInt(1000), decimal.Zero)
	manager := NewBacktestOrderManager(mockExec)

	ctx := context.Background()

	// 初始应该为空
	orders := manager.GetPendingOrders()
	assert.Len(t, orders, 0)

	// 下几个挂单
	order1 := CreateTestPendingOrder(PendingOrderTypeBuyLimit, "buy_1", decimal.NewFromFloat(50000))
	order2 := CreateTestPendingOrder(PendingOrderTypeSellLimit, "sell_1", decimal.NewFromFloat(51000))

	err := manager.PlaceOrder(ctx, order1)
	require.NoError(t, err)
	err = manager.PlaceOrder(ctx, order2)
	require.NoError(t, err)

	// 获取挂单列表
	orders = manager.GetPendingOrders()
	assert.Len(t, orders, 2)

	// 验证返回的是副本，不是原始map
	orders[0] = nil
	actualOrders := manager.GetPendingOrders()
	assert.Len(t, actualOrders, 2)
	assert.NotNil(t, actualOrders[0])
}

func TestBacktestOrderManager_GetOrderCount(t *testing.T) {
	mockExec := newMockOrderExecutor(decimal.NewFromInt(1000), decimal.Zero)
	manager := NewBacktestOrderManager(mockExec)

	ctx := context.Background()

	assert.Equal(t, 0, manager.GetOrderCount())

	// 下一个挂单
	order := CreateTestPendingOrder(PendingOrderTypeBuyLimit, "buy_1", decimal.NewFromFloat(50000))
	err := manager.PlaceOrder(ctx, order)
	require.NoError(t, err)

	assert.Equal(t, 1, manager.GetOrderCount())

	// 取消挂单
	err = manager.CancelOrder(ctx, order.ID)
	require.NoError(t, err)

	assert.Equal(t, 0, manager.GetOrderCount())
}

// ============================================================================
// 复杂场景测试
// ============================================================================

func TestBacktestOrderManager_ComplexTradingScenario(t *testing.T) {
	mockExec := newMockOrderExecutor(decimal.NewFromInt(10000), decimal.Zero)
	manager := NewBacktestOrderManager(mockExec)

	ctx := context.Background()

	// 场景：先买入，再卖出，模拟完整交易流程

	// 1. 下买入限价单
	buyOrder := CreateTestPendingOrder(PendingOrderTypeBuyLimit, "buy_btc", decimal.NewFromFloat(50000))
	buyOrder.Quantity = decimal.NewFromFloat(0.1) // 买入0.1 BTC

	err := manager.PlaceOrder(ctx, buyOrder)
	require.NoError(t, err)

	// 2. K线触发买入
	buyKline := CreateTestKlineWithPrices(
		time.Now(),
		decimal.NewFromFloat(49500),
		decimal.NewFromFloat(51000),
		decimal.NewFromFloat(49000), // 触发买入
		decimal.NewFromFloat(50500),
	)

	results, err := manager.CheckAndExecuteOrders(ctx, buyKline)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, executor.OrderSideBuy, results[0].Side)
	assert.Equal(t, 0, manager.GetOrderCount())

	// 验证执行器状态
	portfolio, err := mockExec.GetPortfolio(ctx)
	require.NoError(t, err)
	assert.True(t, portfolio.Position.Equal(decimal.NewFromFloat(0.1)))

	// 3. 下卖出限价单
	sellOrder := CreateTestPendingOrder(PendingOrderTypeSellLimit, "sell_btc", decimal.NewFromFloat(55000))
	sellOrder.Quantity = decimal.NewFromFloat(0.1) // 卖出全部持仓

	err = manager.PlaceOrder(ctx, sellOrder)
	require.NoError(t, err)

	// 4. K线触发卖出
	sellKline := CreateTestKlineWithPrices(
		time.Now().Add(4*time.Hour),
		decimal.NewFromFloat(54500),
		decimal.NewFromFloat(55500), // 触发卖出
		decimal.NewFromFloat(54000),
		decimal.NewFromFloat(55000),
	)

	results, err = manager.CheckAndExecuteOrders(ctx, sellKline)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, executor.OrderSideSell, results[0].Side)
	assert.Equal(t, 0, manager.GetOrderCount())

	// 验证最终状态
	portfolio, err = mockExec.GetPortfolio(ctx)
	require.NoError(t, err)
	assert.True(t, portfolio.Position.IsZero())
	assert.True(t, portfolio.Cash.GreaterThan(decimal.NewFromInt(10000))) // 应该盈利
}

func TestBacktestOrderManager_OrderPriorityAndExpiry(t *testing.T) {
	mockExec := newMockOrderExecutor(decimal.NewFromInt(200000), decimal.Zero) // 增加资金
	manager := NewBacktestOrderManager(mockExec)

	ctx := context.Background()
	currentTime := time.Now()

	// 创建不同过期时间的挂单
	longExpiry := currentTime.Add(2 * time.Hour)
	shortExpiry := currentTime.Add(30 * time.Minute)

	order1 := CreateTestPendingOrder(PendingOrderTypeBuyLimit, "buy_long", decimal.NewFromFloat(50000))
	order1.ExpireTime = &longExpiry

	order2 := CreateTestPendingOrder(PendingOrderTypeBuyLimit, "buy_short", decimal.NewFromFloat(50000))
	order2.ExpireTime = &shortExpiry

	err := manager.PlaceOrder(ctx, order1)
	require.NoError(t, err)
	err = manager.PlaceOrder(ctx, order2)
	require.NoError(t, err)

	assert.Equal(t, 2, manager.GetOrderCount())

	// 模拟1小时后的K线（order2应该过期，order1仍有效）
	futureTime := currentTime.Add(1 * time.Hour)
	kline := CreateTestKlineWithPrices(
		futureTime,
		decimal.NewFromFloat(49000),
		decimal.NewFromFloat(51000),
		decimal.NewFromFloat(48000), // 触发条件
		decimal.NewFromFloat(50000),
	)

	results, err := manager.CheckAndExecuteOrders(ctx, kline)
	require.NoError(t, err)

	// 只有order1应该执行，order2应该过期
	assert.Len(t, results, 1)
	assert.Equal(t, order1.ID, results[0].OrderID)
	assert.Equal(t, 0, manager.GetOrderCount()) // 一个执行，一个过期，都被移除
}

// ============================================================================
// LiveOrderManager 测试
// ============================================================================

func TestLiveOrderManager_NewLiveOrderManager(t *testing.T) {
	mockClient := &MockCEXClient{}
	manager := NewLiveOrderManager(mockClient)

	assert.NotNil(t, manager)
	assert.Equal(t, mockClient, manager.cexClient)
	assert.NotNil(t, manager.pendingOrders)
	assert.Equal(t, 0, len(manager.pendingOrders))
}

func TestLiveOrderManager_PlaceOrder_NotImplemented(t *testing.T) {
	mockClient := &MockCEXClient{}
	manager := NewLiveOrderManager(mockClient)

	order := CreateTestPendingOrder(PendingOrderTypeBuyLimit, "live_buy", decimal.NewFromFloat(50000))

	ctx := context.Background()
	err := manager.PlaceOrder(ctx, order)

	// 应该返回未实现错误
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented")

	// 但挂单应该被添加到本地管理
	assert.Equal(t, 1, manager.GetOrderCount())
}

func TestLiveOrderManager_CancelOrder_NotImplemented(t *testing.T) {
	mockClient := &MockCEXClient{}
	manager := NewLiveOrderManager(mockClient)

	ctx := context.Background()
	err := manager.CancelOrder(ctx, "any_id")

	// 应该返回未实现错误
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented")
}

func TestLiveOrderManager_CheckAndExecuteOrders_NotImplemented(t *testing.T) {
	mockClient := &MockCEXClient{}
	manager := NewLiveOrderManager(mockClient)

	kline := CreateTestKlineWithPrices(
		time.Now(),
		decimal.NewFromFloat(50000),
		decimal.NewFromFloat(51000),
		decimal.NewFromFloat(49000),
		decimal.NewFromFloat(50500),
	)

	ctx := context.Background()
	results, err := manager.CheckAndExecuteOrders(ctx, kline)

	// 应该返回未实现错误
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented")
	assert.Len(t, results, 0)
}

// ============================================================================
// 并发安全测试
// ============================================================================

func TestBacktestOrderManager_ConcurrentAccess(t *testing.T) {
	mockExec := newMockOrderExecutor(decimal.NewFromInt(100000), decimal.NewFromInt(10))
	manager := NewBacktestOrderManager(mockExec)

	ctx := context.Background()

	// 启动多个goroutine并发操作
	const numGoroutines = 10
	const ordersPerGoroutine = 5

	// 并发下挂单
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			for j := 0; j < ordersPerGoroutine; j++ {
				orderID := fmt.Sprintf("order_%d_%d", goroutineID, j)
				price := decimal.NewFromFloat(50000 + float64(j*100))
				order := CreateTestPendingOrder(PendingOrderTypeBuyLimit, orderID, price)

				err := manager.PlaceOrder(ctx, order)
				assert.NoError(t, err)
			}
		}(i)
	}

	// 等待所有goroutine完成
	time.Sleep(100 * time.Millisecond)

	// 验证所有挂单都被正确添加
	assert.Equal(t, numGoroutines*ordersPerGoroutine, manager.GetOrderCount())

	// 并发取消挂单
	pendingOrders := manager.GetPendingOrders()
	for i, order := range pendingOrders {
		go func(orderID string, index int) {
			if index%2 == 0 { // 只取消一半
				err := manager.CancelOrder(ctx, orderID)
				assert.NoError(t, err)
			}
		}(order.ID, i)
	}

	// 等待取消操作完成
	time.Sleep(100 * time.Millisecond)

	// 验证剩余挂单数量
	remainingCount := manager.GetOrderCount()
	assert.GreaterOrEqual(t, remainingCount, numGoroutines*ordersPerGoroutine/2-1)
	assert.LessOrEqual(t, remainingCount, numGoroutines*ordersPerGoroutine/2+1)
}

// ============================================================================
// 性能测试
// ============================================================================

func TestBacktestOrderManager_Performance(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过性能测试")
	}

	mockExec := newMockOrderExecutor(decimal.NewFromInt(1000000), decimal.NewFromInt(100))
	manager := NewBacktestOrderManager(mockExec)

	ctx := context.Background()

	// 大量挂单
	const numOrders = 1000
	for i := 0; i < numOrders; i++ {
		orderID := fmt.Sprintf("perf_order_%d", i)
		price := decimal.NewFromFloat(50000 + float64(i))
		order := CreateTestPendingOrder(PendingOrderTypeBuyLimit, orderID, price)

		err := manager.PlaceOrder(ctx, order)
		require.NoError(t, err)
	}

	// 测试检查执行性能
	kline := CreateTestKlineWithPrices(
		time.Now(),
		decimal.NewFromFloat(50000),
		decimal.NewFromFloat(60000), // 触发大部分挂单
		decimal.NewFromFloat(40000),
		decimal.NewFromFloat(55000),
	)

	start := time.Now()
	results, err := manager.CheckAndExecuteOrders(ctx, kline)
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Greater(t, len(results), 0)
	assert.Less(t, elapsed, 100*time.Millisecond) // 性能要求：100ms内完成

	t.Logf("执行了 %d 个挂单，耗时: %v", len(results), elapsed)
}

func TestLiveOrderManager_GetOrderCount(t *testing.T) {
	liveOrderManager := NewLiveOrderManager(nil)

	count := liveOrderManager.GetOrderCount()
	assert.Equal(t, 0, count)
}

func TestLiveOrderManager_CancelAllOrders(t *testing.T) {
	liveOrderManager := NewLiveOrderManager(nil)

	ctx := context.Background()
	err := liveOrderManager.CancelAllOrders(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented")
}

func TestLiveOrderManager_GetPendingOrders(t *testing.T) {
	liveOrderManager := NewLiveOrderManager(nil)

	orders := liveOrderManager.GetPendingOrders()
	assert.Empty(t, orders)
}
