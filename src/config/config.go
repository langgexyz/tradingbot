package config

import (
	"fmt"
	"time"

	"go-build-stream-gateway-go-server-main/src/database"
	"go-build-stream-gateway-go-server-main/src/timeframes"

	"github.com/shopspring/decimal"
	"github.com/xpwu/go-config/configs"
)

// Config 主配置结构
type Config struct {
	CEX      CEXConfig      `conf:"cex,中心化交易所配置"`
	Trading  TradingConfig  `conf:"trading,交易基础配置"`
	Strategy StrategyConfig `conf:"strategy,策略配置"`
	Backtest BacktestConfig `conf:"backtest,回测配置"`
	Symbols  []SymbolInfo   `conf:"symbols,支持的交易对列表"`
}

// SymbolInfo 交易对信息（简化版）
type SymbolInfo struct {
	Symbol     string `conf:"symbol,交易对代码"`
	BaseAsset  string `conf:"base_asset,基础资产"`
	QuoteAsset string `conf:"quote_asset,计价资产"`
}

// CEXConfig 中心化交易所配置
type CEXConfig struct {
	Binance BinanceConfig `conf:"binance,币安配置"`
}

// BinanceConfig 币安API配置
type BinanceConfig struct {
	APIKey        string                  `conf:"api_key,API密钥"`
	SecretKey     string                  `conf:"secret_key,API私钥"`
	BaseURL       string                  `conf:"base_url,API地址"`
	Timeout       int                     `conf:"timeout,请求超时时间(秒)"`
	EnableTrading bool                    `conf:"enable_trading,启用交易权限"`
	ReadOnly      bool                    `conf:"read_only,只读模式"`
	Database      database.DatabaseConfig `conf:"database,数据库配置"`
}

// OKXConfig OKX API配置
type OKXConfig struct {
	APIKey        string                  `conf:"api_key,API密钥"`
	SecretKey     string                  `conf:"secret_key,API私钥"`
	Passphrase    string                  `conf:"passphrase,API密码"`
	BaseURL       string                  `conf:"base_url,API地址"`
	Timeout       int                     `conf:"timeout,请求超时时间(秒)"`
	EnableTrading bool                    `conf:"enable_trading,启用交易权限"`
	ReadOnly      bool                    `conf:"read_only,只读模式"`
	Database      database.DatabaseConfig `conf:"database,数据库配置"`
}

// HuobiConfig 火币API配置
type HuobiConfig struct {
	APIKey        string                  `conf:"api_key,API密钥"`
	SecretKey     string                  `conf:"secret_key,API私钥"`
	BaseURL       string                  `conf:"base_url,API地址"`
	Timeout       int                     `conf:"timeout,请求超时时间(秒)"`
	EnableTrading bool                    `conf:"enable_trading,启用交易权限"`
	ReadOnly      bool                    `conf:"read_only,只读模式"`
	Database      database.DatabaseConfig `conf:"database,数据库配置"`
}

// GetCEXConfig 根据CEX名称获取对应的配置
func (c *Config) GetCEXConfig(cex string) (interface{}, *database.DatabaseConfig, error) {
	switch cex {
	case "binance":
		return &c.CEX.Binance, &c.CEX.Binance.Database, nil
	default:
		return nil, nil, fmt.Errorf("unsupported CEX: %s, only binance is supported", cex)
	}
}

// GetSupportedCEXs 获取支持的CEX列表
func (c *Config) GetSupportedCEXs() []string {
	return []string{"binance"}
}

// TradingConfig 交易配置
type TradingConfig struct {
	Symbol              string          `conf:"symbol,交易对 - 通过命令行参数-s指定，如BTCUSDT、ETHUSDT等"`
	Timeframe           string          `conf:"timeframe,K线周期 - 支持1s,1m,3m,5m,15m,30m,1h,2h,4h,6h,8h,12h,1d,3d,1w,1M"`
	InitialCapital      float64         `conf:"initial_capital,初始资金 - 回测或交易的起始金额(USDT)"`
	Mode                string          `conf:"mode,运行模式 - backtest=回测,paper=模拟,live=实盘"`
	MaxPositions        int             `conf:"max_positions,最大持仓数 - 同时持有的不同币种数量，通常设为1"`
	PositionSizePercent float64         `conf:"position_size_percent,仓位比例 - 每次交易使用的资金比例，0.95=95%"`
	MinTradeAmount      float64         `conf:"min_trade_amount,最小交易额 - 低于此金额不交易，避免手续费占比过高"`
	KlineConfig         KlineDataConfig `conf:"kline_config,K线数据配置"`
}

// KlineDataConfig K线数据配置
type KlineDataConfig struct {
	DefaultLimit     int    `conf:"default_limit,默认K线数据条数 - 默认500，最大1000"`
	TimeZone         string `conf:"time_zone,时区设置 - 默认UTC，支持-12:00到+14:00"`
	EnableHistorical bool   `conf:"enable_historical,启用历史数据 - 是否获取指定时间范围的历史数据"`
}

// StrategyConfig 策略配置
type StrategyConfig struct {
	Name       string                   `conf:"name,策略名称 - 目前支持bollinger_bands"`
	Parameters BollingerBandsParameters `conf:"parameters,布林道策略参数"`
}

// BollingerBandsParameters 布林道策略参数
type BollingerBandsParameters struct {
	Period              int     `conf:"period,计算周期 - 默认20"`
	Multiplier          float64 `conf:"multiplier,标准差倍数 - 默认2.0"`
	PositionSizePercent float64 `conf:"position_size_percent,仓位比例 - 默认0.95"`
	MinTradeAmount      float64 `conf:"min_trade_amount,最小交易额 - 默认10"`
}

// BacktestConfig 回测配置
type BacktestConfig struct {
	StartDate string  `conf:"start_date,回测开始日期"`
	EndDate   string  `conf:"end_date,回测结束日期"`
	Fee       float64 `conf:"fee,交易手续费率"`
	Slippage  float64 `conf:"slippage,滑点损失"`
}

// AppConfig 全局配置实例
var AppConfig = &Config{
	CEX: CEXConfig{
		Binance: BinanceConfig{
			APIKey:        "",
			SecretKey:     "",
			BaseURL:       "https://api.binance.com",
			Timeout:       10,
			EnableTrading: false,
			ReadOnly:      true,
			Database:      database.GetDefaultDatabaseConfig("tradingbot_binance"),
		},
	},
	Trading: TradingConfig{
		Symbol:              "", // 通过命令行参数设置
		Timeframe:           "4h",
		InitialCapital:      10000.0,
		Mode:                "backtest",
		MaxPositions:        1,
		PositionSizePercent: 0.95,
		MinTradeAmount:      10.0,
		KlineConfig: KlineDataConfig{
			DefaultLimit:     500,  // 币安默认值
			TimeZone:         "0",  // UTC时区
			EnableHistorical: true, // 启用历史数据获取
		},
	},
	Strategy: StrategyConfig{
		Name: "bollinger_bands",
		Parameters: BollingerBandsParameters{
			Period:              20,   // 布林道计算周期
			Multiplier:          2.0,  // 标准差倍数
			PositionSizePercent: 0.95, // 仓位比例95%
			MinTradeAmount:      10.0, // 最小交易10 USDT
		},
	},
	Backtest: BacktestConfig{
		StartDate: "2023-01-01",
		EndDate:   "2023-12-31",
		Fee:       0.001,  // 0.1%
		Slippage:  0.0001, // 0.01%
	},
	Symbols: []SymbolInfo{
		{Symbol: "BTCUSDT", BaseAsset: "BTC", QuoteAsset: "USDT"},
		{Symbol: "ETHUSDT", BaseAsset: "ETH", QuoteAsset: "USDT"},
		{Symbol: "WIFUSDT", BaseAsset: "WIF", QuoteAsset: "USDT"},
	},
}

// 在包的 init() 函数中注册配置
func init() {
	configs.Unmarshal(AppConfig)
}

// Validate 验证配置
func (c *Config) Validate() error {
	// 注意：交易对现在通过命令行参数设置，这里不验证

	// 验证时间周期
	_, err := timeframes.ParseTimeframe(c.Trading.Timeframe)
	if err != nil {
		return fmt.Errorf("invalid timeframe: %w", err)
	}

	// 验证资金
	if c.Trading.InitialCapital <= 0 {
		return fmt.Errorf("initial capital must be positive")
	}

	// 验证交易模式
	validModes := []string{"live", "paper", "backtest"}
	validMode := false
	for _, mode := range validModes {
		if c.Trading.Mode == mode {
			validMode = true
			break
		}
	}
	if !validMode {
		return fmt.Errorf("invalid trading mode: %s", c.Trading.Mode)
	}

	// 验证仓位百分比
	if c.Trading.PositionSizePercent <= 0 || c.Trading.PositionSizePercent > 1 {
		return fmt.Errorf("position size percent must be between 0 and 1")
	}

	// 验证策略名称
	if c.Strategy.Name == "" {
		return fmt.Errorf("strategy name cannot be empty")
	}

	// 验证回测日期
	if c.Trading.Mode == "backtest" {
		_, err := time.Parse("2006-01-02", c.Backtest.StartDate)
		if err != nil {
			return fmt.Errorf("invalid start date format: %s", c.Backtest.StartDate)
		}

		_, err = time.Parse("2006-01-02", c.Backtest.EndDate)
		if err != nil {
			return fmt.Errorf("invalid end date format: %s", c.Backtest.EndDate)
		}
	}

	return nil
}

// ValidateWithSymbol 验证配置（包含交易对）
func (c *Config) ValidateWithSymbol() error {
	// 验证交易对
	if c.Trading.Symbol == "" {
		return fmt.Errorf("trading symbol cannot be empty")
	}

	// 调用基础验证
	return c.Validate()
}

// GetTimeframe 获取时间周期
func (c *Config) GetTimeframe() (timeframes.Timeframe, error) {
	return timeframes.ParseTimeframe(c.Trading.Timeframe)
}

// GetStartTime 获取回测开始时间
func (c *Config) GetStartTime() (time.Time, error) {
	return time.Parse("2006-01-02", c.Backtest.StartDate)
}

// GetEndTime 获取回测结束时间
func (c *Config) GetEndTime() (time.Time, error) {
	return time.Parse("2006-01-02", c.Backtest.EndDate)
}

// GetInitialCapital 获取初始资金
func (c *Config) GetInitialCapital() decimal.Decimal {
	return decimal.NewFromFloat(c.Trading.InitialCapital)
}

// GetFee 获取手续费率
func (c *Config) GetFee() decimal.Decimal {
	return decimal.NewFromFloat(c.Backtest.Fee)
}

// GetSlippage 获取滑点
func (c *Config) GetSlippage() decimal.Decimal {
	return decimal.NewFromFloat(c.Backtest.Slippage)
}

// IsLiveMode 是否为实盘模式
func (c *Config) IsLiveMode() bool {
	return c.Trading.Mode == "live"
}

// IsPaperMode 是否为模拟交易模式
func (c *Config) IsPaperMode() bool {
	return c.Trading.Mode == "paper"
}

// IsBacktestMode 是否为回测模式
func (c *Config) IsBacktestMode() bool {
	return c.Trading.Mode == "backtest"
}

// GetStrategyParams 获取策略参数
func (c *Config) GetStrategyParams() map[string]interface{} {
	params := make(map[string]interface{})
	params["period"] = c.Strategy.Parameters.Period
	params["multiplier"] = c.Strategy.Parameters.Multiplier
	params["position_size_percent"] = c.Strategy.Parameters.PositionSizePercent
	params["min_trade_amount"] = c.Strategy.Parameters.MinTradeAmount
	return params
}

// UpdateStrategyParams 更新策略参数
func (c *Config) UpdateStrategyParams(params map[string]interface{}) {
	if period, ok := params["period"]; ok {
		if p, ok := period.(int); ok {
			c.Strategy.Parameters.Period = p
		}
	}
	if multiplier, ok := params["multiplier"]; ok {
		if m, ok := multiplier.(float64); ok {
			c.Strategy.Parameters.Multiplier = m
		}
	}
	if positionSize, ok := params["position_size_percent"]; ok {
		if ps, ok := positionSize.(float64); ok {
			c.Strategy.Parameters.PositionSizePercent = ps
		}
	}
	if minTrade, ok := params["min_trade_amount"]; ok {
		if mt, ok := minTrade.(float64); ok {
			c.Strategy.Parameters.MinTradeAmount = mt
		}
	}
}
