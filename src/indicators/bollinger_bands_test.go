package indicators

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestNewBollingerBands(t *testing.T) {
	tests := []struct {
		name       string
		period     int
		multiplier float64
	}{
		{
			name:       "valid parameters",
			period:     20,
			multiplier: 2.0,
		},
		{
			name:       "small period",
			period:     5,
			multiplier: 1.5,
		},
		{
			name:       "large multiplier",
			period:     14,
			multiplier: 3.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bb := NewBollingerBands(tt.period, tt.multiplier)
			assert.NotNil(t, bb)
			assert.Equal(t, tt.period, bb.Period)
			assert.Equal(t, decimal.NewFromFloat(tt.multiplier), bb.Multiplier)
		})
	}
}

func TestBollingerBands_Calculate(t *testing.T) {
	bb := NewBollingerBands(3, 2.0)

	t.Run("insufficient data", func(t *testing.T) {
		// 只提供2个价格，但需要3个
		prices := []decimal.Decimal{
			decimal.NewFromFloat(100),
			decimal.NewFromFloat(102),
		}

		result, err := bb.Calculate(prices)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Equal(t, ErrInsufficientData, err)
	})

	t.Run("sufficient data", func(t *testing.T) {
		// 提供足够的数据
		prices := []decimal.Decimal{
			decimal.NewFromFloat(100),
			decimal.NewFromFloat(102),
			decimal.NewFromFloat(98),
		}

		result, err := bb.Calculate(prices)
		assert.NoError(t, err)
		assert.NotNil(t, result)

		// 验证中轨（移动平均）
		expectedMiddle := decimal.NewFromFloat(100) // (100+102+98)/3 = 100
		assert.True(t, result.MiddleBand.Sub(expectedMiddle).Abs().LessThan(decimal.NewFromFloat(0.01)))

		// 验证上轨和下轨
		assert.True(t, result.UpperBand.GreaterThan(result.MiddleBand))
		assert.True(t, result.LowerBand.LessThan(result.MiddleBand))

		// 验证对称性
		upperDiff := result.UpperBand.Sub(result.MiddleBand)
		lowerDiff := result.MiddleBand.Sub(result.LowerBand)
		assert.True(t, upperDiff.Sub(lowerDiff).Abs().LessThan(decimal.NewFromFloat(0.01)))

		// 验证当前价格
		assert.Equal(t, decimal.NewFromFloat(98), result.Price)
	})

	t.Run("known values calculation", func(t *testing.T) {
		// 重新创建指标，使用已知的测试数据
		bb2 := NewBollingerBands(4, 2.0)
		testPrices := []decimal.Decimal{
			decimal.NewFromFloat(10),
			decimal.NewFromFloat(12),
			decimal.NewFromFloat(14),
			decimal.NewFromFloat(16),
		}

		result, err := bb2.Calculate(testPrices)
		assert.NoError(t, err)

		// 手动计算验证
		// 平均值: (10+12+14+16)/4 = 13
		expectedMiddle := decimal.NewFromFloat(13)
		assert.True(t, result.MiddleBand.Sub(expectedMiddle).Abs().LessThan(decimal.NewFromFloat(0.01)))

		// 标准差: sqrt(((10-13)^2 + (12-13)^2 + (14-13)^2 + (16-13)^2) / 4)
		// = sqrt((9 + 1 + 1 + 9) / 4) = sqrt(5) ≈ 2.236
		expectedStdDev := decimal.NewFromFloat(2.236)

		// 上轨: 13 + 2 * 2.236 = 17.472
		expectedUpper := expectedMiddle.Add(expectedStdDev.Mul(decimal.NewFromFloat(2.0)))
		assert.True(t, result.UpperBand.Sub(expectedUpper).Abs().LessThan(decimal.NewFromFloat(0.01)))

		// 下轨: 13 - 2 * 2.236 = 8.528
		expectedLower := expectedMiddle.Sub(expectedStdDev.Mul(decimal.NewFromFloat(2.0)))
		assert.True(t, result.LowerBand.Sub(expectedLower).Abs().LessThan(decimal.NewFromFloat(0.01)))
	})
}

func TestBollingerBands_EdgeCases(t *testing.T) {
	t.Run("all same prices", func(t *testing.T) {
		bb := NewBollingerBands(3, 2.0)

		// 添加相同的价格
		prices := []decimal.Decimal{
			decimal.NewFromFloat(100),
			decimal.NewFromFloat(100),
			decimal.NewFromFloat(100),
		}

		result, err := bb.Calculate(prices)
		assert.NoError(t, err)

		// 标准差应该为0，所以上下轨应该等于中轨
		assert.True(t, result.MiddleBand.Equal(decimal.NewFromFloat(100)))
		assert.True(t, result.UpperBand.Equal(result.MiddleBand))
		assert.True(t, result.LowerBand.Equal(result.MiddleBand))
	})

	t.Run("very small multiplier", func(t *testing.T) {
		bb := NewBollingerBands(3, 0.1)

		prices := []decimal.Decimal{
			decimal.NewFromFloat(100),
			decimal.NewFromFloat(102),
			decimal.NewFromFloat(98),
		}

		result, err := bb.Calculate(prices)
		assert.NoError(t, err)

		// 小的倍数应该导致窄的带宽
		upperDiff := result.UpperBand.Sub(result.MiddleBand)
		lowerDiff := result.MiddleBand.Sub(result.LowerBand)
		assert.True(t, upperDiff.LessThan(decimal.NewFromFloat(1)))
		assert.True(t, lowerDiff.LessThan(decimal.NewFromFloat(1)))
	})

	t.Run("more data than period", func(t *testing.T) {
		bb := NewBollingerBands(3, 2.0)

		// 提供5个价格，但只使用最近的3个
		prices := []decimal.Decimal{
			decimal.NewFromFloat(90), // 这两个应该被忽略
			decimal.NewFromFloat(95),
			decimal.NewFromFloat(100), // 只使用这3个
			decimal.NewFromFloat(102),
			decimal.NewFromFloat(98),
		}

		result, err := bb.Calculate(prices)
		assert.NoError(t, err)

		// 应该基于最近3个价格 (100, 102, 98) 计算
		expectedMiddle := decimal.NewFromFloat(100) // (100+102+98)/3
		assert.True(t, result.MiddleBand.Sub(expectedMiddle).Abs().LessThan(decimal.NewFromFloat(0.01)))
	})
}

func TestBollingerBands_CalculateSMA(t *testing.T) {
	bb := NewBollingerBands(3, 2.0)

	prices := []decimal.Decimal{
		decimal.NewFromFloat(100),
		decimal.NewFromFloat(102),
		decimal.NewFromFloat(98),
	}

	sma := bb.calculateSMA(prices)
	expected := decimal.NewFromFloat(100) // (100+102+98)/3

	assert.True(t, sma.Sub(expected).Abs().LessThan(decimal.NewFromFloat(0.01)))
}

func TestBollingerBands_CalculateStandardDeviation(t *testing.T) {
	bb := NewBollingerBands(3, 2.0)

	prices := []decimal.Decimal{
		decimal.NewFromFloat(100),
		decimal.NewFromFloat(102),
		decimal.NewFromFloat(98),
	}
	mean := decimal.NewFromFloat(100)

	stdDev := bb.calculateStandardDeviation(prices, mean)

	// 手动计算: sqrt(((100-100)^2 + (102-100)^2 + (98-100)^2) / 3)
	// = sqrt((0 + 4 + 4) / 3) = sqrt(8/3) ≈ 1.633
	expected := decimal.NewFromFloat(1.633)

	assert.True(t, stdDev.Sub(expected).Abs().LessThan(decimal.NewFromFloat(0.01)))
}

// 基准测试
func BenchmarkBollingerBands_Calculate(b *testing.B) {
	bb := NewBollingerBands(20, 2.0)

	// 创建测试数据
	prices := make([]decimal.Decimal, 100)
	for i := 0; i < 100; i++ {
		prices[i] = decimal.NewFromFloat(100 + float64(i%10))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bb.Calculate(prices)
	}
}
