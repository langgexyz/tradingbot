package indicators

import "errors"

var (
	// ErrInsufficientData 数据不足错误
	ErrInsufficientData = errors.New("insufficient data for calculation")
	
	// ErrInvalidPeriod 无效周期错误
	ErrInvalidPeriod = errors.New("invalid period, must be greater than 0")
	
	// ErrInvalidMultiplier 无效倍数错误
	ErrInvalidMultiplier = errors.New("invalid multiplier, must be greater than 0")
	
	// ErrEmptyPrices 空价格数组错误
	ErrEmptyPrices = errors.New("empty prices array")
)
