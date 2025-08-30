package cex

import (
	"fmt"
)

// CEXFactory CEX工厂接口（简化版）
type CEXFactory interface {
	CreateClient() CEXClient
}

// CEXFactoryRegistry CEX工厂注册表
var CEXFactoryRegistry = make(map[string]CEXFactory)

// RegisterCEXFactory 注册CEX工厂
func RegisterCEXFactory(name string, factory CEXFactory) {
	CEXFactoryRegistry[name] = factory
}

// CreateCEXClient 创建CEX客户端
func CreateCEXClient(cexName string) (CEXClient, error) {
	factory, exists := CEXFactoryRegistry[cexName]
	if !exists {
		return nil, fmt.Errorf("unsupported CEX: %s", cexName)
	}

	// 创建客户端，所有信息都从客户端获取
	client := factory.CreateClient()

	return client, nil
}

// GetSupportedCEXes 获取支持的CEX列表
func GetSupportedCEXes() []string {
	var cexes []string
	for name := range CEXFactoryRegistry {
		cexes = append(cexes, name)
	}
	return cexes
}
