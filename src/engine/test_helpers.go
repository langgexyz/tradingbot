package engine

import (
	"context"
	"errors"
	"time"

	"tradingbot/src/cex"

	"github.com/shopspring/decimal"
)

var TestError = errors.New("test error")

// MockCEXClient 用于测试的CEX客户端mock
type MockCEXClient struct {
	ShouldError bool
	CallCount   int
}

func (m *MockCEXClient) GetName() string {
	return "mock_cex"
}

func (m *MockCEXClient) GetDatabase() interface{} {
	return nil
}

func (m *MockCEXClient) GetTradingFee() float64 {
	return 0.001
}

func (m *MockCEXClient) GetKlines(ctx context.Context, pair cex.TradingPair, interval string, limit int) ([]*cex.KlineData, error) {
	m.CallCount++
	if m.ShouldError {
		return nil, TestError
	}
	return []*cex.KlineData{}, nil
}

func (m *MockCEXClient) GetKlinesWithTimeRange(ctx context.Context, pair cex.TradingPair, interval string, startTime, endTime time.Time, limit int) ([]*cex.KlineData, error) {
	m.CallCount++
	if m.ShouldError {
		return nil, TestError
	}
	return []*cex.KlineData{}, nil
}

func (m *MockCEXClient) Buy(ctx context.Context, req cex.BuyOrderRequest) (*cex.OrderResult, error) {
	m.CallCount++
	if m.ShouldError {
		return nil, TestError
	}
	return &cex.OrderResult{}, nil
}

func (m *MockCEXClient) Sell(ctx context.Context, req cex.SellOrderRequest) (*cex.OrderResult, error) {
	m.CallCount++
	if m.ShouldError {
		return nil, TestError
	}
	return &cex.OrderResult{}, nil
}

func (m *MockCEXClient) GetAccount(ctx context.Context) ([]*cex.AccountBalance, error) {
	m.CallCount++
	if m.ShouldError {
		return nil, TestError
	}
	return []*cex.AccountBalance{}, nil
}

func (m *MockCEXClient) Ping(ctx context.Context) error {
	m.CallCount++
	if m.ShouldError {
		return TestError
	}
	return nil
}

// CreateTestKlines 创建测试用的K线数据
func CreateTestKlines(count int, startTime time.Time, interval time.Duration) []*cex.KlineData {
	klines := make([]*cex.KlineData, count)
	basePrice := decimal.NewFromFloat(0.1)

	for i := 0; i < count; i++ {
		// 模拟价格波动
		priceVariation := decimal.NewFromFloat(float64(i%10-5) * 0.001) // ±0.5% 波动
		price := basePrice.Add(priceVariation)

		klines[i] = &cex.KlineData{
			OpenTime:  startTime.Add(time.Duration(i) * interval),
			CloseTime: startTime.Add(time.Duration(i+1) * interval),
			Open:      price,
			High:      price.Mul(decimal.NewFromFloat(1.02)),   // +2%
			Low:       price.Mul(decimal.NewFromFloat(0.98)),   // -2%
			Close:     price.Add(decimal.NewFromFloat(0.0001)), // 微小变化
			Volume:    decimal.NewFromInt(1000),
		}
	}

	return klines
}

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
