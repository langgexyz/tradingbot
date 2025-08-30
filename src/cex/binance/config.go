package binance

import (
	"github.com/xpwu/go-config/configs"
)

// Config 币安配置
type Config struct {
	APIKey        string  `json:"api_key"`        // API密钥
	SecretKey     string  `json:"secret_key"`     // API私钥
	BaseURL       string  `json:"base_url"`       // API地址
	Timeout       int     `json:"timeout"`        // 请求超时时间(秒)
	EnableTrading bool    `json:"enable_trading"` // 启用交易权限
	ReadOnly      bool    `json:"read_only"`      // 只读模式
	Fee           float64 `json:"fee"`            // 交易手续费率
	DBName        string  `json:"db_name"`        // 数据库名称
}

// ConfigValue 币安配置实例
var ConfigValue = Config{
	APIKey:        "",
	SecretKey:     "",
	BaseURL:       "https://api.binance.com",
	Timeout:       10,
	EnableTrading: false,
	ReadOnly:      true,
	Fee:           0.001, // 币安现货交易手续费0.1%
	DBName:        "tradingbot_binance",
}

func init() {
	configs.Unmarshal(&ConfigValue)
}
