package cex

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestTradingPair_String(t *testing.T) {
	tests := []struct {
		name     string
		pair     TradingPair
		expected string
	}{
		{
			name:     "BTC/USDT pair",
			pair:     TradingPair{Base: "BTC", Quote: "USDT"},
			expected: "BTC/USDT",
		},
		{
			name:     "ETH/USDC pair",
			pair:     TradingPair{Base: "ETH", Quote: "USDC"},
			expected: "ETH/USDC",
		},
		{
			name:     "DOGE/USDT pair",
			pair:     TradingPair{Base: "DOGE", Quote: "USDT"},
			expected: "DOGE/USDT",
		},
		{
			name:     "empty pair",
			pair:     TradingPair{Base: "", Quote: ""},
			expected: "/",
		},
		{
			name:     "single base",
			pair:     TradingPair{Base: "BTC", Quote: ""},
			expected: "BTC/",
		},
		{
			name:     "single quote",
			pair:     TradingPair{Base: "", Quote: "USDT"},
			expected: "/USDT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.pair.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestKlineData_Structure(t *testing.T) {
	pair := TradingPair{Base: "BTC", Quote: "USDT"}
	openTime := time.Now().Add(-4 * time.Hour)
	closeTime := time.Now()

	kline := &KlineData{
		TradingPair:         pair,
		OpenTime:            openTime,
		Open:                decimal.NewFromFloat(50000),
		High:                decimal.NewFromFloat(51000),
		Low:                 decimal.NewFromFloat(49000),
		Close:               decimal.NewFromFloat(50500),
		Volume:              decimal.NewFromFloat(100),
		CloseTime:           closeTime,
		QuoteVolume:         decimal.NewFromFloat(5050000),
		TakerBuyVolume:      decimal.NewFromFloat(60),
		TakerBuyQuoteVolume: decimal.NewFromFloat(3030000),
	}

	// 验证所有字段都被正确设置
	assert.Equal(t, pair, kline.TradingPair)
	assert.Equal(t, openTime, kline.OpenTime)
	assert.Equal(t, decimal.NewFromFloat(50000), kline.Open)
	assert.Equal(t, decimal.NewFromFloat(51000), kline.High)
	assert.Equal(t, decimal.NewFromFloat(49000), kline.Low)
	assert.Equal(t, decimal.NewFromFloat(50500), kline.Close)
	assert.Equal(t, decimal.NewFromFloat(100), kline.Volume)
	assert.Equal(t, closeTime, kline.CloseTime)
	assert.Equal(t, decimal.NewFromFloat(5050000), kline.QuoteVolume)
	assert.Equal(t, decimal.NewFromFloat(60), kline.TakerBuyVolume)
	assert.Equal(t, decimal.NewFromFloat(3030000), kline.TakerBuyQuoteVolume)
}

func TestOrderSide_Constants(t *testing.T) {
	assert.Equal(t, OrderSide("BUY"), OrderSideBuy)
	assert.Equal(t, OrderSide("SELL"), OrderSideSell)
}

func TestOrderType_Constants(t *testing.T) {
	assert.Equal(t, OrderType("MARKET"), OrderTypeMarket)
	assert.Equal(t, OrderType("LIMIT"), OrderTypeLimit)
}

func TestBuyOrderRequest_Structure(t *testing.T) {
	pair := TradingPair{Base: "BTC", Quote: "USDT"}

	// Market order
	marketOrder := BuyOrderRequest{
		TradingPair: pair,
		Type:        OrderTypeMarket,
		Quantity:    decimal.NewFromFloat(0.1),
	}

	assert.Equal(t, pair, marketOrder.TradingPair)
	assert.Equal(t, OrderTypeMarket, marketOrder.Type)
	assert.Equal(t, decimal.NewFromFloat(0.1), marketOrder.Quantity)
	assert.True(t, marketOrder.Price.IsZero()) // Market order should not have price

	// Limit order
	limitOrder := BuyOrderRequest{
		TradingPair: pair,
		Type:        OrderTypeLimit,
		Quantity:    decimal.NewFromFloat(0.1),
		Price:       decimal.NewFromFloat(50000),
	}

	assert.Equal(t, pair, limitOrder.TradingPair)
	assert.Equal(t, OrderTypeLimit, limitOrder.Type)
	assert.Equal(t, decimal.NewFromFloat(0.1), limitOrder.Quantity)
	assert.Equal(t, decimal.NewFromFloat(50000), limitOrder.Price)
}

func TestSellOrderRequest_Structure(t *testing.T) {
	pair := TradingPair{Base: "BTC", Quote: "USDT"}

	// Market order
	marketOrder := SellOrderRequest{
		TradingPair: pair,
		Type:        OrderTypeMarket,
		Quantity:    decimal.NewFromFloat(0.1),
	}

	assert.Equal(t, pair, marketOrder.TradingPair)
	assert.Equal(t, OrderTypeMarket, marketOrder.Type)
	assert.Equal(t, decimal.NewFromFloat(0.1), marketOrder.Quantity)
	assert.True(t, marketOrder.Price.IsZero()) // Market order should not have price

	// Limit order
	limitOrder := SellOrderRequest{
		TradingPair: pair,
		Type:        OrderTypeLimit,
		Quantity:    decimal.NewFromFloat(0.1),
		Price:       decimal.NewFromFloat(50000),
	}

	assert.Equal(t, pair, limitOrder.TradingPair)
	assert.Equal(t, OrderTypeLimit, limitOrder.Type)
	assert.Equal(t, decimal.NewFromFloat(0.1), limitOrder.Quantity)
	assert.Equal(t, decimal.NewFromFloat(50000), limitOrder.Price)
}

func TestOrderResult_Structure(t *testing.T) {
	pair := TradingPair{Base: "BTC", Quote: "USDT"}
	transactTime := time.Now()

	result := &OrderResult{
		TradingPair:   pair,
		OrderID:       "12345",
		ClientOrderID: "client_123",
		Price:         decimal.NewFromFloat(50000),
		Quantity:      decimal.NewFromFloat(0.1),
		Side:          OrderSideBuy,
		Status:        "FILLED",
		Type:          OrderTypeMarket,
		TransactTime:  transactTime,
	}

	assert.Equal(t, pair, result.TradingPair)
	assert.Equal(t, "12345", result.OrderID)
	assert.Equal(t, "client_123", result.ClientOrderID)
	assert.Equal(t, decimal.NewFromFloat(50000), result.Price)
	assert.Equal(t, decimal.NewFromFloat(0.1), result.Quantity)
	assert.Equal(t, OrderSideBuy, result.Side)
	assert.Equal(t, "FILLED", result.Status)
	assert.Equal(t, OrderTypeMarket, result.Type)
	assert.Equal(t, transactTime, result.TransactTime)
}

func TestAccountBalance_Structure(t *testing.T) {
	balance := &AccountBalance{
		Asset:  "USDT",
		Free:   decimal.NewFromFloat(1000.50),
		Locked: decimal.NewFromFloat(100.25),
	}

	assert.Equal(t, "USDT", balance.Asset)
	assert.Equal(t, decimal.NewFromFloat(1000.50), balance.Free)
	assert.Equal(t, decimal.NewFromFloat(100.25), balance.Locked)
}

// 测试边界情况
func TestTradingPair_EdgeCases(t *testing.T) {
	t.Run("very long symbols", func(t *testing.T) {
		pair := TradingPair{
			Base:  "VERYLONGBASETOKEN123456789",
			Quote: "VERYLONGQUOTETOKEN123456789",
		}
		expected := "VERYLONGBASETOKEN123456789/VERYLONGQUOTETOKEN123456789"
		assert.Equal(t, expected, pair.String())
	})

	t.Run("special characters in symbols", func(t *testing.T) {
		pair := TradingPair{
			Base:  "BTC-USD",
			Quote: "USD_T",
		}
		expected := "BTC-USD/USD_T"
		assert.Equal(t, expected, pair.String())
	})

	t.Run("unicode symbols", func(t *testing.T) {
		pair := TradingPair{
			Base:  "比特币",
			Quote: "美元",
		}
		expected := "比特币/美元"
		assert.Equal(t, expected, pair.String())
	})
}

// 性能基准测试
func BenchmarkTradingPair_String(b *testing.B) {
	pair := TradingPair{Base: "BTC", Quote: "USDT"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pair.String()
	}
}

func BenchmarkKlineData_Creation(b *testing.B) {
	pair := TradingPair{Base: "BTC", Quote: "USDT"}
	openTime := time.Now()
	closeTime := time.Now().Add(time.Hour)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = &KlineData{
			TradingPair:         pair,
			OpenTime:            openTime,
			Open:                decimal.NewFromFloat(50000),
			High:                decimal.NewFromFloat(51000),
			Low:                 decimal.NewFromFloat(49000),
			Close:               decimal.NewFromFloat(50500),
			Volume:              decimal.NewFromFloat(100),
			CloseTime:           closeTime,
			QuoteVolume:         decimal.NewFromFloat(5050000),
			TakerBuyVolume:      decimal.NewFromFloat(60),
			TakerBuyQuoteVolume: decimal.NewFromFloat(3030000),
		}
	}
}
