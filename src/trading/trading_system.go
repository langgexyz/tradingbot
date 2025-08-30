package trading

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go-build-stream-gateway-go-server-main/src/backtest"
	"go-build-stream-gateway-go-server-main/src/binance"
	"go-build-stream-gateway-go-server-main/src/config"
	"go-build-stream-gateway-go-server-main/src/strategies"

	"github.com/shopspring/decimal"
)

// TradingSystem 交易系统
type TradingSystem struct {
	config        *config.Config
	binanceClient *binance.Client
	strategy      backtest.Strategy
	backtest      *backtest.Engine
	ctx           context.Context
	cancel        context.CancelFunc
}

// NewTradingSystem 创建新的交易系统
func NewTradingSystem() (*TradingSystem, error) {
	// 使用全局配置
	cfg := config.AppConfig

	// 验证配置
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())

	return &TradingSystem{
		config: cfg,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

// Initialize 初始化系统
func (ts *TradingSystem) Initialize() error {
	// 初始化币安客户端
	ts.binanceClient = binance.NewClient(
		ts.config.Binance.APIKey,
		ts.config.Binance.SecretKey,
		ts.config.Binance.BaseURL,
	)

	// 测试连接（如果不是回测模式）
	if !ts.config.IsBacktestMode() {
		err := ts.binanceClient.Ping(ts.ctx)
		if err != nil {
			return fmt.Errorf("failed to connect to Binance: %w", err)
		}
		fmt.Println("✓ Connected to Binance API")
	}

	// 初始化策略
	err := ts.initializeStrategy()
	if err != nil {
		return fmt.Errorf("failed to initialize strategy: %w", err)
	}

	return nil
}

// initializeStrategy 初始化交易策略
func (ts *TradingSystem) initializeStrategy() error {
	switch ts.config.Strategy.Name {
	case "bollinger_bands":
		strategy := strategies.NewBollingerBandsStrategy()
		err := strategy.SetParams(ts.config.GetStrategyParams())
		if err != nil {
			return fmt.Errorf("failed to set strategy parameters: %w", err)
		}
		ts.strategy = strategy
		fmt.Printf("✓ Initialized Bollinger Bands strategy with params: %+v\n", strategy.GetParams())
	default:
		return fmt.Errorf("unsupported strategy: %s", ts.config.Strategy.Name)
	}

	return nil
}

// RunBacktest 运行回测
func (ts *TradingSystem) RunBacktest() (*backtest.Statistics, error) {
	if !ts.config.IsBacktestMode() {
		return nil, fmt.Errorf("not in backtest mode")
	}

	fmt.Println("🔄 Starting backtest...")

	// 获取时间范围
	startTime, err := ts.config.GetStartTime()
	if err != nil {
		return nil, fmt.Errorf("invalid start time: %w", err)
	}

	endTime, err := ts.config.GetEndTime()
	if err != nil {
		return nil, fmt.Errorf("invalid end time: %w", err)
	}

	// 获取时间周期
	timeframe, err := ts.config.GetTimeframe()
	if err != nil {
		return nil, fmt.Errorf("invalid timeframe: %w", err)
	}

	// 创建回测引擎
	ts.backtest = backtest.NewEngine(
		ts.config.Trading.Symbol,
		timeframe,
		startTime,
		endTime,
		ts.config.GetInitialCapital(),
	)

	// 设置手续费和滑点
	ts.backtest.SetCommission(ts.config.Backtest.Fee)
	ts.backtest.SetSlippage(ts.config.Backtest.Slippage)

	// 设置策略
	ts.backtest.SetStrategy(ts.strategy)

	// 加载历史数据
	fmt.Printf("📊 Loading historical data for %s (%s) from %s to %s...\n",
		ts.config.Trading.Symbol,
		timeframe,
		startTime.Format("2006-01-02"),
		endTime.Format("2006-01-02"),
	)

	err = ts.backtest.LoadData(ts.ctx, ts.binanceClient)
	if err != nil {
		return nil, fmt.Errorf("failed to load historical data: %w", err)
	}

	fmt.Printf("✓ Loaded historical data successfully\n")

	// 运行回测
	fmt.Println("🚀 Running backtest...")
	err = ts.backtest.Run(ts.ctx)
	if err != nil {
		return nil, fmt.Errorf("backtest failed: %w", err)
	}

	// 获取统计结果
	stats := ts.backtest.GetStatistics()

	fmt.Println("✅ Backtest completed")
	return stats, nil
}

// SaveBacktestResults 保存回测结果
func (ts *TradingSystem) SaveBacktestResults(stats *backtest.Statistics, resultsPath string) error {
	// 创建结果目录
	err := os.MkdirAll(resultsPath, 0755)
	if err != nil {
		return fmt.Errorf("failed to create results directory: %w", err)
	}

	// 生成文件名
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("backtest_%s_%s_%s.json",
		ts.config.Trading.Symbol,
		ts.config.Trading.Timeframe,
		timestamp,
	)
	filePath := filepath.Join(resultsPath, filename)

	// 创建完整的结果报告
	report := BacktestReport{
		Config:     ts.config,
		Statistics: stats,
		Trades:     ts.backtest.GetTrades(),
		Timestamp:  time.Now(),
	}

	// 保存为JSON
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal results: %w", err)
	}

	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write results file: %w", err)
	}

	return nil
}

// BacktestReport 回测报告
type BacktestReport struct {
	Config     *config.Config       `json:"config"`
	Statistics *backtest.Statistics `json:"statistics"`
	Trades     []*backtest.Trade    `json:"trades"`
	Timestamp  time.Time            `json:"timestamp"`
}

// PrintBacktestResults 打印回测结果
func (ts *TradingSystem) PrintBacktestResults(stats *backtest.Statistics) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("📊 BACKTEST RESULTS")
	fmt.Println(strings.Repeat("=", 60))

	fmt.Printf("Strategy: %s\n", ts.strategy.GetName())
	fmt.Printf("Symbol: %s\n", ts.config.Trading.Symbol)
	fmt.Printf("Timeframe: %s\n", ts.config.Trading.Timeframe)
	fmt.Printf("Initial Capital: $%.2f\n", ts.config.Trading.InitialCapital)

	fmt.Println("\n📈 PERFORMANCE METRICS")
	fmt.Println(strings.Repeat("-", 30))
	totalReturn, _ := stats.TotalReturn.Mul(decimal.NewFromInt(100)).Float64()
	annualizedReturn, _ := stats.AnnualizedReturn.Mul(decimal.NewFromInt(100)).Float64()
	maxDrawdown, _ := stats.MaxDrawdown.Mul(decimal.NewFromInt(100)).Float64()
	sharpeRatio, _ := stats.SharpeRatio.Float64()
	fmt.Printf("Total Return: %.2f%%\n", totalReturn)
	fmt.Printf("Annualized Return: %.2f%%\n", annualizedReturn)
	fmt.Printf("Max Drawdown: %.2f%%\n", maxDrawdown)
	fmt.Printf("Sharpe Ratio: %.4f\n", sharpeRatio)

	fmt.Println("\n📊 TRADING STATISTICS")
	fmt.Println(strings.Repeat("-", 30))
	fmt.Printf("Total Trades: %d\n", stats.TotalTrades)
	fmt.Printf("Winning Trades: %d\n", stats.WinningTrades)
	fmt.Printf("Losing Trades: %d\n", stats.LosingTrades)
	winRate, _ := stats.WinRate.Mul(decimal.NewFromInt(100)).Float64()
	totalPnL, _ := stats.TotalPnL.Float64()
	totalCommission, _ := stats.TotalCommission.Float64()
	fmt.Printf("Win Rate: %.2f%%\n", winRate)
	fmt.Printf("Total P&L: $%.2f\n", totalPnL)
	fmt.Printf("Total Commission: $%.2f\n", totalCommission)

	// 显示最近几笔交易
	trades := ts.backtest.GetTrades()
	if len(trades) > 0 {
		fmt.Println("\n📋 RECENT TRADES (Last 10)")
		fmt.Println(strings.Repeat("-", 80))
		fmt.Printf("%-10s %-4s %-10s %-12s %-12s %-15s\n",
			"Time", "Side", "Quantity", "Price", "P&L", "Reason")
		fmt.Println(strings.Repeat("-", 80))

		// 显示最后10笔交易
		start := len(trades) - 10
		if start < 0 {
			start = 0
		}

		for i := start; i < len(trades); i++ {
			trade := trades[i]
			timeStr := trade.Timestamp.Format("01-02 15:04")
			pnlFloat, _ := trade.PnL.Float64()
			pnlStr := fmt.Sprintf("$%.2f", pnlFloat)
			if trade.Side == "buy" {
				pnlStr = "-"
			}

			quantityFloat, _ := trade.Quantity.Float64()
			priceFloat, _ := trade.Price.Float64()
			fmt.Printf("%-10s %-4s %-10.6f %-12.2f %-12s\n",
				timeStr,
				trade.Side,
				quantityFloat,
				priceFloat,
				pnlStr,
			)
		}
	}

	fmt.Println("\n" + strings.Repeat("=", 60))
}

// RunLiveTrading 运行实时交易
func (ts *TradingSystem) RunLiveTrading() error {
	if ts.config.IsBacktestMode() {
		return fmt.Errorf("cannot run live trading in backtest mode")
	}

	fmt.Println("🔴 Live trading is not implemented yet")
	fmt.Println("Please use backtest mode for now")

	// TODO: 实现实时交易逻辑
	// 1. 实时获取K线数据
	// 2. 执行策略
	// 3. 下单管理
	// 4. 风险控制
	// 5. 日志记录

	return fmt.Errorf("live trading not implemented")
}

// Stop 停止交易系统
func (ts *TradingSystem) Stop() {
	if ts.cancel != nil {
		ts.cancel()
	}
	fmt.Println("✅ Trading system stopped")
}

// GetConfig 获取配置
func (ts *TradingSystem) GetConfig() *config.Config {
	return ts.config
}

// GetBinanceClient 获取币安客户端
func (ts *TradingSystem) GetBinanceClient() *binance.Client {
	return ts.binanceClient
}

// GetStrategy 获取策略
func (ts *TradingSystem) GetStrategy() backtest.Strategy {
	return ts.strategy
}
