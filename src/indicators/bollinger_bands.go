package indicators

import (
	"math"
	"github.com/shopspring/decimal"
)

// BollingerBands 布林道结构体
type BollingerBands struct {
	Period     int             // 计算周期，通常为20
	Multiplier decimal.Decimal // 标准差倍数，通常为2
}

// BollingerBandsResult 布林道计算结果
type BollingerBandsResult struct {
	UpperBand  decimal.Decimal // 上轨
	MiddleBand decimal.Decimal // 中轨（移动平均线）
	LowerBand  decimal.Decimal // 下轨
	Price      decimal.Decimal // 当前价格
	Timestamp  int64           // 时间戳
}

// NewBollingerBands 创建新的布林道指标
func NewBollingerBands(period int, multiplier float64) *BollingerBands {
	return &BollingerBands{
		Period:     period,
		Multiplier: decimal.NewFromFloat(multiplier),
	}
}

// Calculate 计算布林道指标
func (bb *BollingerBands) Calculate(prices []decimal.Decimal) (*BollingerBandsResult, error) {
	if len(prices) < bb.Period {
		return nil, ErrInsufficientData
	}

	// 取最近period个价格
	recentPrices := prices[len(prices)-bb.Period:]
	
	// 计算移动平均线（中轨）
	sma := bb.calculateSMA(recentPrices)
	
	// 计算标准差
	std := bb.calculateStandardDeviation(recentPrices, sma)
	
	// 计算上轨和下轨
	upperBand := sma.Add(bb.Multiplier.Mul(std))
	lowerBand := sma.Sub(bb.Multiplier.Mul(std))
	
	return &BollingerBandsResult{
		UpperBand:  upperBand,
		MiddleBand: sma,
		LowerBand:  lowerBand,
		Price:      prices[len(prices)-1], // 最新价格
	}, nil
}

// calculateSMA 计算简单移动平均线
func (bb *BollingerBands) calculateSMA(prices []decimal.Decimal) decimal.Decimal {
	sum := decimal.Zero
	for _, price := range prices {
		sum = sum.Add(price)
	}
	return sum.Div(decimal.NewFromInt(int64(len(prices))))
}

// calculateStandardDeviation 计算标准差
func (bb *BollingerBands) calculateStandardDeviation(prices []decimal.Decimal, mean decimal.Decimal) decimal.Decimal {
	sum := decimal.Zero
	count := decimal.NewFromInt(int64(len(prices)))
	
	for _, price := range prices {
		diff := price.Sub(mean)
		sum = sum.Add(diff.Mul(diff))
	}
	
	variance := sum.Div(count)
	// 由于decimal包没有直接的sqrt方法，我们需要转换到float64计算后再转回来
	varianceFloat, _ := variance.Float64()
	stdFloat := math.Sqrt(varianceFloat)
	
	return decimal.NewFromFloat(stdFloat)
}

// IsUpperBreakout 判断是否突破上轨（卖出信号）
func (result *BollingerBandsResult) IsUpperBreakout() bool {
	return result.Price.GreaterThanOrEqual(result.UpperBand)
}

// IsLowerBreakout 判断是否突破下轨（买入信号）
func (result *BollingerBandsResult) IsLowerBreakout() bool {
	return result.Price.LessThanOrEqual(result.LowerBand)
}

// GetBandWidth 获取布林道带宽（上轨-下轨）/中轨
func (result *BollingerBandsResult) GetBandWidth() decimal.Decimal {
	return result.UpperBand.Sub(result.LowerBand).Div(result.MiddleBand)
}

// GetPercentB 获取%B指标 (价格-下轨)/(上轨-下轨)
func (result *BollingerBandsResult) GetPercentB() decimal.Decimal {
	numerator := result.Price.Sub(result.LowerBand)
	denominator := result.UpperBand.Sub(result.LowerBand)
	if denominator.Equal(decimal.Zero) {
		return decimal.Zero
	}
	return numerator.Div(denominator)
}
