package binance

import (
	"tradingbot/src/cex"
)

// BinanceFactory Binance工厂实现
type BinanceFactory struct{}

// CreateClient 创建Binance客户端
func (f *BinanceFactory) CreateClient() cex.CEXClient {
	config := &ConfigValue
	return NewClient(config.APIKey, config.SecretKey)
}

// 注册Binance工厂
func init() {
	cex.RegisterCEXFactory("binance", &BinanceFactory{})
}
