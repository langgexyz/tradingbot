package cex

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockCEXClient 实现 CEXClient 接口用于测试
type mockCEXClient struct {
	name string
}

func (m *mockCEXClient) GetName() string {
	return m.name
}

func (m *mockCEXClient) GetDatabase() interface{} {
	return nil
}

func (m *mockCEXClient) GetTradingFee() float64 {
	return 0.001
}

func (m *mockCEXClient) GetKlines(ctx context.Context, pair TradingPair, interval string, limit int) ([]*KlineData, error) {
	return []*KlineData{}, nil
}

func (m *mockCEXClient) GetKlinesWithTimeRange(ctx context.Context, pair TradingPair, interval string, startTime, endTime time.Time, limit int) ([]*KlineData, error) {
	return []*KlineData{}, nil
}

func (m *mockCEXClient) Buy(ctx context.Context, order BuyOrderRequest) (*OrderResult, error) {
	return &OrderResult{
		OrderID:      "mock_buy_123",
		TradingPair:  order.TradingPair,
		Price:        order.Price,
		Quantity:     order.Quantity,
		Side:         OrderSideBuy,
		Status:       "FILLED",
		Type:         order.Type,
		TransactTime: time.Now(),
	}, nil
}

func (m *mockCEXClient) Sell(ctx context.Context, order SellOrderRequest) (*OrderResult, error) {
	return &OrderResult{
		OrderID:      "mock_sell_123",
		TradingPair:  order.TradingPair,
		Price:        order.Price,
		Quantity:     order.Quantity,
		Side:         OrderSideSell,
		Status:       "FILLED",
		Type:         order.Type,
		TransactTime: time.Now(),
	}, nil
}

func (m *mockCEXClient) GetAccount(ctx context.Context) ([]*AccountBalance, error) {
	return []*AccountBalance{
		{Asset: "USDT", Free: decimal.NewFromFloat(1000), Locked: decimal.Zero},
		{Asset: "BTC", Free: decimal.Zero, Locked: decimal.Zero},
	}, nil
}

func (m *mockCEXClient) Ping(ctx context.Context) error {
	return nil
}

// mockCEXFactory 实现 CEXFactory 接口用于测试
type mockCEXFactory struct {
	clientName string
}

func (f *mockCEXFactory) CreateClient() CEXClient {
	return &mockCEXClient{name: f.clientName}
}

func TestRegisterCEXFactory(t *testing.T) {
	// 清空注册表
	CEXFactoryRegistry = make(map[string]CEXFactory)

	factory := &mockCEXFactory{clientName: "test-cex"}

	RegisterCEXFactory("test", factory)

	// 验证工厂已注册
	assert.Contains(t, CEXFactoryRegistry, "test")
	assert.Equal(t, factory, CEXFactoryRegistry["test"])
}

func TestCreateCEXClient_Success(t *testing.T) {
	// 清空注册表并注册测试工厂
	CEXFactoryRegistry = make(map[string]CEXFactory)
	factory := &mockCEXFactory{clientName: "test-cex"}
	RegisterCEXFactory("test", factory)

	client, err := CreateCEXClient("test")

	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, "test-cex", client.GetName())
}

func TestCreateCEXClient_UnsupportedCEX(t *testing.T) {
	// 清空注册表
	CEXFactoryRegistry = make(map[string]CEXFactory)

	client, err := CreateCEXClient("nonexistent")

	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "unsupported CEX: nonexistent")
}

func TestGetSupportedCEXes_Empty(t *testing.T) {
	// 清空注册表
	CEXFactoryRegistry = make(map[string]CEXFactory)

	cexes := GetSupportedCEXes()

	assert.Empty(t, cexes)
}

func TestGetSupportedCEXes_Multiple(t *testing.T) {
	// 清空注册表并注册多个工厂
	CEXFactoryRegistry = make(map[string]CEXFactory)

	factory1 := &mockCEXFactory{clientName: "cex1"}
	factory2 := &mockCEXFactory{clientName: "cex2"}
	factory3 := &mockCEXFactory{clientName: "cex3"}

	RegisterCEXFactory("cex1", factory1)
	RegisterCEXFactory("cex2", factory2)
	RegisterCEXFactory("cex3", factory3)

	cexes := GetSupportedCEXes()

	assert.Len(t, cexes, 3)
	assert.Contains(t, cexes, "cex1")
	assert.Contains(t, cexes, "cex2")
	assert.Contains(t, cexes, "cex3")
}

func TestFactoryIntegration(t *testing.T) {
	// 清空注册表
	CEXFactoryRegistry = make(map[string]CEXFactory)

	// 注册多个不同的工厂
	factory1 := &mockCEXFactory{clientName: "binance"}
	factory2 := &mockCEXFactory{clientName: "coinbase"}

	RegisterCEXFactory("binance", factory1)
	RegisterCEXFactory("coinbase", factory2)

	// 测试创建不同的客户端
	client1, err1 := CreateCEXClient("binance")
	require.NoError(t, err1)
	assert.Equal(t, "binance", client1.GetName())

	client2, err2 := CreateCEXClient("coinbase")
	require.NoError(t, err2)
	assert.Equal(t, "coinbase", client2.GetName())

	// 测试获取支持的CEX列表
	supportedCEXes := GetSupportedCEXes()
	assert.Len(t, supportedCEXes, 2)
	assert.Contains(t, supportedCEXes, "binance")
	assert.Contains(t, supportedCEXes, "coinbase")
}

func TestFactoryRegistry_Overwrite(t *testing.T) {
	// 清空注册表
	CEXFactoryRegistry = make(map[string]CEXFactory)

	factory1 := &mockCEXFactory{clientName: "original"}
	factory2 := &mockCEXFactory{clientName: "updated"}

	// 注册第一个工厂
	RegisterCEXFactory("test", factory1)
	client1, _ := CreateCEXClient("test")
	assert.Equal(t, "original", client1.GetName())

	// 覆盖注册第二个工厂
	RegisterCEXFactory("test", factory2)
	client2, _ := CreateCEXClient("test")
	assert.Equal(t, "updated", client2.GetName())
}

// 测试边界情况和错误处理
func TestFactoryEdgeCases(t *testing.T) {
	t.Run("empty cex name", func(t *testing.T) {
		CEXFactoryRegistry = make(map[string]CEXFactory)

		client, err := CreateCEXClient("")
		assert.Error(t, err)
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "unsupported CEX: ")
	})

	t.Run("nil factory registration", func(t *testing.T) {
		CEXFactoryRegistry = make(map[string]CEXFactory)

		// 注册nil工厂（虽然不推荐，但应该能处理）
		RegisterCEXFactory("nil-factory", nil)

		// 尝试创建客户端应该panic，因为factory是nil
		assert.Panics(t, func() {
			CreateCEXClient("nil-factory")
		})
	})

	t.Run("concurrent access safety", func(t *testing.T) {
		CEXFactoryRegistry = make(map[string]CEXFactory)

		// 并发注册和访问
		done := make(chan bool, 2)

		go func() {
			for i := 0; i < 100; i++ {
				factory := &mockCEXFactory{clientName: "concurrent"}
				RegisterCEXFactory("concurrent", factory)
			}
			done <- true
		}()

		go func() {
			for i := 0; i < 100; i++ {
				GetSupportedCEXes()
			}
			done <- true
		}()

		// 等待两个goroutine完成
		<-done
		<-done

		// 验证最终状态
		cexes := GetSupportedCEXes()
		assert.Contains(t, cexes, "concurrent")
	})
}

// 性能基准测试
func BenchmarkRegisterCEXFactory(b *testing.B) {
	CEXFactoryRegistry = make(map[string]CEXFactory)
	factory := &mockCEXFactory{clientName: "bench"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RegisterCEXFactory("bench", factory)
	}
}

func BenchmarkCreateCEXClient(b *testing.B) {
	CEXFactoryRegistry = make(map[string]CEXFactory)
	factory := &mockCEXFactory{clientName: "bench"}
	RegisterCEXFactory("bench", factory)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = CreateCEXClient("bench")
	}
}

func BenchmarkGetSupportedCEXes(b *testing.B) {
	CEXFactoryRegistry = make(map[string]CEXFactory)
	for i := 0; i < 10; i++ {
		factory := &mockCEXFactory{clientName: "bench"}
		RegisterCEXFactory("bench", factory)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GetSupportedCEXes()
	}
}
