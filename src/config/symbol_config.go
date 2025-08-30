package config

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/shopspring/decimal"
)

// SymbolConfig 交易对配置
type SymbolConfig struct {
	Symbol      string          `json:"symbol"`
	BaseAsset   string          `json:"base_asset"`
	QuoteAsset  string          `json:"quote_asset"`
	Status      string          `json:"status"`
	MinQty      decimal.Decimal `json:"min_qty"`
	MaxQty      decimal.Decimal `json:"max_qty"`
	StepSize    decimal.Decimal `json:"step_size"`
	MinPrice    decimal.Decimal `json:"min_price"`
	MaxPrice    decimal.Decimal `json:"max_price"`
	TickSize    decimal.Decimal `json:"tick_size"`
	MinNotional decimal.Decimal `json:"min_notional"`
}

// SymbolConfigManager 交易对配置管理器
type SymbolConfigManager struct {
	symbols map[string]*SymbolConfig
	mu      sync.RWMutex
}

// NewSymbolConfigManager 创建交易对配置管理器
func NewSymbolConfigManager() *SymbolConfigManager {
	return &SymbolConfigManager{
		symbols: make(map[string]*SymbolConfig),
	}
}

// LoadFromFile 从文件加载配置
func (m *SymbolConfigManager) LoadFromFile(filePath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read symbol config file: %w", err)
	}

	var symbols []*SymbolConfig
	if err := json.Unmarshal(data, &symbols); err != nil {
		return fmt.Errorf("failed to unmarshal symbol config: %w", err)
	}

	// 重新构建索引
	m.symbols = make(map[string]*SymbolConfig)
	for _, symbol := range symbols {
		m.symbols[symbol.Symbol] = symbol
	}

	return nil
}

// SaveToFile 保存配置到文件
func (m *SymbolConfigManager) SaveToFile(filePath string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var symbols []*SymbolConfig
	for _, symbol := range m.symbols {
		symbols = append(symbols, symbol)
	}

	data, err := json.MarshalIndent(symbols, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal symbol config: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write symbol config file: %w", err)
	}

	return nil
}

// IsSupported 检查交易对是否支持
func (m *SymbolConfigManager) IsSupported(symbol string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	config, exists := m.symbols[symbol]
	return exists && config.Status == "TRADING"
}

// GetSymbolConfig 获取交易对配置
func (m *SymbolConfigManager) GetSymbolConfig(symbol string) (*SymbolConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	config, exists := m.symbols[symbol]
	if !exists {
		return nil, fmt.Errorf("trading pair %s is not configured", symbol)
	}

	if config.Status != "TRADING" {
		return nil, fmt.Errorf("trading pair %s is not active (status: %s)", symbol, config.Status)
	}

	return config, nil
}

// GetSupportedSymbols 获取所有支持的交易对
func (m *SymbolConfigManager) GetSupportedSymbols() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var symbols []string
	for symbol, config := range m.symbols {
		if config.Status == "TRADING" {
			symbols = append(symbols, symbol)
		}
	}

	return symbols
}

// AddSymbol 添加交易对配置
func (m *SymbolConfigManager) AddSymbol(config *SymbolConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.symbols[config.Symbol] = config
}

// RemoveSymbol 移除交易对配置
func (m *SymbolConfigManager) RemoveSymbol(symbol string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.symbols, symbol)
}

// UpdateSymbolStatus 更新交易对状态
func (m *SymbolConfigManager) UpdateSymbolStatus(symbol, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	config, exists := m.symbols[symbol]
	if !exists {
		return fmt.Errorf("trading pair %s not found", symbol)
	}

	config.Status = status
	return nil
}

// 全局配置管理器实例
var GlobalSymbolConfigManager = NewSymbolConfigManager()
