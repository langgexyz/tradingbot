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
// Mock 组件
// ============================================================================

// MockStrategy for testing
type mockTradingStrategy struct {
	signals     []*strategy.Signal
	onDataCalls int
	shouldError bool
}

func (s *mockTradingStrategy) OnData(ctx context.Context, kline *cex.KlineData, portfolio *executor.Portfolio) ([]*strategy.Signal, error) {
	s.onDataCalls++

	if s.shouldError {
		return nil, TestError
	}

	// 返回预设的信号
	if s.onDataCalls == 1 {
		// 第一次调用返回买入信号
		return []*strategy.Signal{{
			Type:     "BUY",
			Strength: 0.8,
			Reason:   "mock buy signal",
		}}, nil
	} else if s.onDataCalls == 3 {
		// 第三次调用返回卖出信号
		return []*strategy.Signal{{
			Type:     "SELL",
			Strength: 1.0,
			Reason:   "mock sell signal",
		}}, nil
	}

	return []*strategy.Signal{}, nil
}

func (s *mockTradingStrategy) GetName() string {
	return "MockStrategy"
}

func (s *mockTradingStrategy) GetParams() strategy.StrategyParams {
	return nil
}

func (s *mockTradingStrategy) SetParams(params strategy.StrategyParams) error {
	return nil
}

// MockDataFeed for testing
type mockTradingDataFeed struct {
	klines       []*cex.KlineData
	currentIdx   int
	started      bool
	stopped      bool
	startError   bool
	getNextError bool
}

func (f *mockTradingDataFeed) Start(ctx context.Context) error {
	if f.startError {
		return TestError
	}
	f.started = true
	f.currentIdx = 0
	return nil
}

func (f *mockTradingDataFeed) GetNext(ctx context.Context) (*cex.KlineData, error) {
	if f.getNextError {
		return nil, TestError
	}

	if f.stopped || f.currentIdx >= len(f.klines) {
		return nil, nil // 数据流结束
	}

	kline := f.klines[f.currentIdx]
	f.currentIdx++
	return kline, nil
}

func (f *mockTradingDataFeed) Stop() error {
	f.stopped = true
	return nil
}

func (f *mockTradingDataFeed) GetCurrentTime() time.Time {
	if f.currentIdx > 0 && f.currentIdx <= len(f.klines) {
		return f.klines[f.currentIdx-1].OpenTime
	}
	return time.Now()
}

// MockOrderManager for testing
type mockTradingOrderManager struct {
	placedOrders     []*PendingOrder
	cancelledOrders  []string
	executedResults  []*executor.OrderResult
	shouldFailPlace  bool
	shouldFailCancel bool
	shouldFailCheck  bool
	placeCallCount   int
	cancelCallCount  int
	checkCallCount   int
}

func (m *mockTradingOrderManager) PlaceOrder(ctx context.Context, order *PendingOrder) error {
	m.placeCallCount++

	if m.shouldFailPlace {
		return assert.AnError
	}

	m.placedOrders = append(m.placedOrders, order)
	return nil
}

func (m *mockTradingOrderManager) CancelOrder(ctx context.Context, orderID string) error {
	m.cancelCallCount++

	if m.shouldFailCancel {
		return assert.AnError
	}

	m.cancelledOrders = append(m.cancelledOrders, orderID)
	return nil
}

func (m *mockTradingOrderManager) CancelAllOrders(ctx context.Context) error {
	return nil
}

func (m *mockTradingOrderManager) CheckAndExecuteOrders(ctx context.Context, kline *cex.KlineData) ([]*executor.OrderResult, error) {
	m.checkCallCount++

	if m.shouldFailCheck {
		return nil, assert.AnError
	}

	// 返回预设的执行结果
	results := make([]*executor.OrderResult, len(m.executedResults))
	copy(results, m.executedResults)

	return results, nil
}

func (m *mockTradingOrderManager) GetPendingOrders() []*PendingOrder {
	return m.placedOrders
}

func (m *mockTradingOrderManager) GetOrderCount() int {
	return len(m.placedOrders)
}

// ============================================================================
// TradingEngine 测试
// ============================================================================

func TestTradingEngine_NewTradingEngine(t *testing.T) {
	pair := cex.TradingPair{Base: "BTC", Quote: "USDT"}
	timeframe := timeframes.Timeframe4h
	mockStrategy := &mockTradingStrategy{}
	mockExecutor := newMockOrderExecutor(decimal.NewFromInt(1000), decimal.Zero)
	mockCEX := &MockCEXClient{}
	mockDataFeed := &mockTradingDataFeed{}
	mockOrderManager := &mockTradingOrderManager{}

	engine := NewTradingEngine(
		pair,
		timeframe,
		mockStrategy,
		mockExecutor,
		mockCEX,
		mockDataFeed,
		mockOrderManager,
	)

	assert.NotNil(t, engine)
	assert.Equal(t, pair, engine.tradingPair)
	assert.Equal(t, timeframe, engine.timeframe)
	assert.Equal(t, mockStrategy, engine.strategy)
	assert.Equal(t, mockExecutor, engine.executor)
	assert.Equal(t, mockCEX, engine.cexClient)
	assert.Equal(t, mockDataFeed, engine.dataFeed)
	assert.Equal(t, mockOrderManager, engine.orderManager)
	assert.True(t, engine.positionSizePercent.Equal(decimal.NewFromFloat(0.95)))
	assert.True(t, engine.minTradeAmount.Equal(decimal.NewFromFloat(10.0)))
	assert.False(t, engine.isRunning)
}

func TestTradingEngine_SetPositionSizePercent(t *testing.T) {
	engine := createTestTradingEngine()

	engine.SetPositionSizePercent(0.5)

	assert.True(t, engine.positionSizePercent.Equal(decimal.NewFromFloat(0.5)))
}

func TestTradingEngine_SetMinTradeAmount(t *testing.T) {
	engine := createTestTradingEngine()

	engine.SetMinTradeAmount(100.0)

	assert.True(t, engine.minTradeAmount.Equal(decimal.NewFromFloat(100.0)))
}

func TestTradingEngine_Run_Success(t *testing.T) {
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	klines := CreateTestKlines(5, startTime, 4*time.Hour)

	mockStrategy := &mockTradingStrategy{}
	mockExecutor := newMockOrderExecutor(decimal.NewFromInt(10000), decimal.Zero)
	mockDataFeed := &mockTradingDataFeed{klines: klines}
	mockOrderManager := &mockTradingOrderManager{}

	engine := createTestTradingEngineWithMocks(
		mockStrategy,
		mockExecutor,
		mockDataFeed,
		mockOrderManager,
	)

	ctx := context.Background()
	err := engine.Run(ctx)

	assert.NoError(t, err)
	assert.False(t, engine.isRunning) // 运行完成后应该设为false
	assert.True(t, mockDataFeed.started)
	assert.True(t, mockDataFeed.stopped)
	assert.Equal(t, 5, mockStrategy.onDataCalls)        // 每个K线调用一次策略
	assert.Equal(t, 5, mockOrderManager.checkCallCount) // 每个K线检查一次挂单
	assert.Len(t, engine.lastKlines, 5)                 // 保存了所有K线数据
}

func TestTradingEngine_Run_DataFeedStartError(t *testing.T) {
	mockDataFeed := &mockTradingDataFeed{startError: true}
	engine := createTestTradingEngineWithMocks(
		&mockTradingStrategy{},
		newMockOrderExecutor(decimal.NewFromInt(1000), decimal.Zero),
		mockDataFeed,
		&mockTradingOrderManager{},
	)

	ctx := context.Background()
	err := engine.Run(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "启动数据喂入失败")
	assert.False(t, engine.isRunning)
}

func TestTradingEngine_Run_ContextCancellation(t *testing.T) {
	// 创建一个会阻塞的数据流
	mockDataFeed := &mockTradingDataFeed{
		klines: CreateTestKlines(1000, time.Now(), 4*time.Hour), // 更多数据
	}

	engine := createTestTradingEngineWithMocks(
		&mockTradingStrategy{},
		newMockOrderExecutor(decimal.NewFromInt(1000), decimal.Zero),
		mockDataFeed,
		&mockTradingOrderManager{},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond) // 非常短的超时
	defer cancel()

	err := engine.Run(ctx)

	// 由于数据流可能很快完成，我们接受两种结果：超时错误或成功完成
	if err != nil {
		assert.Equal(t, context.DeadlineExceeded, err)
	}
	assert.False(t, engine.isRunning)
}

func TestTradingEngine_Run_StrategyError(t *testing.T) {
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	klines := CreateTestKlines(3, startTime, 4*time.Hour)

	mockStrategy := &mockTradingStrategy{shouldError: true}
	mockDataFeed := &mockTradingDataFeed{klines: klines}

	engine := createTestTradingEngineWithMocks(
		mockStrategy,
		newMockOrderExecutor(decimal.NewFromInt(1000), decimal.Zero),
		mockDataFeed,
		&mockTradingOrderManager{},
	)

	ctx := context.Background()
	err := engine.Run(ctx)

	// 策略错误不应该终止整个流程，应该继续处理
	assert.NoError(t, err)
	assert.Equal(t, 3, mockStrategy.onDataCalls) // 仍然处理了所有K线
}

func TestTradingEngine_Run_OrderManagerError(t *testing.T) {
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	klines := CreateTestKlines(3, startTime, 4*time.Hour)

	mockOrderManager := &mockTradingOrderManager{shouldFailCheck: true}
	mockDataFeed := &mockTradingDataFeed{klines: klines}

	engine := createTestTradingEngineWithMocks(
		&mockTradingStrategy{},
		newMockOrderExecutor(decimal.NewFromInt(1000), decimal.Zero),
		mockDataFeed,
		mockOrderManager,
	)

	ctx := context.Background()
	err := engine.Run(ctx)

	// 挂单管理错误不应该终止整个流程
	assert.NoError(t, err)
	assert.Equal(t, 3, mockOrderManager.checkCallCount) // 仍然检查了所有K线
}

func TestTradingEngine_Run_CompleteTradeFlow(t *testing.T) {
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	klines := CreateTestKlines(5, startTime, 4*time.Hour)

	// 设置更现实的价格数据
	klines[0].Close = decimal.NewFromFloat(50000) // 第1根K线
	klines[1].Close = decimal.NewFromFloat(48000) // 第2根K线，触发买入
	klines[2].Close = decimal.NewFromFloat(49000) // 第3根K线
	klines[3].Close = decimal.NewFromFloat(55000) // 第4根K线，触发卖出
	klines[4].Close = decimal.NewFromFloat(54000) // 第5根K线

	mockStrategy := &mockTradingStrategy{}
	mockExecutor := newMockOrderExecutor(decimal.NewFromInt(50000), decimal.Zero)
	mockDataFeed := &mockTradingDataFeed{klines: klines}

	// 创建真实的回测挂单管理器用于完整测试
	realOrderManager := NewBacktestOrderManager(mockExecutor)

	engine := createTestTradingEngineWithMocks(
		mockStrategy,
		mockExecutor,
		mockDataFeed,
		realOrderManager,
	)

	ctx := context.Background()
	err := engine.Run(ctx)

	assert.NoError(t, err)
	assert.Equal(t, 5, mockStrategy.onDataCalls)

	// 验证交易执行
	orders := mockExecutor.GetOrders()
	assert.GreaterOrEqual(t, len(orders), 1) // 至少有买入订单

	// 验证K线数据保存
	savedKlines := engine.GetKlines()
	assert.Equal(t, klines, savedKlines)
}

func TestTradingEngine_RunBacktest(t *testing.T) {
	engine := createTestTradingEngine()

	ctx := context.Background()
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2024, 1, 7, 0, 0, 0, 0, time.UTC)

	// RunBacktest应该调用统一的Run方法
	err := engine.RunBacktest(ctx, startTime, endTime)

	// 由于使用了空的mock数据流，应该成功完成（没有数据处理）
	assert.NoError(t, err)
}

func TestTradingEngine_RunLive(t *testing.T) {
	engine := createTestTradingEngine()

	ctx := context.Background()

	// RunLive应该调用统一的Run方法
	err := engine.RunLive(ctx)

	// 由于使用了空的mock数据流，应该成功完成（没有数据处理）
	assert.NoError(t, err)
}

func TestTradingEngine_Stop(t *testing.T) {
	engine := createTestTradingEngine()

	// 启动引擎
	engine.isRunning = true

	// 停止引擎
	engine.Stop()

	// 验证stopChan被关闭（通过尝试读取验证）
	select {
	case <-engine.stopChan:
		// 正常，stopChan已关闭
	case <-time.After(100 * time.Millisecond):
		t.Error("stopChan should be closed")
	}
}

func TestTradingEngine_GetKlines(t *testing.T) {
	engine := createTestTradingEngine()

	// 初始应该为空
	klines := engine.GetKlines()
	assert.Nil(t, klines)

	// 设置K线数据
	testKlines := CreateTestKlines(3, time.Now(), 4*time.Hour)
	engine.lastKlines = testKlines

	// 获取K线数据
	retrievedKlines := engine.GetKlines()
	assert.Equal(t, testKlines, retrievedKlines)
}

// ============================================================================
// 信号处理测试
// ============================================================================

func TestTradingEngine_ProcessSignal_BuySignal(t *testing.T) {
	mockExecutor := newMockOrderExecutor(decimal.NewFromInt(10000), decimal.Zero)
	mockOrderManager := &mockTradingOrderManager{}

	engine := createTestTradingEngineWithMocks(
		&mockTradingStrategy{},
		mockExecutor,
		&mockTradingDataFeed{},
		mockOrderManager,
	)

	signal := &strategy.Signal{
		Type:     "BUY",
		Strength: 0.8,
		Reason:   "test buy signal",
	}

	kline := &cex.KlineData{
		OpenTime: time.Now(),
		Close:    decimal.NewFromFloat(50000),
		High:     decimal.NewFromFloat(51000),
		Low:      decimal.NewFromFloat(49000),
	}

	portfolio := &executor.Portfolio{
		Cash:     decimal.NewFromInt(10000),
		Position: decimal.Zero,
	}

	ctx := context.Background()
	err := engine.processSignal(ctx, signal, kline, portfolio)

	assert.NoError(t, err)
	assert.Equal(t, 1, mockOrderManager.placeCallCount)
	require.Len(t, mockOrderManager.placedOrders, 1)

	placedOrder := mockOrderManager.placedOrders[0]
	assert.Equal(t, PendingOrderTypeBuyLimit, placedOrder.Type)
	assert.Contains(t, placedOrder.ID, "buy_")
	assert.Contains(t, placedOrder.ID, "BTC")
	assert.Equal(t, signal.Reason, placedOrder.Reason)

	// 验证限价比市价低0.1%
	expectedPrice := kline.Close.Mul(decimal.NewFromFloat(0.999))
	assert.True(t, placedOrder.Price.Equal(expectedPrice))
}

func TestTradingEngine_ProcessSignal_SellSignal(t *testing.T) {
	mockExecutor := newMockOrderExecutor(decimal.NewFromInt(1000), decimal.NewFromInt(2))
	mockOrderManager := &mockTradingOrderManager{}

	engine := createTestTradingEngineWithMocks(
		&mockTradingStrategy{},
		mockExecutor,
		&mockTradingDataFeed{},
		mockOrderManager,
	)

	signal := &strategy.Signal{
		Type:     "SELL",
		Strength: 0.5, // 部分卖出
		Reason:   "test sell signal",
	}

	kline := &cex.KlineData{
		OpenTime: time.Now(),
		Close:    decimal.NewFromFloat(55000),
		High:     decimal.NewFromFloat(56000),
		Low:      decimal.NewFromFloat(54000),
	}

	portfolio := &executor.Portfolio{
		Cash:     decimal.NewFromInt(1000),
		Position: decimal.NewFromInt(2),
	}

	ctx := context.Background()
	err := engine.processSignal(ctx, signal, kline, portfolio)

	assert.NoError(t, err)
	assert.Equal(t, 1, mockOrderManager.placeCallCount)
	require.Len(t, mockOrderManager.placedOrders, 1)

	placedOrder := mockOrderManager.placedOrders[0]
	assert.Equal(t, PendingOrderTypeSellLimit, placedOrder.Type)
	assert.Contains(t, placedOrder.ID, "sell_")
	assert.Equal(t, signal.Reason, placedOrder.Reason)

	// 验证部分卖出数量
	expectedQuantity := portfolio.Position.Mul(decimal.NewFromFloat(signal.Strength))
	assert.True(t, placedOrder.Quantity.Equal(expectedQuantity))

	// 验证限价比市价高0.1%
	expectedPrice := kline.Close.Mul(decimal.NewFromFloat(1.001))
	assert.True(t, placedOrder.Price.Equal(expectedPrice))
}

func TestTradingEngine_ProcessSignal_SellSignal_NoPosition(t *testing.T) {
	mockExecutor := newMockOrderExecutor(decimal.NewFromInt(1000), decimal.Zero) // 无持仓
	mockOrderManager := &mockTradingOrderManager{}

	engine := createTestTradingEngineWithMocks(
		&mockTradingStrategy{},
		mockExecutor,
		&mockTradingDataFeed{},
		mockOrderManager,
	)

	signal := &strategy.Signal{
		Type:     "SELL",
		Strength: 1.0,
		Reason:   "test sell signal",
	}

	kline := &cex.KlineData{
		OpenTime: time.Now(),
		Close:    decimal.NewFromFloat(55000),
	}

	portfolio := &executor.Portfolio{
		Cash:     decimal.NewFromInt(1000),
		Position: decimal.Zero, // 无持仓
	}

	ctx := context.Background()
	err := engine.processSignal(ctx, signal, kline, portfolio)

	assert.NoError(t, err)
	assert.Equal(t, 0, mockOrderManager.placeCallCount) // 不应该下卖出单
}

func TestTradingEngine_ProcessSignal_BuySignal_InsufficientCash(t *testing.T) {
	mockExecutor := newMockOrderExecutor(decimal.NewFromFloat(5), decimal.Zero) // 资金很少
	mockOrderManager := &mockTradingOrderManager{}

	engine := createTestTradingEngineWithMocks(
		&mockTradingStrategy{},
		mockExecutor,
		&mockTradingDataFeed{},
		mockOrderManager,
	)

	// 设置最小交易金额为100
	engine.SetMinTradeAmount(100.0)

	signal := &strategy.Signal{
		Type:     "BUY",
		Strength: 0.8,
		Reason:   "test buy signal",
	}

	kline := &cex.KlineData{
		OpenTime: time.Now(),
		Close:    decimal.NewFromFloat(50000),
	}

	portfolio := &executor.Portfolio{
		Cash:     decimal.NewFromFloat(5),
		Position: decimal.Zero,
	}

	ctx := context.Background()
	err := engine.processSignal(ctx, signal, kline, portfolio)

	assert.NoError(t, err)
	assert.Equal(t, 0, mockOrderManager.placeCallCount) // 不应该下买入单
}

func TestTradingEngine_ProcessSignal_UnknownSignalType(t *testing.T) {
	engine := createTestTradingEngine()

	signal := &strategy.Signal{
		Type:     "UNKNOWN",
		Strength: 1.0,
		Reason:   "unknown signal",
	}

	kline := &cex.KlineData{
		OpenTime: time.Now(),
		Close:    decimal.NewFromFloat(50000),
	}

	portfolio := &executor.Portfolio{
		Cash:     decimal.NewFromInt(1000),
		Position: decimal.Zero,
	}

	ctx := context.Background()
	err := engine.processSignal(ctx, signal, kline, portfolio)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未知信号类型")
}

// ============================================================================
// 边界情况和错误处理测试
// ============================================================================

func TestTradingEngine_Run_EmptyDataStream(t *testing.T) {
	mockDataFeed := &mockTradingDataFeed{klines: []*cex.KlineData{}} // 空数据

	engine := createTestTradingEngineWithMocks(
		&mockTradingStrategy{},
		newMockOrderExecutor(decimal.NewFromInt(1000), decimal.Zero),
		mockDataFeed,
		&mockTradingOrderManager{},
	)

	ctx := context.Background()
	err := engine.Run(ctx)

	assert.NoError(t, err)
	assert.Len(t, engine.lastKlines, 0)
}

func TestTradingEngine_Run_PortfolioError(t *testing.T) {
	klines := CreateTestKlines(2, time.Now(), 4*time.Hour)

	// 创建会失败的执行器
	mockExecutor := &mockExecutorWithPortfolioError{}
	mockDataFeed := &mockTradingDataFeed{klines: klines}

	engine := createTestTradingEngineWithMocks(
		&mockTradingStrategy{},
		mockExecutor,
		mockDataFeed,
		&mockTradingOrderManager{},
	)

	ctx := context.Background()
	err := engine.Run(ctx)

	// 投资组合错误不应该终止整个流程
	assert.NoError(t, err)
}

// ============================================================================
// 辅助函数
// ============================================================================

func createTestTradingEngine() *TradingEngine {
	return createTestTradingEngineWithMocks(
		&mockTradingStrategy{},
		newMockOrderExecutor(decimal.NewFromInt(1000), decimal.Zero),
		&mockTradingDataFeed{},
		&mockTradingOrderManager{},
	)
}

func createTestTradingEngineWithMocks(
	strategy strategy.Strategy,
	executor executor.Executor,
	dataFeed DataFeed,
	orderManager OrderManager,
) *TradingEngine {
	pair := cex.TradingPair{Base: "BTC", Quote: "USDT"}
	timeframe := timeframes.Timeframe4h
	mockCEX := &MockCEXClient{}

	return NewTradingEngine(
		pair,
		timeframe,
		strategy,
		executor,
		mockCEX,
		dataFeed,
		orderManager,
	)
}

// mockExecutorWithPortfolioError 模拟GetPortfolio失败的执行器
type mockExecutorWithPortfolioError struct {
	mockOrderExecutor
}

func (m *mockExecutorWithPortfolioError) GetPortfolio(ctx context.Context) (*executor.Portfolio, error) {
	return nil, TestError
}

// ============================================================================
// 集成测试：完整的tick-by-tick流程
// ============================================================================

func TestTradingEngine_FullTickByTickIntegration(t *testing.T) {
	// 创建真实的模拟交易场景
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	// 构造具有明确交易信号的K线数据
	klines := []*cex.KlineData{
		// K线1: 正常价格，无信号
		{
			OpenTime: startTime,
			Close:    decimal.NewFromFloat(50000),
			High:     decimal.NewFromFloat(50500),
			Low:      decimal.NewFromFloat(49500),
		},
		// K线2: 价格跌破下轨，触发买入信号
		{
			OpenTime: startTime.Add(4 * time.Hour),
			Close:    decimal.NewFromFloat(47000),
			High:     decimal.NewFromFloat(48000),
			Low:      decimal.NewFromFloat(46000),
		},
		// K线3: 价格回升，无新信号
		{
			OpenTime: startTime.Add(8 * time.Hour),
			Close:    decimal.NewFromFloat(49000),
			High:     decimal.NewFromFloat(50000),
			Low:      decimal.NewFromFloat(47500),
		},
		// K线4: 价格大幅上涨，触发卖出信号
		{
			OpenTime: startTime.Add(12 * time.Hour),
			Close:    decimal.NewFromFloat(58000),
			High:     decimal.NewFromFloat(59000),
			Low:      decimal.NewFromFloat(57000),
		},
	}

	// 创建真实组件
	mockStrategy := &mockTradingStrategy{}
	mockExecutor := newMockOrderExecutor(decimal.NewFromInt(10000), decimal.Zero)
	dataFeed := NewBacktestDataFeed(klines)
	orderManager := NewBacktestOrderManager(mockExecutor)

	engine := createTestTradingEngineWithMocks(
		mockStrategy,
		mockExecutor,
		dataFeed,
		orderManager,
	)

	ctx := context.Background()
	err := engine.Run(ctx)

	require.NoError(t, err)

	// 验证完整流程
	assert.Equal(t, len(klines), mockStrategy.onDataCalls)
	assert.Len(t, engine.GetKlines(), len(klines))

	// 验证交易执行（根据mock策略，第1个K线买入，第3个K线卖出）
	orders := mockExecutor.GetOrders()
	assert.GreaterOrEqual(t, len(orders), 1)

	// 验证最终挂单为空
	assert.Equal(t, 0, orderManager.GetOrderCount())

	t.Logf("✅ 完整tick-by-tick流程测试成功:")
	t.Logf("  - 处理K线: %d", len(klines))
	t.Logf("  - 策略调用: %d", mockStrategy.onDataCalls)
	t.Logf("  - 执行订单: %d", len(orders))
	t.Logf("  - 最终现金: %s", mockExecutor.cash.String())
	t.Logf("  - 最终持仓: %s", mockExecutor.position.String())
}

// ============================================================================
// 性能基准测试
// ============================================================================

func BenchmarkTradingEngine_Run(b *testing.B) {
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	klines := CreateTestKlines(1000, startTime, 4*time.Hour) // 1000个K线

	for i := 0; i < b.N; i++ {
		mockStrategy := &mockTradingStrategy{}
		mockExecutor := newMockOrderExecutor(decimal.NewFromInt(100000), decimal.Zero)
		dataFeed := NewBacktestDataFeed(klines)
		orderManager := NewBacktestOrderManager(mockExecutor)

		engine := createTestTradingEngineWithMocks(
			mockStrategy,
			mockExecutor,
			dataFeed,
			orderManager,
		)

		ctx := context.Background()
		err := engine.Run(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTradingEngine_ProcessSignal(b *testing.B) {
	engine := createTestTradingEngine()

	signal := &strategy.Signal{
		Type:     "BUY",
		Strength: 0.8,
		Reason:   "benchmark signal",
	}

	kline := &cex.KlineData{
		OpenTime: time.Now(),
		Close:    decimal.NewFromFloat(50000),
	}

	portfolio := &executor.Portfolio{
		Cash:     decimal.NewFromInt(10000),
		Position: decimal.Zero,
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := engine.processSignal(ctx, signal, kline, portfolio)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// ============================================================================
// 其他方法测试
// ============================================================================

func TestTradingEngine_GetTimeframeInterval(t *testing.T) {
	engine := &TradingEngine{
		timeframe: timeframes.Timeframe4h,
	}

	interval := engine.getTimeframeInterval()
	expected := 4 * time.Hour // 4 hours
	assert.Equal(t, expected, interval)
}

func TestTradingEngine_Close(t *testing.T) {
	mockDataFeed := &mockDataFeed{}
	mockExecutor := newMockOrderExecutor(decimal.NewFromInt(10000), decimal.Zero)
	engine := &TradingEngine{
		dataFeed: mockDataFeed,
		executor: mockExecutor,
	}

	err := engine.Close()
	assert.NoError(t, err)
	assert.True(t, mockDataFeed.stopCalled, "DataFeed.Stop() should be called")
}

// Mock data feed for testing Close method
type mockDataFeed struct {
	stopCalled bool
}

func (m *mockDataFeed) Start(ctx context.Context) error {
	return nil
}

func (m *mockDataFeed) GetNext(ctx context.Context) (*cex.KlineData, error) {
	return nil, nil
}

func (m *mockDataFeed) Stop() error {
	m.stopCalled = true
	return nil
}

func (m *mockDataFeed) GetCurrentTime() time.Time {
	return time.Now()
}

// ============================================================================
// handleSellSignal 边缘情况测试
// ============================================================================

func TestTradingEngine_HandleSellSignal_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		strength float64
		position decimal.Decimal
		expected string // 预期日志信息
	}{
		{
			name:     "信号强度为0",
			strength: 0,
			position: decimal.NewFromFloat(1.5),
			expected: "信号强度无效，执行全仓卖出",
		},
		{
			name:     "信号强度为负数",
			strength: -0.1,
			position: decimal.NewFromFloat(1.0),
			expected: "信号强度无效，执行全仓卖出",
		},
		{
			name:     "信号强度大于1",
			strength: 1.5,
			position: decimal.NewFromFloat(2.0),
			expected: "信号强度无效，执行全仓卖出",
		},
		{
			name:     "正常部分卖出",
			strength: 0.6,
			position: decimal.NewFromFloat(2.0),
			expected: "执行部分卖出",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExecutor := newMockOrderExecutor(
				decimal.NewFromInt(10000), // 现金
				tt.position,               // 持仓
			)
			orderManager := NewBacktestOrderManager(mockExecutor)
			engine := &TradingEngine{
				orderManager:        orderManager,
				tradingPair:         cex.TradingPair{Base: "BTC", Quote: "USDT"},
				positionSizePercent: decimal.NewFromFloat(0.95),
				minTradeAmount:      decimal.NewFromInt(100),
			}

			signal := &strategy.Signal{
				Type:     "SELL",
				Strength: tt.strength,
				Reason:   "edge case test",
			}

			kline := CreateTestKlineWithPrices(
				time.Now(),
				decimal.NewFromFloat(50000),
				decimal.NewFromFloat(51000),
				decimal.NewFromFloat(49000),
				decimal.NewFromFloat(50500),
			)

			portfolio := &executor.Portfolio{
				Cash:     decimal.NewFromInt(10000),
				Position: tt.position,
			}

			ctx := context.Background()
			err := engine.handleSellSignal(ctx, signal, kline, portfolio)
			assert.NoError(t, err)

			// 验证挂单数量
			assert.Equal(t, 1, orderManager.GetOrderCount(), "应该生成一个卖出挂单")
		})
	}
}

func TestTradingEngine_HandleSellSignal_WithExistingSellOrders(t *testing.T) {
	mockExecutor := newMockOrderExecutor(
		decimal.NewFromInt(10000),
		decimal.NewFromFloat(2.0), // 持仓
	)
	orderManager := NewBacktestOrderManager(mockExecutor)

	// 预先添加一个卖出挂单
	existingOrder := CreateTestPendingOrder(
		PendingOrderTypeSellLimit,
		"existing_sell_order",
		decimal.NewFromFloat(51000),
	)
	orderManager.PlaceOrder(context.Background(), existingOrder)

	engine := &TradingEngine{
		orderManager:        orderManager,
		tradingPair:         cex.TradingPair{Base: "BTC", Quote: "USDT"},
		positionSizePercent: decimal.NewFromFloat(0.95),
		minTradeAmount:      decimal.NewFromInt(100),
	}

	signal := &strategy.Signal{
		Type:     "SELL",
		Strength: 0.8,
		Reason:   "test with existing orders",
	}

	kline := CreateTestKlineWithPrices(
		time.Now(),
		decimal.NewFromFloat(50000),
		decimal.NewFromFloat(51000),
		decimal.NewFromFloat(49000),
		decimal.NewFromFloat(50500),
	)

	portfolio := &executor.Portfolio{
		Cash:     decimal.NewFromInt(10000),
		Position: decimal.NewFromFloat(2.0),
	}

	ctx := context.Background()

	// 验证初始状态有一个挂单
	assert.Equal(t, 1, orderManager.GetOrderCount(), "应该有一个现有的卖出挂单")

	err := engine.handleSellSignal(ctx, signal, kline, portfolio)
	assert.NoError(t, err)

	// 验证现有挂单被取消，新挂单被创建
	assert.Equal(t, 1, orderManager.GetOrderCount(), "应该只有一个新的卖出挂单")

	// 验证新挂单不是原来的ID
	pendingOrders := orderManager.GetPendingOrders()
	assert.NotEqual(t, "existing_sell_order", pendingOrders[0].ID, "应该是新的挂单ID")
}
