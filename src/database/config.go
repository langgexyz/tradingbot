package database

import (
	"github.com/xpwu/go-config/configs"
)

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Host         string `json:"host"`           // 数据库主机地址
	Port         string `json:"port"`           // 数据库端口
	User         string `json:"user"`           // 数据库用户名
	Password     string `json:"password"`       // 数据库密码
	DBName       string `json:"dbname"`         // 数据库名称
	SSLMode      string `json:"sslmode"`        // SSL模式
	MaxOpenConns int    `json:"max_open_conns"` // 最大连接数
	MaxIdleConns int    `json:"max_idle_conns"` // 最大空闲连接数
}

// GlobalDatabaseConfig 全局数据库配置实例
var GlobalDatabaseConfig = DatabaseConfig{
	Host:         "localhost",
	Port:         "5432",
	User:         "tradingbot",
	Password:     "",
	DBName:       "tradingbot",
	SSLMode:      "disable",
	MaxOpenConns: 25,
	MaxIdleConns: 5,
}

// GetDatabaseConfigForCEX 获取指定CEX的数据库配置
func GetDatabaseConfigForCEX(cexDBName string) DatabaseConfig {
	config := GlobalDatabaseConfig
	config.DBName = cexDBName
	return config
}

// GetDefaultDatabaseConfig 获取默认数据库配置（保持向后兼容）
func GetDefaultDatabaseConfig(dbname string) DatabaseConfig {
	return GetDatabaseConfigForCEX(dbname)
}

func init() {
	configs.Unmarshal(&GlobalDatabaseConfig)
}
