# 布林道现货交易策略系统

基于Go语言开发的币安现货交易策略系统，实现布林道（Bollinger Bands）交易策略，支持多时间刻度和回测功能。

## 🚀 功能特性

- **布林道交易策略**: 下轨买入，上轨卖出的经典策略
- **多时间刻度支持**: 1m, 5m, 15m, 30m, 1h, 2h, 4h, 6h, 12h, 1d, 3d, 1w, 1M
- **完整回测系统**: 历史数据回测，性能分析，风险指标计算
- **币安API集成**: 支持实时数据获取和交易执行
- **风险控制**: 止损、止盈、仓位管理、资金管理
- **灵活配置**: JSON配置文件，支持环境变量

## 📦 快速开始

### 1. 安装依赖

```bash
go mod tidy
```

### 2. 编译系统

```bash
# 使用Makefile编译
make build

# 或手动编译
go build -o tradingbot src/main/main.go
```

### 3. 配置系统

```bash
# 复制配置模板
cp config.template.json config.json

# 编辑配置文件（修改API密钥等参数）
vim config.json
```

### 4. 运行回测

```bash
./tradingbot bollinger -c config.json
```

## 🔧 配置说明

项目使用统一的配置文件系统：

- `config.json` - **主配置文件**（程序运行时读取）
- `config.template.json` - 配置模板文件（包含详细说明）

### 主要配置项

#### Binance API 配置
```json
"binance": {
  "api_key": "YOUR_API_KEY_HERE",        // 从币安获取的API密钥
  "secret_key": "YOUR_SECRET_KEY_HERE",  // 从币安获取的密钥
  "testnet": true                        // 是否使用测试网
}
```

#### 交易配置
```json
"trading": {
  "symbol": "BTCUSDT",                   // 交易对
  "timeframe": "4h",                     // K线周期
  "initial_capital": 10000,              // 初始资金
  "mode": "backtest",                    // 运行模式
  "max_positions": 1,                    // 最大持仓数
  "position_size_percent": 0.95,         // 单次交易资金比例
  "min_trade_amount": 10                 // 最小交易金额
}
```

#### 策略配置
```json
"strategy": {
  "name": "bollinger_bands",
  "parameters": {
    "period": 20,                        // 布林道计算周期
    "multiplier": 2.0,                   // 标准差倍数
    "stop_loss_percent": 0.05,           // 止损比例
    "take_profit_percent": 0.10,         // 止盈比例
    "cooldown_bars": 3                   // 冷却K线数
  }
}
```

#### 风险管理
```json
"risk": {
  "max_daily_loss": 0.05,               // 最大日亏损
  "max_total_loss": 0.20,               // 最大总亏损
  "stop_loss_percent": 0.05,            // 止损百分比
  "take_profit_percent": 0.10,          // 止盈百分比
  "max_drawdown_percent": 0.15,         // 最大回撤
  "cooldown_minutes": 60                // 冷却时间（分钟）
}
```

### 设置API密钥

可以通过环境变量设置API密钥，避免在配置文件中暴露：

```bash
export BINANCE_API_KEY="your_api_key"
export BINANCE_SECRET_KEY="your_secret_key"
export BINANCE_TESTNET="true"
```

## 📊 策略说明

### 布林道策略逻辑

1. **买入信号**: 价格触及或跌破布林道下轨
2. **卖出信号**: 价格触及或突破布林道上轨
3. **止损**: 亏损超过设定百分比时平仓
4. **止盈**: 盈利超过设定百分比时平仓

### 策略参数

- `period`: 布林道计算周期（默认20）
- `multiplier`: 标准差倍数（默认2.0）
- `position_size_percent`: 仓位大小百分比（默认0.95）
- `stop_loss_percent`: 止损百分比（默认0.05）
- `take_profit_percent`: 止盈百分比（默认0.10）
- `cooldown_bars`: 交易冷却期K线数量（默认3）

### 参数调优建议

| 参数 | 建议值 | 说明 |
|------|--------|------|
| period | 20 | 经典设置，适用于4h周期 |
| multiplier | 2.0 | 标准差倍数，2.0覆盖95%价格 |
| stop_loss_percent | 0.03-0.08 | 根据波动性调整 |
| take_profit_percent | 0.08-0.15 | 风险收益比1:2 |

## ⚙️ 支持的时间刻度

| 时间刻度 | 说明 | 建议用途 |
|---------|------|----------|
| 1m, 3m, 5m | 短周期 | 高频交易、短线策略 |
| 15m, 30m | 中短周期 | 日内交易 |
| 1h, 2h, 4h | 中长周期 | 短中线策略（推荐） |
| 6h, 12h, 1d | 长周期 | 中长线策略 |
| 3d, 1w, 1M | 超长周期 | 长线投资 |

## 📈 回测结果

回测完成后会显示详细的性能指标：

```
📊 BACKTEST RESULTS
============================================================
Strategy: Bollinger Bands Strategy
Symbol: BTCUSDT
Timeframe: 4h
Initial Capital: $10000.00

📈 PERFORMANCE METRICS
------------------------------
Total Return: 25.50%        # 总收益率
Annualized Return: 28.75%   # 年化收益率
Max Drawdown: 8.20%         # 最大回撤
Sharpe Ratio: 1.2500        # 夏普比率（>1为优秀）

📊 TRADING STATISTICS
------------------------------
Total Trades: 45            # 总交易次数
Winning Trades: 28          # 盈利交易
Losing Trades: 17           # 亏损交易
Win Rate: 62.22%            # 胜率
Total P&L: $2550.00         # 总盈亏
Total Commission: $125.50   # 总手续费
```

### 关键指标说明

- **Sharpe Ratio > 1**: 策略表现优秀
- **Max Drawdown < 15%**: 风险可控
- **Win Rate > 55%**: 胜率较高
- **总收益率 > 年化无风险利率**: 有投资价值

### 结果文件

回测结果会自动保存到 `./backtest_results/` 目录，包含：
- 详细的交易记录
- 权益曲线数据
- 策略配置参数
- 性能统计指标

## 🛡️ 风险控制

### 内置风险控制机制

1. **止损止盈**: 自动平仓保护
2. **仓位控制**: 限制单次交易仓位大小
3. **最大回撤**: 监控账户最大回撤
4. **交易冷却**: 避免过度频繁交易
5. **资金管理**: 最大日/总亏损限制

### 风险提示

⚠️ **重要提醒**:
- 本系统仅供学习和研究使用
- 任何交易策略都存在亏损风险
- 实盘交易前请充分测试和评估
- 建议先在测试网络上验证策略
- 请合理控制仓位，不要投入超过承受能力的资金
- 数字货币市场波动性极高
- 回测结果不等于实盘表现

## 🔧 开发和扩展

### 项目结构

```
src/
├── indicators/          # 技术指标库
│   ├── bollinger_bands.go
│   └── errors.go
├── binance/            # 币安API客户端
│   └── client.go
├── strategies/         # 交易策略
│   └── bollinger_strategy.go
├── backtest/          # 回测引擎
│   └── engine.go
├── timeframes/        # 时间刻度管理
│   └── timeframes.go
├── config/            # 配置管理
│   └── config.go
├── trading/           # 交易系统核心
│   └── trading_system.go
├── cmd/               # 命令行接口
│   └── bollinger_trading.go
└── main/              # 主程序
    └── main.go
```

### 添加新策略

1. 在 `strategies/` 目录下创建新策略文件
2. 实现 `backtest.Strategy` 接口
3. 在 `trading_system.go` 中注册新策略
4. 在 `cmd/` 目录下添加对应的命令

### 添加新指标

1. 在 `indicators/` 目录下创建新指标文件
2. 实现计算逻辑和相关方法
3. 在策略中引用和使用

## 🔨 构建和部署

### 构建命令

```bash
# 构建（推荐）
make build

# 手动构建
go build -o tradingbot src/main/main.go

# 交叉编译Linux版本
make build-linux
```

### 运行模式

#### 回测模式
```bash
./tradingbot bollinger -c config.json
```

#### 实时交易（开发中）
```bash
# 修改配置文件中的mode为"live"
# 注意：这将使用真实资金进行交易！
./tradingbot bollinger -c config.json
```

## 🛠️ 故障排除

### 常见错误

1. **"can't read config file"**
   - 确保配置文件存在且格式正确

2. **"failed to connect to Binance"**
   - 检查网络连接和API密钥

3. **"insufficient data"**
   - 减少回测周期或增加数据获取天数

4. **"invalid timeframe"**
   - 检查时间刻度格式是否正确

### 获取帮助

如果遇到问题，请检查：
1. 配置文件格式是否正确
2. API密钥是否有效
3. 网络连接是否正常
4. 时间范围是否合理

## 📝 最佳实践

1. **策略开发流程**:
   - 历史数据回测 → 纸上交易 → 小资金实盘 → 正式交易

2. **参数优化**:
   - 使用多个时间段验证参数稳定性
   - 避免过度拟合历史数据

3. **风险管理**:
   - 设置合理的止损止盈
   - 分散投资，不要all-in单一币种
   - 定期评估和调整策略

4. **监控和维护**:
   - 定期检查策略表现
   - 市场环境变化时及时调整
   - 保持学习和改进

## 📝 日志

系统会自动生成交易日志，保存在 `./logs/trading.log`：

- 策略信号生成
- 交易执行记录
- 错误和异常信息
- 性能统计数据

## 🔒 安全提示

⚠️ **重要提醒**：
- 不要将包含真实API密钥的 `config.json` 提交到版本控制系统
- 建议将 `config.json` 添加到 `.gitignore` 文件中
- 实盘交易前请务必在测试网环境中充分测试

## 🤝 贡献

欢迎提交问题和功能请求！

## 📄 许可证

本项目仅供学习和研究使用。

---

**注意**: 这是一个学习和研究工具，不建议直接用于生产环境的大额交易。任何投资都有风险，请谨慎使用。