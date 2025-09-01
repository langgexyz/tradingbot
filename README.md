# 🤖 交易机器人

基于Go语言开发的加密货币交易机器人，支持布林道策略、PostgreSQL数据存储和高效回测。

## ✨ 功能特性

### 🔄 核心功能
- **币安API集成**: 支持现货交易、K线数据获取
- **布林道策略**: 经典技术分析策略实现
- **数据库存储**: PostgreSQL存储历史K线数据
- **高效回测**: 基于历史数据的策略回测
- **实时交易**: 支持模拟和币安实盘交易
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
./bin/tradingbot bollinger-backtest -base DOGE -quote USDT -start 2024-01-01 -end 2024-06-30 -t 4h -capital 10000 -sell-strategy trailing_5
```

### 6. 配置实盘交易（可选）

**⚠️ 风险提示：实盘交易涉及真实资金，请谨慎操作！**

```bash
# 1. 配置币安API密钥（编辑 bin/config.json）
{
  "tradingbot/src/cex/binance:Config": {
    "APIKey": "你的API密钥",
    "SecretKey": "你的Secret密钥",
    "EnableTrading": true,
    "ReadOnly": false
  }
}

# 2. 测试连接
./bin/tradingbot bollinger-backtest -base BTC -quote USDT -start 2024-01-01 -end 2024-01-02 -t 1h -capital 100

# 3. 启动实盘交易
./bin/tradingbot bollinger-live -base DOGE -quote USDT -t 4h -sell-strategy conservative
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

### 布林道策略交易

```bash
# 回测命令
./bin/tradingbot bollinger-backtest -base DOGE -quote USDT -start 2024-01-01 -end 2024-06-30 -t 4h -capital 10000 -sell-strategy trailing_5

# 实盘交易命令
./bin/tradingbot bollinger-live -base DOGE -quote USDT -t 4h -sell-strategy conservative

# 支持的交易策略
-sell-strategy conservative   # 保守策略 (15%固定止盈)
-sell-strategy moderate      # 中等策略 (20%固定止盈) - 默认
-sell-strategy aggressive    # 激进策略 (30%固定止盈)
-sell-strategy trailing_5    # 5%跟踪止损 (15%后启动)
-sell-strategy trailing_10   # 10%跟踪止损 (20%后启动)
-sell-strategy combo_smart   # 智能组合策略

# 查看命令帮助
./bin/tradingbot bollinger-backtest --help
./bin/tradingbot bollinger-live --help
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
  "tradingbot/src/cex/binance:Config": {
    "APIKey": "",                // 币安API密钥
    "SecretKey": "",             // 币安Secret密钥
    "BaseURL": "https://api.binance.com",
    "Timeout": 10,
    "EnableTrading": false,      // 实盘交易开关
    "ReadOnly": true,            // 只读模式
    "Fee": 0.001,               // 交易手续费率
    "DBName": "tradingbot_binance"
  },
  "tradingbot/src/database:DatabaseConfig": {
    "Host": "localhost",
    "Port": "5432",
    "User": "tradingbot",
    "Password": "",
    "DBName": "tradingbot",
    "SSLMode": "disable",
    "MaxOpenConns": 25,
    "MaxIdleConns": 5
  },
  "tradingbot/src/trading:TradingConfig": {
    "Timeframe": "4h",
    "MaxPositions": 1,
    "PositionSizePercent": 0.95,
    "MinTradeAmount": 10
  }
}
```

### 配置详解

- **binance:Config**: 币安API配置，包含密钥、交易开关等
- **database:DatabaseConfig**: PostgreSQL数据库连接配置  
- **trading:TradingConfig**: 交易基础配置，仓位大小、最小交易金额等

### ⚙️ 常用配置修改

#### 修改仓位大小
```json
"PositionSizePercent": 0.5  // 改为50%仓位
```

#### 修改最小交易金额
```json
"MinTradeAmount": 50  // 最小50 USDT
```

#### 启用实盘交易
```json
"EnableTrading": true,
"ReadOnly": false
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

## 💰 币安实盘交易

### 🚨 重要安全提醒

**⚠️  使用真实资金进行交易前，请仔细阅读本指南！**
**⚠️  建议先使用小额资金进行测试！**
**⚠️  交易有风险，可能导致资金损失！**

### 🔑 配置币安API密钥

#### 1. 创建币安API密钥
1. 登录 [币安官网](https://www.binance.com)
2. 进入 **账户管理** → **API管理**
3. 创建新的API密钥
4. **重要**：只启用以下权限：
   - ✅ **读取** (Read)
   - ✅ **现货交易** (Spot Trading)
   - ❌ **合约交易** (Futures Trading) - 不需要
   - ❌ **提币** (Withdraw) - 为了安全，不启用
5. 设置IP白名单（推荐）
6. 保存API Key和Secret Key

#### 2. 配置交易系统

编辑 `bin/config.json` 文件：

```json
{
  "tradingbot/src/cex/binance:Config": {
    "APIKey": "你的币安API密钥",
    "SecretKey": "你的币安Secret密钥",
    "BaseURL": "https://api.binance.com",
    "Timeout": 10,
    "EnableTrading": true,        // 启用实盘交易
    "ReadOnly": false,           // 关闭只读模式
    "Fee": 0.001,
    "DBName": "tradingbot_binance"
  }
}
```

**安全建议**：
- 测试阶段设置 `"EnableTrading": false, "ReadOnly": true`
- 生产环境设置 `"EnableTrading": true, "ReadOnly": false`

### 🚀 启动实盘交易

#### 1. 测试连接
```bash
# 测试API连接
./bin/tradingbot bollinger-backtest -base BTC -quote USDT -start 2024-01-01 -end 2024-01-02 -t 1h -capital 100
```

成功配置后应显示：
```
✓ Connected to CEX API
🗄️ Connecting to binance database... connected!
```

#### 2. 启动实盘交易
```bash
# 启动DOGE/USDT实盘交易（保守策略）
./bin/tradingbot bollinger-live \
  -base DOGE \
  -quote USDT \
  -t 4h \
  -sell-strategy conservative
```

#### 3. 支持的交易策略

| 策略 | 风险级别 | 预期收益 | 适用场景 |
|------|----------|----------|----------|
| `conservative` | 🟢 低 | 34.57% | 稳健投资 |
| `moderate` | 🟡 中 | 43.83% | 平衡投资 |
| `aggressive` | 🟠 高 | 83.22% | 激进投资 |
| `trailing_5` | 🔴 最高 | 128.40% | 牛市趋势 |
| `trailing_10` | 🟠 高 | 106.69% | 震荡市场 |

#### 4. 实时监控

交易启动后会显示：
```
🔴 Starting live trading...
✓ Connected to CEX API
📊 投资组合状态: DOGE余额=1000, USDT余额=500, 当前价格=0.08, 总价值=580
🔵 生成买入限价单: quantity=12500, price=0.078
✅ 实盘买入订单成功: OrderID=123456789
```

使用 `Ctrl+C` 安全停止交易。

### 🛡️ 安全最佳实践

#### API安全
- ✅ 设置IP白名单
- ✅ 定期更换API密钥
- ✅ 只启用必要权限
- ❌ 永远不要分享API密钥

#### 资金安全
- ✅ 从小额开始测试
- ✅ 设置合理的仓位大小
- ✅ 定期检查交易记录
- ✅ 保持账户资金监控

#### 系统安全
- ✅ 在安全的网络环境运行
- ✅ 使用最新版本的交易系统
- ✅ 定期备份配置文件

### 🔧 故障排除

**API连接失败:**
```
failed to connect to CEX: Binance ping failed
```
- 检查API密钥是否正确
- 检查网络连接和IP白名单
- 确认币安服务状态

**交易权限错误:**
```
This request is not enabled for this account
```
- 检查API权限设置
- 确认启用了现货交易权限

**余额不足:**
```
Account has insufficient balance
```
- 检查USDT余额是否充足
- 确认交易对资产余额

### ✅ 启动检查清单

在开始实盘交易前，请确认：

- [ ] 已创建币安API密钥
- [ ] 已正确配置 `bin/config.json`
- [ ] 已测试API连接成功
- [ ] 已了解选择的交易策略
- [ ] 已设置合理的资金规模
- [ ] 已准备好监控交易过程
- [ ] 已了解如何停止交易
- [ ] 已备份重要配置文件

## ⚠️ 风险提示

**实盘交易特别风险提示：**
1. **资金损失风险**: 实盘交易使用真实资金，可能导致全部本金损失
2. **策略风险**: 任何交易策略都无法保证盈利，历史收益不代表未来表现
3. **技术风险**: 软件Bug、网络中断、API故障等可能导致交易执行异常
4. **市场风险**: 加密货币市场波动极大，价格可能急剧下跌
5. **操作风险**: 错误配置、误操作可能导致意外损失

**通用风险：**
1. **投资风险**: 加密货币交易存在高风险，仅投入您能承受损失的资金
2. **技术风险**: 软件可能存在bug，请在小额资金上充分测试后再使用
3. **API风险**: 请妥善保管API密钥，建议设置IP白名单和最小权限
4. **网络风险**: 网络延迟、断线可能影响交易执行

**免责声明**: 本软件仅供学习和研究使用，使用者须自行承担所有交易风险和损失。

## 📄 许可证

本项目采用MIT许可证，详见LICENSE文件。

## 🤝 贡献

欢迎提交Issue和Pull Request来改进项目。

## 📞 支持

如有问题，请通过以下方式联系：
- 提交GitHub Issue
- 发送邮件至项目维护者