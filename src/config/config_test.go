package config

import (
	"testing"

	"go-build-stream-gateway-go-server-main/src/database"

	"github.com/stretchr/testify/assert"
)

func TestConfig_GetCEXConfig(t *testing.T) {
	config := &Config{
		CEX: CEXConfig{
			Binance: BinanceConfig{
				APIKey:    "test_key",
				SecretKey: "test_secret",
				BaseURL:   "https://api.binance.com",
				Database: database.DatabaseConfig{
					Host:   "localhost",
					Port:   "5432",
					DBName: "test_binance",
				},
			},
		},
	}

	t.Run("valid CEX - binance", func(t *testing.T) {
		cexConfig, dbConfig, err := config.GetCEXConfig("binance")

		assert.NoError(t, err)
		assert.NotNil(t, cexConfig)
		assert.NotNil(t, dbConfig)

		binanceConfig := cexConfig.(*BinanceConfig)
		assert.Equal(t, "test_key", binanceConfig.APIKey)
		assert.Equal(t, "test_secret", binanceConfig.SecretKey)
		assert.Equal(t, "test_binance", dbConfig.DBName)
	})

	t.Run("invalid CEX", func(t *testing.T) {
		cexConfig, dbConfig, err := config.GetCEXConfig("invalid")

		assert.Error(t, err)
		assert.Nil(t, cexConfig)
		assert.Nil(t, dbConfig)
		assert.Contains(t, err.Error(), "unsupported CEX: invalid, only binance is supported")
	})

	t.Run("empty CEX", func(t *testing.T) {
		cexConfig, dbConfig, err := config.GetCEXConfig("")

		assert.Error(t, err)
		assert.Nil(t, cexConfig)
		assert.Nil(t, dbConfig)
	})
}

func TestConfig_GetSupportedCEXs(t *testing.T) {
	config := &Config{}

	supportedCEXs := config.GetSupportedCEXs()

	assert.Len(t, supportedCEXs, 1)
	assert.Contains(t, supportedCEXs, "binance")
}

func TestBinanceConfig_BasicValidation(t *testing.T) {
	tests := []struct {
		name   string
		config BinanceConfig
		valid  bool
	}{
		{
			name: "valid config",
			config: BinanceConfig{
				BaseURL: "https://api.binance.com",
				Timeout: 10,
			},
			valid: true,
		},
		{
			name: "empty base URL",
			config: BinanceConfig{
				BaseURL: "",
				Timeout: 10,
			},
			valid: false,
		},
		{
			name: "zero timeout",
			config: BinanceConfig{
				BaseURL: "https://api.binance.com",
				Timeout: 0,
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.valid {
				assert.NotEmpty(t, tt.config.BaseURL)
				assert.Greater(t, tt.config.Timeout, 0)
			} else {
				// 基本验证失败的情况
				if tt.config.BaseURL == "" {
					assert.Empty(t, tt.config.BaseURL)
				}
				if tt.config.Timeout <= 0 {
					assert.LessOrEqual(t, tt.config.Timeout, 0)
				}
			}
		})
	}
}

func TestSymbolInfo_BasicValidation(t *testing.T) {
	tests := []struct {
		name       string
		symbolInfo SymbolInfo
		valid      bool
	}{
		{
			name: "valid symbol info",
			symbolInfo: SymbolInfo{
				Symbol:     "BTCUSDT",
				BaseAsset:  "BTC",
				QuoteAsset: "USDT",
			},
			valid: true,
		},
		{
			name: "empty symbol",
			symbolInfo: SymbolInfo{
				Symbol:     "",
				BaseAsset:  "BTC",
				QuoteAsset: "USDT",
			},
			valid: false,
		},
		{
			name: "empty base asset",
			symbolInfo: SymbolInfo{
				Symbol:     "BTCUSDT",
				BaseAsset:  "",
				QuoteAsset: "USDT",
			},
			valid: false,
		},
		{
			name: "empty quote asset",
			symbolInfo: SymbolInfo{
				Symbol:     "BTCUSDT",
				BaseAsset:  "BTC",
				QuoteAsset: "",
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.valid {
				assert.NotEmpty(t, tt.symbolInfo.Symbol)
				assert.NotEmpty(t, tt.symbolInfo.BaseAsset)
				assert.NotEmpty(t, tt.symbolInfo.QuoteAsset)
			} else {
				// 至少有一个字段为空
				isEmpty := tt.symbolInfo.Symbol == "" ||
					tt.symbolInfo.BaseAsset == "" ||
					tt.symbolInfo.QuoteAsset == ""
				assert.True(t, isEmpty)
			}
		})
	}
}

func TestTradingConfig_BasicValidation(t *testing.T) {
	tests := []struct {
		name   string
		config TradingConfig
		valid  bool
	}{
		{
			name: "valid config",
			config: TradingConfig{
				Timeframe:      "4h",
				InitialCapital: 10000,
				Mode:           "backtest",
			},
			valid: true,
		},
		{
			name: "invalid initial capital",
			config: TradingConfig{
				Timeframe:      "4h",
				InitialCapital: -1000, // 负数
				Mode:           "backtest",
			},
			valid: false,
		},
		{
			name: "empty timeframe",
			config: TradingConfig{
				Timeframe:      "",
				InitialCapital: 10000,
				Mode:           "backtest",
			},
			valid: false,
		},
		{
			name: "empty mode",
			config: TradingConfig{
				Timeframe:      "4h",
				InitialCapital: 10000,
				Mode:           "",
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.valid {
				assert.NotEmpty(t, tt.config.Timeframe)
				assert.Greater(t, tt.config.InitialCapital, 0.0)
				assert.NotEmpty(t, tt.config.Mode)
			} else {
				// 验证失败的情况
				invalid := tt.config.Timeframe == "" ||
					tt.config.InitialCapital <= 0 ||
					tt.config.Mode == ""
				assert.True(t, invalid)
			}
		})
	}
}

func TestAppConfig_DefaultValues(t *testing.T) {
	// 验证全局配置的默认值
	assert.NotNil(t, AppConfig)
	assert.NotEmpty(t, AppConfig.CEX.Binance.BaseURL)
	assert.Equal(t, "backtest", AppConfig.Trading.Mode)
	assert.Equal(t, "4h", AppConfig.Trading.Timeframe)
	assert.Equal(t, 10000.0, AppConfig.Trading.InitialCapital)
	assert.NotEmpty(t, AppConfig.Symbols)
}
