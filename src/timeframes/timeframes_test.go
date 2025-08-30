package timeframes

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTimeframe_GetDuration(t *testing.T) {
	tests := []struct {
		name      string
		timeframe Timeframe
		expected  time.Duration
		wantErr   bool
	}{
		// 分钟
		{"1m", Timeframe1m, time.Minute, false},
		{"3m", Timeframe3m, 3 * time.Minute, false},
		{"5m", Timeframe5m, 5 * time.Minute, false},
		{"15m", Timeframe15m, 15 * time.Minute, false},
		{"30m", Timeframe30m, 30 * time.Minute, false},

		// 小时
		{"1h", Timeframe1h, time.Hour, false},
		{"2h", Timeframe2h, 2 * time.Hour, false},
		{"4h", Timeframe4h, 4 * time.Hour, false},
		{"6h", Timeframe6h, 6 * time.Hour, false},
		{"8h", Timeframe8h, 8 * time.Hour, false},
		{"12h", Timeframe12h, 12 * time.Hour, false},

		// 天
		{"1d", Timeframe1d, 24 * time.Hour, false},
		{"3d", Timeframe3d, 3 * 24 * time.Hour, false},

		// 周
		{"1w", Timeframe1w, 7 * 24 * time.Hour, false},

		// 月
		{"1M", Timeframe1M, 30 * 24 * time.Hour, false},

		// 无效
		{"invalid", Timeframe("invalid"), 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.timeframe.GetDuration()

			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, time.Duration(0), result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestTimeframe_GetMinutes(t *testing.T) {
	tests := []struct {
		name      string
		timeframe Timeframe
		expected  int64
		wantErr   bool
	}{
		{"1m", Timeframe1m, 1, false},
		{"5m", Timeframe5m, 5, false},
		{"1h", Timeframe1h, 60, false},
		{"4h", Timeframe4h, 240, false},
		{"1d", Timeframe1d, 1440, false},
		{"invalid", Timeframe("invalid"), 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.timeframe.GetMinutes()

			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, int64(0), result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestTimeframe_String(t *testing.T) {
	tests := []struct {
		timeframe Timeframe
		expected  string
	}{
		{Timeframe1m, "1m"},
		{Timeframe1h, "1h"},
		{Timeframe1d, "1d"},
		{Timeframe1w, "1w"},
		{Timeframe1M, "1M"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.timeframe.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTimeframe_IsValid(t *testing.T) {
	tests := []struct {
		name      string
		timeframe Timeframe
		expected  bool
	}{
		// 有效的时间周期
		{"1m", Timeframe1m, true},
		{"3m", Timeframe3m, true},
		{"5m", Timeframe5m, true},
		{"15m", Timeframe15m, true},
		{"30m", Timeframe30m, true},
		{"1h", Timeframe1h, true},
		{"2h", Timeframe2h, true},
		{"4h", Timeframe4h, true},
		{"6h", Timeframe6h, true},
		{"8h", Timeframe8h, true},
		{"12h", Timeframe12h, true},
		{"1d", Timeframe1d, true},
		{"3d", Timeframe3d, true},
		{"1w", Timeframe1w, true},
		{"1M", Timeframe1M, true},

		// 无效的时间周期
		{"invalid", Timeframe("invalid"), false},
		{"empty", Timeframe(""), false},
		{"2s", Timeframe("2s"), false},
		{"7m", Timeframe("7m"), false},
		{"3h", Timeframe("3h"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.timeframe.IsValid()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetAllTimeframes(t *testing.T) {
	timeframes := GetAllTimeframes()

	// 验证返回的时间周期列表
	assert.NotEmpty(t, timeframes)
	assert.Contains(t, timeframes, Timeframe1m)
	assert.Contains(t, timeframes, Timeframe1h)
	assert.Contains(t, timeframes, Timeframe1d)
	assert.Contains(t, timeframes, Timeframe1w)
	assert.Contains(t, timeframes, Timeframe1M)

	// 验证所有返回的时间周期都是有效的
	for _, tf := range timeframes {
		assert.True(t, tf.IsValid(), "timeframe %s should be valid", tf)
	}

	// 验证数量
	assert.Equal(t, 15, len(timeframes)) // 应该有15个时间周期
}

func TestParseTimeframe(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected Timeframe
		wantErr  bool
	}{
		{"valid 1m", "1m", Timeframe1m, false},
		{"valid 1h", "1h", Timeframe1h, false},
		{"valid 1d", "1d", Timeframe1d, false},
		{"valid 1M", "1M", Timeframe1M, false},
		{"invalid", "invalid", "", true},
		{"empty", "", "", true},
		{"unsupported", "2s", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseTimeframe(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, Timeframe(""), result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestTimeframe_GetBinanceInterval(t *testing.T) {
	tests := []struct {
		timeframe Timeframe
		expected  string
	}{
		{Timeframe1m, "1m"},
		{Timeframe5m, "5m"},
		{Timeframe1h, "1h"},
		{Timeframe4h, "4h"},
		{Timeframe1d, "1d"},
		{Timeframe1w, "1w"},
		{Timeframe1M, "1M"},
	}

	for _, tt := range tests {
		t.Run(string(tt.timeframe), func(t *testing.T) {
			result := tt.timeframe.GetBinanceInterval()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTimeframe_GetMaxHistoryDays(t *testing.T) {
	tests := []struct {
		timeframe Timeframe
		expected  int
	}{
		{Timeframe1m, 7},
		{Timeframe5m, 7},
		{Timeframe15m, 30},
		{Timeframe1h, 90},
		{Timeframe4h, 180},
		{Timeframe1d, 365},
		{Timeframe1w, 730},
		{Timeframe1M, 1095},
	}

	for _, tt := range tests {
		t.Run(string(tt.timeframe), func(t *testing.T) {
			result := tt.timeframe.GetMaxHistoryDays()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTimeframe_CalculateDataPoints(t *testing.T) {
	tests := []struct {
		name      string
		timeframe Timeframe
		days      int
		expected  int
		wantErr   bool
	}{
		{"1m for 1 day", Timeframe1m, 1, 1440, false}, // 24*60 = 1440 minutes
		{"5m for 1 day", Timeframe5m, 1, 288, false},  // 1440/5 = 288
		{"1h for 1 day", Timeframe1h, 1, 24, false},   // 24 hours
		{"4h for 1 day", Timeframe4h, 1, 6, false},    // 24/4 = 6
		{"1d for 7 days", Timeframe1d, 7, 7, false},   // 7 days
		{"invalid timeframe", Timeframe("invalid"), 1, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.timeframe.CalculateDataPoints(tt.days)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, 0, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestTimeframeConstants(t *testing.T) {
	// 验证常量定义
	assert.Equal(t, "1m", string(Timeframe1m))
	assert.Equal(t, "3m", string(Timeframe3m))
	assert.Equal(t, "5m", string(Timeframe5m))
	assert.Equal(t, "15m", string(Timeframe15m))
	assert.Equal(t, "30m", string(Timeframe30m))
	assert.Equal(t, "1h", string(Timeframe1h))
	assert.Equal(t, "2h", string(Timeframe2h))
	assert.Equal(t, "4h", string(Timeframe4h))
	assert.Equal(t, "6h", string(Timeframe6h))
	assert.Equal(t, "8h", string(Timeframe8h))
	assert.Equal(t, "12h", string(Timeframe12h))
	assert.Equal(t, "1d", string(Timeframe1d))
	assert.Equal(t, "3d", string(Timeframe3d))
	assert.Equal(t, "1w", string(Timeframe1w))
	assert.Equal(t, "1M", string(Timeframe1M))
}

// 基准测试
func BenchmarkTimeframe_IsValid(b *testing.B) {
	timeframes := []Timeframe{Timeframe1m, Timeframe1h, Timeframe1d, Timeframe("invalid")}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tf := timeframes[i%len(timeframes)]
		tf.IsValid()
	}
}

func BenchmarkTimeframe_GetDuration(b *testing.B) {
	timeframes := []Timeframe{Timeframe1m, Timeframe5m, Timeframe1h, Timeframe4h, Timeframe1d}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tf := timeframes[i%len(timeframes)]
		tf.GetDuration()
	}
}

func BenchmarkParseTimeframe(b *testing.B) {
	timeframeStrings := []string{"1m", "5m", "1h", "4h", "1d", "invalid"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := timeframeStrings[i%len(timeframeStrings)]
		ParseTimeframe(s)
	}
}
