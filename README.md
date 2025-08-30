# 🤖 交易机器人

基于Go语言开发的加密货币交易机器人，支持布林道策略、PostgreSQL数据存储和高效回测。

## ✨ 功能特性

### 🔄 核心功能
- **币安API集成**: 支持现货交易、K线数据获取
- **布林道策略**: 经典技术分析策略实现
- **数据库存储**: PostgreSQL存储历史K线数据
- **高效回测**: 基于历史数据的策略回测
- **实时交易**: 支持模拟和实盘交易
- **风险管理**: 止损止盈、仓位控制

### 📊 数据管理
- **K线数据存储**: 支持多交易对、多时间周期
- **增量同步**: 智能数据同步，避免重复获取
- **回测记录**: 完整的回测历史和交易记录
- **性能优化**: 数据库索引优化，查询高效

### 🛠️ 开发工具
- **命令行工具**: ping测试、K线获取、数据同步
- **配置管理**: 基于go-config的配置系统
- **日志系统**: 基于go-log的结构化日志

## 🚀 快速开始

### 1. 环境准备

```bash
# 安装Go (1.19+)
# 安装PostgreSQL (可选，用于数据存储)

# 克隆项目
git clone <repository>
cd tradingbot
```

### 2. 编译项目

```bash
# 安装依赖
go mod tidy

# 编译
make build
```

### 3. 配置设置

```bash
# 生成默认配置
./bin/tradingbot --help

# 编辑配置文件
cp config.json.default config.json
# 修改config.json中的API密钥等配置
```

### 4. 测试连接

```bash
# 测试币安API连接
make ping

# 测试K线数据获取
make kline
```

### 5. 运行回测

```bash
# 运行布林道策略回测
./bin/tradingbot bollinger
```

## 📋 命令使用

### 基础命令

```bash
# 查看帮助
./bin/tradingbot --help

# 测试API连接
./bin/tradingbot ping -v

# 获取K线数据
./bin/tradingbot kline -s BTCUSDT -i 4h -l 10 -v

# 查看支持的交易对
./bin/tradingbot bollinger --list
```

### 布林道策略回测

```bash
# 使用默认参数回测BTCUSDT
./bin/tradingbot bollinger -s BTCUSDT

# 指定时间周期回测
./bin/tradingbot bollinger -s ETHUSDT -t 1h

# 指定交易所回测（目前只支持binance）
./bin/tradingbot bollinger -s WIFUSDT -cex binance

# 查看布林道策略帮助
./bin/tradingbot bollinger --help
```

### Makefile快捷命令

```bash
make build      # 编译项目
make ping       # 测试连接
make kline      # 测试K线
make sync       # 同步数据
make clean      # 清理构建文件
```

## ⚙️ 配置说明

### 🎯 核心配置文件 `bin/config.json`

```json
{
  "cex": {
    "binance": {
      "api_key": "",             // API密钥(可选)
      "secret_key": "",          // API私钥(可选)
      "base_url": "https://api.binance.com",
      "timeout": 10,
      "enable_trading": false,   // 是否启用交易
      "read_only": true,         // 只读模式
      "database": {
        "host": "localhost",
        "port": "5432", 
        "user": "tradingbot",
        "password": "tradingbot123",
        "dbname": "tradingbot_binance",
        "sslmode": "disable",
        "max_open_conns": 25,
        "max_idle_conns": 5
      }
    }
  },
  "trading": {
    "symbol": "",                // 通过命令行参数-s指定
    "timeframe": "4h",
    "initial_capital": 10000,    // 初始资金(USDT)
    "mode": "backtest"           // 运行模式: backtest/paper/live
  },
  "strategy": {
    "name": "bollinger_bands",
    "parameters": {
      "stop_loss_percent": 1.0,  // 止损: 1.0=永不止损, 0.05=5%止损
      "take_profit_percent": 0.5 // 止盈: 0.5=50%止盈
    }
  },
  "backtest": {
    "start_date": "2025-03-16",  // 回测开始日期
    "end_date": "2025-08-30",    // 回测结束日期
    "fee": 0.001                 // 手续费: 0.001=0.1%
  },
  "symbols": [
    {"symbol": "BTCUSDT", "base_asset": "BTC", "quote_asset": "USDT"},
    {"symbol": "ETHUSDT", "base_asset": "ETH", "quote_asset": "USDT"},
    {"symbol": "WIFUSDT", "base_asset": "WIF", "quote_asset": "USDT"}
  ]
}
```

### 配置详解

- **cex.binance**: 币安API和数据库配置，每个交易所有独立的数据库
- **trading**: 交易基础配置，交易对通过命令行参数指定
- **strategy**: 策略参数配置，支持止损止盈设置
- **backtest**: 回测相关配置，包含时间范围和手续费
- **symbols**: 支持的交易对列表，用于验证命令行参数

### 🚀 使用方法

```bash
# 查看支持的交易对
./bin/tradingbot bollinger --list

# 回测BTCUSDT (4小时周期，默认使用binance)
./bin/tradingbot bollinger -s BTCUSDT

# 回测ETHUSDT (1小时周期，指定交易所)  
./bin/tradingbot bollinger -s ETHUSDT -t 1h -cex binance

# 查看命令行帮助
./bin/tradingbot bollinger --help
```

### ⚙️ 常用配置修改

#### 修改初始资金
```json
"initial_capital": 50000  // 改为5万USDT
```

#### 修改止损止盈
```json
"stop_loss_percent": 0.05,   // 5%止损
"take_profit_percent": 0.1   // 10%止盈
```

#### 添加新交易对
在配置文件的 `symbols` 数组中添加:
```json
{"symbol": "DOGEUSDT", "base_asset": "DOGE", "quote_asset": "USDT"}
```

### 🗄️ 数据库连接信息

**Binance数据库连接**:
- 主机: localhost:5432
- 用户: tradingbot  
- 密码: tradingbot123
- 数据库: tradingbot_binance

**连接命令**:
```bash
psql -U tradingbot -d tradingbot_binance
```

## 🗄️ 数据库设计

### 核心表结构

1. **klines**: K线数据表（核心）
   - 存储历史K线数据
   - 支持多交易对、多时间周期
   - 唯一约束防止重复数据

2. **backtest_runs**: 回测记录表
   - 存储回测配置和结果
   - 支持策略参数对比

3. **trades**: 交易记录表
   - 详细的交易历史
   - 关联回测运行记录

4. **sync_status**: 同步状态表
   - 跟踪数据同步进度
   - 支持增量同步

### 数据库初始化

```bash
# 1. 创建数据库
createdb tradingbot

# 2. 执行schema
psql -d tradingbot -f database/schema.sql
```

## 📈 策略说明

### 布林道策略

布林道（Bollinger Bands）是一种技术分析工具，包含三条线：
- **中轨**: 移动平均线（默认20期）
- **上轨**: 中轨 + 2倍标准差
- **下轨**: 中轨 - 2倍标准差

#### 交易逻辑
- **买入信号**: 价格触及下轨时买入
- **卖出信号**: 价格触及上轨时卖出
- **风险控制**: 支持止损止盈和冷却期

#### 参数配置
```json
{
  "period": 20,                    // 计算周期
  "multiplier": 2.0,              // 标准差倍数
  "position_size_percent": 0.95,   // 仓位比例
  "stop_loss_percent": 0.05,      // 止损比例
  "take_profit_percent": 0.1,     // 止盈比例
  "cooldown_bars": 3              // 冷却期
}
```

## 🔧 开发指南

### 项目结构

```
tradingbot/
├── src/
│   ├── main/           # 程序入口
│   ├── config/         # 配置管理
│   ├── binance/        # 币安API客户端
│   ├── database/       # 数据库操作
│   ├── strategies/     # 交易策略
│   ├── backtest/       # 回测引擎
│   ├── trading/        # 交易系统
│   ├── indicators/     # 技术指标
│   ├── timeframes/     # 时间周期
│   └── cmd/           # 命令行工具
├── database/          # 数据库schema
├── bin/              # 编译输出
└── config.json       # 配置文件
```

### 添加新策略

1. 在`src/strategies/`目录下创建新策略文件
2. 实现`Strategy`接口
3. 在配置中添加策略参数
4. 注册到交易系统

### 扩展功能

- 添加新的技术指标
- 支持更多交易所
- 实现更复杂的风险管理
- 添加Web界面

## 🏗️ 策略架构设计

### 协议分离架构

新的策略架构采用**协议分离**的设计理念，将交易策略拆分为四个独立的协议：

1. **EntryStrategy** - 入场策略协议
2. **ExitStrategy** - 出场策略协议  
3. **RiskManagementStrategy** - 风险管理协议
4. **PositionSizingStrategy** - 仓位管理协议

### 设计原则

#### 单一职责原则
每个协议只负责一个特定的功能：
- 入场策略只关心何时买入
- 出场策略只关心何时卖出
- 风险管理只关心止损止盈
- 仓位管理只关心买卖多少

#### 组合优于继承
通过`CompositeStrategy`组合不同的策略协议，而不是通过继承实现复杂策略。

### 架构图

```
┌─────────────────────────────────────────────────────────────┐
│                    CompositeStrategy                        │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────┐ │
│  │EntryStrategy│ │ExitStrategy │ │RiskStrategy │ │SizeStrat│ │
│  └─────────────┘ └─────────────┘ └─────────────┘ └─────────┘ │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
                    ┌─────────────────┐
                    │ TradingEngine   │
                    └─────────────────┘
                              │
                              ▼
                    ┌─────────────────┐
                    │   Executor      │
                    └─────────────────┘
```

### 协议定义

#### EntryStrategy 入场策略协议
```go
type EntryStrategy interface {
    ShouldEnter(ctx context.Context, kline *binance.KlineData, portfolio *executor.Portfolio) (*EnhancedSignal, error)
    GetName() string
    GetParams() map[string]interface{}
}
```

**示例实现**：
- `BollingerEntryStrategy` - 布林道下轨触及入场
- `MACDEntryStrategy` - MACD金叉入场
- `RSIEntryStrategy` - RSI超卖入场

#### ExitStrategy 出场策略协议
```go
type ExitStrategy interface {
    ShouldExit(ctx context.Context, kline *binance.KlineData, portfolio *executor.Portfolio) (*EnhancedSignal, error)
    GetName() string
    GetParams() map[string]interface{}
}
```

**示例实现**：
- `BollingerExitStrategy` - 布林道上轨触及出场
- `MACDExitStrategy` - MACD死叉出场
- `TimeBasedExitStrategy` - 时间到期出场

#### RiskManagementStrategy 风险管理协议
```go
type RiskManagementStrategy interface {
    CheckRisk(ctx context.Context, kline *binance.KlineData, portfolio *executor.Portfolio) (*EnhancedSignal, error)
    GetName() string
    GetParams() map[string]interface{}
}
```

**示例实现**：
- `StopLossTakeProfitStrategy` - 固定百分比止损止盈
- `TrailingStopStrategy` - 移动止损
- `VaRRiskStrategy` - VaR风险管理

#### PositionSizingStrategy 仓位管理协议
```go
type PositionSizingStrategy interface {
    CalculateSize(ctx context.Context, signal *EnhancedSignal, portfolio *executor.Portfolio) (float64, error)
    GetName() string
    GetParams() map[string]interface{}
}
```

**示例实现**：
- `FixedPercentageStrategy` - 固定百分比仓位
- `KellyStrategy` - 凯利公式仓位
- `VolatilityBasedStrategy` - 基于波动率的仓位

### 使用示例

#### 创建组合策略
```go
// 创建各个子策略
entryStrategy := entry.NewBollingerEntryStrategy(20, 2.0, 3)
exitStrategy := exit.NewBollingerExitStrategy(20, 2.0)
riskStrategy := risk.NewStopLossTakeProfitStrategy(0.05, 0.1)
sizingStrategy := sizing.NewFixedPercentageStrategy(0.95, 10.0)

// 组合策略
compositeStrategy := strategy.NewCompositeStrategy(
    "MyBollingerStrategy",
    entryStrategy,
    exitStrategy,
    riskStrategy,
    sizingStrategy,
)
```

#### 灵活组合不同策略
```go
// 布林道入场 + MACD出场 + 移动止损 + 凯利仓位
strategy1 := strategy.NewCompositeStrategy(
    "BollingerMACDStrategy",
    entry.NewBollingerEntryStrategy(20, 2.0, 3),
    exit.NewMACDExitStrategy(12, 26, 9),
    risk.NewTrailingStopStrategy(0.05),
    sizing.NewKellyStrategy(0.25),
)
```

### 架构优势

1. **高度模块化** - 每个协议独立开发和测试
2. **灵活组合** - 可以任意组合不同的策略实现
3. **易于扩展** - 添加新策略非常简单
4. **职责清晰** - 每个协议的职责明确
5. **可测试性** - 每个协议可以独立进行单元测试

## 📊 性能优化

### 数据库优化
- 使用索引加速查询
- 批量操作提高效率
- 连接池管理连接

### 回测优化
- 数据预加载
- 并行计算
- 内存优化

## ⚠️ 风险提示

1. **投资风险**: 加密货币交易存在高风险，可能导致资金损失
2. **技术风险**: 软件可能存在bug，请充分测试后使用
3. **API风险**: 请妥善保管API密钥，建议使用只读权限
4. **网络风险**: 网络延迟可能影响交易执行

## 📄 许可证

本项目采用MIT许可证，详见LICENSE文件。

## 🤝 贡献

欢迎提交Issue和Pull Request来改进项目。

## 📞 支持

如有问题，请通过以下方式联系：
- 提交GitHub Issue
- 发送邮件至项目维护者