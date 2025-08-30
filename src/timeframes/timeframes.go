package timeframes

import (
	"fmt"
	"time"
)

// Timeframe 时间刻度枚举
type Timeframe string

const (
	// 支持的时间刻度
	Timeframe1m  Timeframe = "1m"  // 1分钟
	Timeframe3m  Timeframe = "3m"  // 3分钟
	Timeframe5m  Timeframe = "5m"  // 5分钟
	Timeframe15m Timeframe = "15m" // 15分钟
	Timeframe30m Timeframe = "30m" // 30分钟
	Timeframe1h  Timeframe = "1h"  // 1小时
	Timeframe2h  Timeframe = "2h"  // 2小时
	Timeframe4h  Timeframe = "4h"  // 4小时
	Timeframe6h  Timeframe = "6h"  // 6小时
	Timeframe8h  Timeframe = "8h"  // 8小时
	Timeframe12h Timeframe = "12h" // 12小时
	Timeframe1d  Timeframe = "1d"  // 1天
	Timeframe3d  Timeframe = "3d"  // 3天
	Timeframe1w  Timeframe = "1w"  // 1周
	Timeframe1M  Timeframe = "1M"  // 1月
)

// GetDuration 获取时间刻度对应的Duration
func (tf Timeframe) GetDuration() (time.Duration, error) {
	switch tf {
	case Timeframe1m:
		return time.Minute, nil
	case Timeframe3m:
		return 3 * time.Minute, nil
	case Timeframe5m:
		return 5 * time.Minute, nil
	case Timeframe15m:
		return 15 * time.Minute, nil
	case Timeframe30m:
		return 30 * time.Minute, nil
	case Timeframe1h:
		return time.Hour, nil
	case Timeframe2h:
		return 2 * time.Hour, nil
	case Timeframe4h:
		return 4 * time.Hour, nil
	case Timeframe6h:
		return 6 * time.Hour, nil
	case Timeframe8h:
		return 8 * time.Hour, nil
	case Timeframe12h:
		return 12 * time.Hour, nil
	case Timeframe1d:
		return 24 * time.Hour, nil
	case Timeframe3d:
		return 3 * 24 * time.Hour, nil
	case Timeframe1w:
		return 7 * 24 * time.Hour, nil
	case Timeframe1M:
		return 30 * 24 * time.Hour, nil // 近似1个月
	default:
		return 0, fmt.Errorf("unsupported timeframe: %s", tf)
	}
}

// GetMinutes 获取时间刻度对应的分钟数
func (tf Timeframe) GetMinutes() (int64, error) {
	duration, err := tf.GetDuration()
	if err != nil {
		return 0, err
	}
	return int64(duration.Minutes()), nil
}

// String 返回字符串表示
func (tf Timeframe) String() string {
	return string(tf)
}

// IsValid 检查时间刻度是否有效
func (tf Timeframe) IsValid() bool {
	_, err := tf.GetDuration()
	return err == nil
}

// GetAllTimeframes 获取所有支持的时间刻度
func GetAllTimeframes() []Timeframe {
	return []Timeframe{
		Timeframe1m,
		Timeframe3m,
		Timeframe5m,
		Timeframe15m,
		Timeframe30m,
		Timeframe1h,
		Timeframe2h,
		Timeframe4h,
		Timeframe6h,
		Timeframe8h,
		Timeframe12h,
		Timeframe1d,
		Timeframe3d,
		Timeframe1w,
		Timeframe1M,
	}
}

// ParseTimeframe 解析时间刻度字符串
func ParseTimeframe(s string) (Timeframe, error) {
	tf := Timeframe(s)
	if !tf.IsValid() {
		return "", fmt.Errorf("invalid timeframe: %s", s)
	}
	return tf, nil
}

// GetBinanceInterval 获取币安API对应的时间间隔字符串
func (tf Timeframe) GetBinanceInterval() string {
	// 币安API使用的时间间隔格式与我们的定义相同
	return string(tf)
}

// GetMaxHistoryDays 获取该时间刻度建议的最大历史数据天数
func (tf Timeframe) GetMaxHistoryDays() int {
	switch tf {
	case Timeframe1m, Timeframe3m, Timeframe5m:
		return 7 // 短周期最多取7天
	case Timeframe15m, Timeframe30m:
		return 30 // 30天
	case Timeframe1h, Timeframe2h:
		return 90 // 90天
	case Timeframe4h, Timeframe6h, Timeframe8h, Timeframe12h:
		return 180 // 半年
	case Timeframe1d:
		return 365 // 1年
	case Timeframe3d, Timeframe1w:
		return 730 // 2年
	case Timeframe1M:
		return 1095 // 3年
	default:
		return 30
	}
}

// CalculateDataPoints 计算指定天数内的数据点数量
func (tf Timeframe) CalculateDataPoints(days int) (int, error) {
	duration, err := tf.GetDuration()
	if err != nil {
		return 0, err
	}

	totalDuration := time.Duration(days) * 24 * time.Hour
	points := int(totalDuration / duration)

	return points, nil
}
