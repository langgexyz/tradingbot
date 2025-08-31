package engine

import (
	"context"
	"testing"
	"time"

	"tradingbot/src/cex"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// BacktestDataFeed 测试
// ============================================================================

func TestBacktestDataFeed_NewBacktestDataFeed(t *testing.T) {
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	klines := CreateTestKlines(5, startTime, 4*time.Hour)

	tests := []struct {
		name           string
		klines         []*cex.KlineData
		expectedTime   time.Time
		expectedIdx    int
		expectedFinish bool
	}{
		{
			name:           "正常K线数据",
			klines:         klines,
			expectedTime:   startTime,
			expectedIdx:    0,
			expectedFinish: false,
		},
		{
			name:           "空K线数据",
			klines:         []*cex.KlineData{},
			expectedTime:   time.Now(), // 会用当前时间
			expectedIdx:    0,
			expectedFinish: false,
		},
		{
			name:           "nil K线数据",
			klines:         nil,
			expectedTime:   time.Now(), // 会用当前时间
			expectedIdx:    0,
			expectedFinish: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			feed := NewBacktestDataFeed(tt.klines)

			assert.NotNil(t, feed)
			assert.Equal(t, tt.expectedIdx, feed.currentIdx)
			assert.Equal(t, tt.expectedFinish, feed.finished)

			if len(tt.klines) > 0 {
				assert.Equal(t, tt.expectedTime, feed.currentTime)
			} else {
				// 对于空数据，时间应该接近当前时间
				assert.WithinDuration(t, time.Now(), feed.currentTime, 1*time.Second)
			}
		})
	}
}

func TestBacktestDataFeed_Start(t *testing.T) {
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	klines := CreateTestKlines(3, startTime, 4*time.Hour)
	feed := NewBacktestDataFeed(klines)

	// 模拟使用后的状态
	feed.currentIdx = 2
	feed.finished = true
	feed.currentTime = time.Now()

	ctx := context.Background()
	err := feed.Start(ctx)

	assert.NoError(t, err)
	assert.Equal(t, 0, feed.currentIdx)
	assert.False(t, feed.finished)
	assert.Equal(t, startTime, feed.currentTime)
}

func TestBacktestDataFeed_GetNext(t *testing.T) {
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	klines := CreateTestKlines(3, startTime, 4*time.Hour)
	feed := NewBacktestDataFeed(klines)

	ctx := context.Background()

	// 启动数据流
	err := feed.Start(ctx)
	require.NoError(t, err)

	// 第一个K线
	kline1, err := feed.GetNext(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, kline1)
	assert.Equal(t, klines[0], kline1)
	assert.Equal(t, 1, feed.currentIdx)
	assert.Equal(t, klines[0].OpenTime, feed.currentTime)
	assert.False(t, feed.finished)

	// 第二个K线
	kline2, err := feed.GetNext(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, kline2)
	assert.Equal(t, klines[1], kline2)
	assert.Equal(t, 2, feed.currentIdx)
	assert.Equal(t, klines[1].OpenTime, feed.currentTime)
	assert.False(t, feed.finished)

	// 第三个K线（最后一个）
	kline3, err := feed.GetNext(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, kline3)
	assert.Equal(t, klines[2], kline3)
	assert.Equal(t, 3, feed.currentIdx)
	assert.Equal(t, klines[2].OpenTime, feed.currentTime)
	assert.True(t, feed.finished)

	// 再次调用应该返回nil（数据流结束）
	kline4, err := feed.GetNext(ctx)
	assert.NoError(t, err)
	assert.Nil(t, kline4)
}

func TestBacktestDataFeed_GetNext_EmptyData(t *testing.T) {
	feed := NewBacktestDataFeed([]*cex.KlineData{})
	ctx := context.Background()

	err := feed.Start(ctx)
	require.NoError(t, err)

	kline, err := feed.GetNext(ctx)
	assert.NoError(t, err)
	assert.Nil(t, kline)
}

func TestBacktestDataFeed_GetNext_AfterFinished(t *testing.T) {
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	klines := CreateTestKlines(1, startTime, 4*time.Hour)
	feed := NewBacktestDataFeed(klines)

	ctx := context.Background()
	err := feed.Start(ctx)
	require.NoError(t, err)

	// 获取唯一的K线
	_, err = feed.GetNext(ctx)
	require.NoError(t, err)
	assert.True(t, feed.finished)

	// 再次调用应该返回nil
	kline, err := feed.GetNext(ctx)
	assert.NoError(t, err)
	assert.Nil(t, kline)
}

func TestBacktestDataFeed_Stop(t *testing.T) {
	klines := CreateTestKlines(5, time.Now(), 4*time.Hour)
	feed := NewBacktestDataFeed(klines)

	err := feed.Stop()
	assert.NoError(t, err)
	assert.True(t, feed.finished)
}

func TestBacktestDataFeed_GetCurrentTime(t *testing.T) {
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	klines := CreateTestKlines(3, startTime, 4*time.Hour)
	feed := NewBacktestDataFeed(klines)

	// 初始时间
	assert.Equal(t, startTime, feed.GetCurrentTime())

	ctx := context.Background()
	err := feed.Start(ctx)
	require.NoError(t, err)

	// 处理第一个K线后时间应该更新
	_, err = feed.GetNext(ctx)
	require.NoError(t, err)
	assert.Equal(t, klines[0].OpenTime, feed.GetCurrentTime())

	// 处理第二个K线后时间再次更新
	_, err = feed.GetNext(ctx)
	require.NoError(t, err)
	assert.Equal(t, klines[1].OpenTime, feed.GetCurrentTime())
}

// ============================================================================
// LiveDataFeed 测试
// ============================================================================

// MockLiveDataCEXClient 专门用于LiveDataFeed测试的CEX客户端mock
type mockLiveDataCEXClient struct {
	klines      []*cex.KlineData
	currentIdx  int
	shouldError bool
	callCount   int
}

func (m *mockLiveDataCEXClient) GetKlines(ctx context.Context, pair cex.TradingPair, interval string, limit int) ([]*cex.KlineData, error) {
	m.callCount++

	if m.shouldError {
		return nil, TestError
	}

	if m.currentIdx >= len(m.klines) {
		return []*cex.KlineData{}, nil // 返回空数据
	}

	result := []*cex.KlineData{m.klines[m.currentIdx]}
	m.currentIdx++
	return result, nil
}

// 实现CEXClient接口的其他必需方法
func (m *mockLiveDataCEXClient) GetKlinesWithTimeRange(ctx context.Context, pair cex.TradingPair, interval string, startTime, endTime time.Time, limit int) ([]*cex.KlineData, error) {
	return nil, nil
}
func (m *mockLiveDataCEXClient) Ping(ctx context.Context) error { return nil }
func (m *mockLiveDataCEXClient) GetName() string                { return "mock" }
func (m *mockLiveDataCEXClient) GetDatabase() interface{}       { return nil }
func (m *mockLiveDataCEXClient) GetTradingFee() float64         { return 0.001 }
func (m *mockLiveDataCEXClient) Buy(ctx context.Context, req cex.BuyOrderRequest) (*cex.OrderResult, error) {
	return nil, nil
}
func (m *mockLiveDataCEXClient) Sell(ctx context.Context, req cex.SellOrderRequest) (*cex.OrderResult, error) {
	return nil, nil
}
func (m *mockLiveDataCEXClient) GetAccount(ctx context.Context) ([]*cex.AccountBalance, error) {
	return nil, nil
}

func TestLiveDataFeed_NewLiveDataFeed(t *testing.T) {
	mockClient := &mockLiveDataCEXClient{}
	pair := cex.TradingPair{Base: "BTC", Quote: "USDT"}
	interval := "4h"
	tickerInterval := 5 * time.Minute

	feed := NewLiveDataFeed(mockClient, pair, interval, tickerInterval)

	assert.NotNil(t, feed)
	assert.Equal(t, mockClient, feed.cexClient)
	assert.Equal(t, pair, feed.tradingPair)
	assert.Equal(t, interval, feed.interval)
	assert.NotNil(t, feed.ticker)
	assert.NotNil(t, feed.stopChan)
	assert.WithinDuration(t, time.Now(), feed.currentTime, 1*time.Second)
}

func TestLiveDataFeed_Start(t *testing.T) {
	mockClient := &mockLiveDataCEXClient{}
	pair := cex.TradingPair{Base: "BTC", Quote: "USDT"}
	feed := NewLiveDataFeed(mockClient, pair, "4h", 5*time.Minute)

	// 设置过去的时间
	pastTime := time.Now().Add(-1 * time.Hour)
	feed.currentTime = pastTime

	ctx := context.Background()
	err := feed.Start(ctx)

	assert.NoError(t, err)
	assert.WithinDuration(t, time.Now(), feed.currentTime, 1*time.Second)
}

func TestLiveDataFeed_GetNext_Success(t *testing.T) {
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	mockKlines := CreateTestKlines(2, startTime, 4*time.Hour)

	mockClient := &mockLiveDataCEXClient{
		klines: mockKlines,
	}

	pair := cex.TradingPair{Base: "BTC", Quote: "USDT"}
	feed := NewLiveDataFeed(mockClient, pair, "4h", 10*time.Millisecond) // 快速ticker用于测试

	ctx := context.Background()
	err := feed.Start(ctx)
	require.NoError(t, err)
	defer feed.Stop()

	// 第一次调用应该返回第一个K线
	kline1, err := feed.GetNext(ctx)
	assert.NoError(t, err)
	assert.Equal(t, mockKlines[0], kline1)
	assert.Equal(t, 1, mockClient.callCount)

	// 第二次调用应该返回第二个K线
	kline2, err := feed.GetNext(ctx)
	assert.NoError(t, err)
	assert.Equal(t, mockKlines[1], kline2)
	assert.Equal(t, 2, mockClient.callCount)

	// 第三次调用应该返回nil（无更多数据）
	kline3, err := feed.GetNext(ctx)
	assert.NoError(t, err)
	assert.Nil(t, kline3)
}

func TestLiveDataFeed_GetNext_CEXError(t *testing.T) {
	mockClient := &mockLiveDataCEXClient{
		shouldError: true,
	}

	pair := cex.TradingPair{Base: "BTC", Quote: "USDT"}
	feed := NewLiveDataFeed(mockClient, pair, "4h", 10*time.Millisecond)

	ctx := context.Background()
	err := feed.Start(ctx)
	require.NoError(t, err)
	defer feed.Stop()

	kline, err := feed.GetNext(ctx)
	assert.Error(t, err)
	assert.Nil(t, kline)
}

func TestLiveDataFeed_GetNext_ContextCancelled(t *testing.T) {
	mockClient := &mockLiveDataCEXClient{}
	pair := cex.TradingPair{Base: "BTC", Quote: "USDT"}
	feed := NewLiveDataFeed(mockClient, pair, "4h", 1*time.Hour) // 长间隔

	ctx, cancel := context.WithCancel(context.Background())
	err := feed.Start(ctx)
	require.NoError(t, err)
	defer feed.Stop()

	// 立即取消context
	cancel()

	kline, err := feed.GetNext(ctx)
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
	assert.Nil(t, kline)
}

func TestLiveDataFeed_GetNext_Stopped(t *testing.T) {
	mockClient := &mockLiveDataCEXClient{}
	pair := cex.TradingPair{Base: "BTC", Quote: "USDT"}
	feed := NewLiveDataFeed(mockClient, pair, "4h", 10*time.Millisecond)

	ctx := context.Background()
	err := feed.Start(ctx)
	require.NoError(t, err)

	// 立即停止
	err = feed.Stop()
	require.NoError(t, err)

	kline, err := feed.GetNext(ctx)
	assert.NoError(t, err)
	assert.Nil(t, kline)
}

func TestLiveDataFeed_Stop(t *testing.T) {
	mockClient := &mockLiveDataCEXClient{}
	pair := cex.TradingPair{Base: "BTC", Quote: "USDT"}
	feed := NewLiveDataFeed(mockClient, pair, "4h", 10*time.Millisecond)

	err := feed.Stop()
	assert.NoError(t, err)

	// 验证ticker已停止（通过尝试从stopChan读取来验证已关闭）
	select {
	case <-feed.stopChan:
		// 正常，stopChan已关闭
	case <-time.After(100 * time.Millisecond):
		t.Error("stopChan should be closed")
	}
}

func TestLiveDataFeed_GetCurrentTime(t *testing.T) {
	mockClient := &mockLiveDataCEXClient{}
	pair := cex.TradingPair{Base: "BTC", Quote: "USDT"}
	feed := NewLiveDataFeed(mockClient, pair, "4h", 5*time.Minute)

	// 初始时间应该接近当前时间
	assert.WithinDuration(t, time.Now(), feed.GetCurrentTime(), 1*time.Second)

	// Start后时间应该更新
	ctx := context.Background()
	err := feed.Start(ctx)
	require.NoError(t, err)
	defer feed.Stop()

	assert.WithinDuration(t, time.Now(), feed.GetCurrentTime(), 1*time.Second)
}

// ============================================================================
// 数据流完整性测试
// ============================================================================

func TestDataFeed_CompleteFlow(t *testing.T) {
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	klines := CreateTestKlines(10, startTime, 4*time.Hour)

	tests := []struct {
		name     string
		dataFeed DataFeed
	}{
		{
			name:     "BacktestDataFeed完整流程",
			dataFeed: NewBacktestDataFeed(klines),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// 1. 启动数据流
			err := tt.dataFeed.Start(ctx)
			require.NoError(t, err)

			// 2. 逐个获取所有K线
			receivedKlines := []*cex.KlineData{}
			for {
				kline, err := tt.dataFeed.GetNext(ctx)
				require.NoError(t, err)

				if kline == nil {
					break // 数据流结束
				}

				receivedKlines = append(receivedKlines, kline)
			}

			// 3. 验证接收到的数据
			assert.Len(t, receivedKlines, len(klines))
			for i, kline := range receivedKlines {
				assert.Equal(t, klines[i], kline)
			}

			// 4. 停止数据流
			err = tt.dataFeed.Stop()
			assert.NoError(t, err)

			// 5. 停止后再次调用GetNext应该返回nil
			kline, err := tt.dataFeed.GetNext(ctx)
			assert.NoError(t, err)
			assert.Nil(t, kline)
		})
	}
}

// ============================================================================
// 边界情况和压力测试
// ============================================================================

func TestBacktestDataFeed_EdgeCases(t *testing.T) {
	t.Run("大量K线数据", func(t *testing.T) {
		startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		klines := CreateTestKlines(10000, startTime, 1*time.Minute)
		feed := NewBacktestDataFeed(klines)

		ctx := context.Background()
		err := feed.Start(ctx)
		require.NoError(t, err)

		count := 0
		for {
			kline, err := feed.GetNext(ctx)
			require.NoError(t, err)

			if kline == nil {
				break
			}
			count++
		}

		assert.Equal(t, 10000, count)
	})

	t.Run("重复Start调用", func(t *testing.T) {
		klines := CreateTestKlines(3, time.Now(), 4*time.Hour)
		feed := NewBacktestDataFeed(klines)

		ctx := context.Background()

		// 第一次Start
		err := feed.Start(ctx)
		require.NoError(t, err)

		// 获取第一个K线
		_, err = feed.GetNext(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, feed.currentIdx)

		// 第二次Start应该重置状态
		err = feed.Start(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, feed.currentIdx)
		assert.False(t, feed.finished)
	})

	t.Run("重复Stop调用", func(t *testing.T) {
		klines := CreateTestKlines(3, time.Now(), 4*time.Hour)
		feed := NewBacktestDataFeed(klines)

		// 第一次Stop
		err := feed.Stop()
		assert.NoError(t, err)
		assert.True(t, feed.finished)

		// 第二次Stop应该不出错
		err = feed.Stop()
		assert.NoError(t, err)
		assert.True(t, feed.finished)
	})
}

func TestLiveDataFeed_EdgeCases(t *testing.T) {
	t.Run("重复Stop调用", func(t *testing.T) {
		mockClient := &mockLiveDataCEXClient{}
		pair := cex.TradingPair{Base: "BTC", Quote: "USDT"}
		feed := NewLiveDataFeed(mockClient, pair, "4h", 10*time.Millisecond)

		// 第一次Stop
		err := feed.Stop()
		assert.NoError(t, err)

		// 第二次Stop应该不panic
		assert.NotPanics(t, func() {
			err = feed.Stop()
			assert.NoError(t, err)
		})
	})
}
