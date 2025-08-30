package database

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Host         string `conf:"host,数据库主机地址 - 默认localhost"`
	Port         string `conf:"port,数据库端口 - 默认5432"`
	User         string `conf:"user,数据库用户名"`
	Password     string `conf:"password,数据库密码"`
	DBName       string `conf:"dbname,数据库名称"`
	SSLMode      string `conf:"sslmode,SSL模式 - disable/require/verify-ca/verify-full"`
	MaxOpenConns int    `conf:"max_open_conns,最大连接数 - 默认25"`
	MaxIdleConns int    `conf:"max_idle_conns,最大空闲连接数 - 默认5"`
}

// GetDefaultDatabaseConfig 获取默认数据库配置
func GetDefaultDatabaseConfig(dbname string) DatabaseConfig {
	return DatabaseConfig{
		Host:         "localhost",
		Port:         "5432",
		User:         "tradingbot",
		Password:     "",
		DBName:       dbname,
		SSLMode:      "disable",
		MaxOpenConns: 25,
		MaxIdleConns: 5,
	}
}
