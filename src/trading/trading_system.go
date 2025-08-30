package trading

import (
	"context"
	"fmt"

	"go-build-stream-gateway-go-server-main/src/binance"
	"go-build-stream-gateway-go-server-main/src/config"
	"go-build-stream-gateway-go-server-main/src/database"
	"go-build-stream-gateway-go-server-main/src/engine"
	"go-build-stream-gateway-go-server-main/src/executor"
	"go-build-stream-gateway-go-server-main/src/strategies"
	"go-build-stream-gateway-go-server-main/src/strategy"

	"github.com/shopspring/decimal"
)

// TradingSystem 交易系统（重构版）
type TradingSystem struct {
	config        *config.Config
	binanceClient *binance.Client
	database      *database.PostgresDB
	klineManager  *database.KlineManager
	tradingEngine *engine.TradingEngine
	currentCEX    string // 当前使用的交易所
	ctx           context.Context
	cancel        context.CancelFunc
}

// NewTradingSystem 创建新的交易系统
func NewTradingSystem() (*TradingSystem, error) {
	cfg := config.AppConfig

	ctx, cancel := context.WithCancel(context.Background())

	return &TradingSystem{
		config: cfg,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

// SetSymbolAndTimeframe 设置交易对和时间周期
func (ts *TradingSystem) SetSymbolAndTimeframe(symbol, timeframe string) error {
	return ts.SetSymbolTimeframeAndCEX(symbol, timeframe, "binance")
}

// SetSymbolTimeframeAndCEX 设置交易对、时间周期和交易所
func (ts *TradingSystem) SetSymbolTimeframeAndCEX(symbol, timeframe, cex string) error {
	// 验证交易对是否在配置中支持
	supported := false
	for _, s := range config.AppConfig.Symbols {
		if s.Symbol == symbol {
			supported = true
			break
		}
	}
	if !supported {
		supportedSymbols := make([]string, 0, len(config.AppConfig.Symbols))
		for _, s := range config.AppConfig.Symbols {
			supportedSymbols = append(supportedSymbols, s.Symbol)
		}
		if len(supportedSymbols) > 0 {
			return fmt.Errorf("trading pair %s is not supported. Supported pairs: %v", symbol, supportedSymbols[:min(5, len(supportedSymbols))])
		} else {
			return fmt.Errorf("trading pair %s is not supported", symbol)
		}
	}

	// 验证时间周期格式
	originalTimeframe := ts.config.Trading.Timeframe
	ts.config.Trading.Timeframe = timeframe
	_, err := ts.config.GetTimeframe()
	if err != nil {
		ts.config.Trading.Timeframe = originalTimeframe
		return fmt.Errorf("invalid timeframe: %s", timeframe)
	}

	// 验证CEX是否支持
	supportedCEXs := ts.config.GetSupportedCEXs()
	supported = false
	for _, supportedCEX := range supportedCEXs {
		if supportedCEX == cex {
			supported = true
			break
		}
	}
	if !supported {
		return fmt.Errorf("exchange %s is not supported. Supported exchanges: %v", cex, supportedCEXs)
	}

	// 设置交易对、时间周期和交易所
	ts.config.Trading.Symbol = symbol
	ts.config.Trading.Timeframe = timeframe
	ts.currentCEX = cex

	// 验证完整配置
	return ts.config.ValidateWithSymbol()
}

// min 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Initialize 初始化系统
func (ts *TradingSystem) Initialize() error {
	// 根据当前CEX获取对应的配置
	cexConfig, dbConfig, err := ts.config.GetCEXConfig(ts.currentCEX)
	if err != nil {
		return fmt.Errorf("failed to get CEX config: %w", err)
	}

	// 初始化CEX客户端（目前只支持Binance）
	if ts.currentCEX != "binance" {
		return fmt.Errorf("unsupported CEX: %s, only binance is supported", ts.currentCEX)
	}

	binanceConfig := cexConfig.(*config.BinanceConfig)
	ts.binanceClient = binance.NewClient(
		binanceConfig.APIKey,
		binanceConfig.SecretKey,
		binanceConfig.BaseURL,
	)

	// 尝试连接数据库（根据当前CEX选择对应的数据库）
	if dbConfig.Host != "" {
		fmt.Printf("🗄️ Connecting to %s database...", ts.currentCEX)
		db, err := database.NewPostgresDB(
			dbConfig.Host,
			dbConfig.Port,
			dbConfig.User,
			dbConfig.Password,
			dbConfig.DBName,
			dbConfig.SSLMode,
		)
		if err != nil {
			fmt.Printf(" failed: %v\n", err)
			fmt.Println("⚠️ Database unavailable, using network only")
		} else {
			ts.database = db
			ts.klineManager = database.NewKlineManager(db, ts.binanceClient)
			fmt.Println(" connected!")
		}
	}

	// 测试连接（如果不是回测模式）
	if !ts.config.IsBacktestMode() {
		err := ts.binanceClient.Ping(ts.ctx)
		if err != nil {
			return fmt.Errorf("failed to connect to Binance: %w", err)
		}
		fmt.Println("✓ Connected to Binance API")
	}

	return nil
}

// RunBacktest 运行回测
func (ts *TradingSystem) RunBacktest() (*BacktestStatistics, error) {
	if !ts.config.IsBacktestMode() {
		return nil, fmt.Errorf("not in backtest mode")
	}

	fmt.Println("🔄 Starting backtest...")

	// 创建策略
	var strategyImpl strategy.Strategy
	switch ts.config.Strategy.Name {
	case "bollinger_bands":
		strategyImpl = strategies.NewBollingerBandsStrategy()
		err := strategyImpl.SetParams(ts.config.GetStrategyParams())
		if err != nil {
			return nil, fmt.Errorf("failed to set strategy parameters: %w", err)
		}
		fmt.Printf("✓ Initialized %s with params: %+v\n", strategyImpl.GetName(), strategyImpl.GetParams())
	default:
		return nil, fmt.Errorf("unsupported strategy: %s", ts.config.Strategy.Name)
	}

	// 创建回测执行器
	initialCapital := decimal.NewFromFloat(ts.config.Trading.InitialCapital)
	backtestExecutor := executor.NewBacktestExecutor(ts.config.Trading.Symbol, initialCapital)
	backtestExecutor.SetCommission(ts.config.Backtest.Fee)
	backtestExecutor.SetSlippage(ts.config.Backtest.Slippage)

	// 获取时间周期
	timeframe, err := ts.config.GetTimeframe()
	if err != nil {
		return nil, fmt.Errorf("invalid timeframe: %w", err)
	}

	// 创建交易引擎
	ts.tradingEngine = engine.NewTradingEngine(
		ts.config.Trading.Symbol,
		timeframe,
		strategyImpl,
		backtestExecutor,
		ts.klineManager,
		ts.binanceClient,
	)

	// 设置交易参数
	ts.tradingEngine.SetPositionSizePercent(ts.config.Trading.PositionSizePercent)
	ts.tradingEngine.SetMinTradeAmount(ts.config.Trading.MinTradeAmount)

	// 获取时间范围
	startTime, err := ts.config.GetStartTime()
	if err != nil {
		return nil, fmt.Errorf("invalid start time: %w", err)
	}

	endTime, err := ts.config.GetEndTime()
	if err != nil {
		return nil, fmt.Errorf("invalid end time: %w", err)
	}

	// 运行回测
	err = ts.tradingEngine.RunBacktest(ts.ctx, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("backtest failed: %w", err)
	}

	fmt.Println("✅ Backtest completed")

	// 获取回测统计
	stats := backtestExecutor.GetStatistics()
	orders := backtestExecutor.GetOrders()

	return &BacktestStatistics{
		InitialCapital:  stats["initial_capital"].(decimal.Decimal),
		FinalPortfolio:  stats["final_portfolio"].(decimal.Decimal),
		TotalReturn:     stats["total_return"].(decimal.Decimal),
		TotalTrades:     stats["total_trades"].(int),
		WinningTrades:   stats["winning_trades"].(int),
		LosingTrades:    stats["losing_trades"].(int),
		TotalCommission: stats["total_commission"].(decimal.Decimal),
		Orders:          orders,
	}, nil
}

// RunLiveTrading 运行实时交易
func (ts *TradingSystem) RunLiveTrading() error {
	if ts.config.IsBacktestMode() {
		return fmt.Errorf("cannot run live trading in backtest mode")
	}

	fmt.Println("🔴 Starting live trading...")

	// 创建策略
	var strategyImpl strategy.Strategy
	switch ts.config.Strategy.Name {
	case "bollinger_bands":
		strategyImpl = strategies.NewBollingerBandsStrategy()
		err := strategyImpl.SetParams(ts.config.GetStrategyParams())
		if err != nil {
			return fmt.Errorf("failed to set strategy parameters: %w", err)
		}
		fmt.Printf("✓ Initialized %s with params: %+v\n", strategyImpl.GetName(), strategyImpl.GetParams())
	default:
		return fmt.Errorf("unsupported strategy: %s", ts.config.Strategy.Name)
	}

	// 创建实盘执行器
	liveExecutor := executor.NewLiveExecutor(ts.binanceClient, ts.config.Trading.Symbol)

	// 获取时间周期
	timeframe, err := ts.config.GetTimeframe()
	if err != nil {
		return fmt.Errorf("invalid timeframe: %w", err)
	}

	// 创建交易引擎
	ts.tradingEngine = engine.NewTradingEngine(
		ts.config.Trading.Symbol,
		timeframe,
		strategyImpl,
		liveExecutor,
		ts.klineManager,
		ts.binanceClient,
	)

	// 设置交易参数
	ts.tradingEngine.SetPositionSizePercent(ts.config.Trading.PositionSizePercent)
	ts.tradingEngine.SetMinTradeAmount(ts.config.Trading.MinTradeAmount)

	// 运行实盘交易
	return ts.tradingEngine.RunLive(ts.ctx)
}

// Stop 停止交易系统
func (ts *TradingSystem) Stop() {
	if ts.tradingEngine != nil {
		ts.tradingEngine.Stop()
	}
	if ts.database != nil {
		ts.database.Close()
	}
	ts.cancel()
	fmt.Println("Trading system stopped")
}

// GetConfig 获取配置
func (ts *TradingSystem) GetConfig() *config.Config {
	return ts.config
}

// BacktestStatistics 回测统计结果
type BacktestStatistics struct {
	InitialCapital  decimal.Decimal        `json:"initial_capital"`
	FinalPortfolio  decimal.Decimal        `json:"final_portfolio"`
	TotalReturn     decimal.Decimal        `json:"total_return"`
	TotalTrades     int                    `json:"total_trades"`
	WinningTrades   int                    `json:"winning_trades"`
	LosingTrades    int                    `json:"losing_trades"`
	TotalCommission decimal.Decimal        `json:"total_commission"`
	Orders          []executor.OrderResult `json:"orders"`
}

// PrintBacktestResults 打印回测结果
func (ts *TradingSystem) PrintBacktestResults(stats *BacktestStatistics) {
	fmt.Println("\n============================================================")
	fmt.Println("📊 BACKTEST RESULTS")
	fmt.Println("============================================================")
	fmt.Printf("Strategy: Bollinger Bands Strategy\n")
	fmt.Printf("Symbol: %s\n", ts.config.Trading.Symbol)
	fmt.Printf("Timeframe: %s\n", ts.config.Trading.Timeframe)
	fmt.Printf("Initial Capital: $%.2f\n", stats.InitialCapital.InexactFloat64())

	fmt.Println("\n📈 PERFORMANCE METRICS")
	fmt.Println("------------------------------")
	totalReturnPercent := stats.TotalReturn.Mul(decimal.NewFromInt(100))
	fmt.Printf("Total Return: %.2f%%\n", totalReturnPercent.InexactFloat64())

	winRate := decimal.Zero
	if stats.TotalTrades > 0 {
		winRate = decimal.NewFromInt(int64(stats.WinningTrades)).Div(decimal.NewFromInt(int64(stats.TotalTrades))).Mul(decimal.NewFromInt(100))
	}

	fmt.Println("\n📊 TRADING STATISTICS")
	fmt.Println("------------------------------")
	fmt.Printf("Total Trades: %d\n", stats.TotalTrades)
	fmt.Printf("Winning Trades: %d\n", stats.WinningTrades)
	fmt.Printf("Losing Trades: %d\n", stats.LosingTrades)
	fmt.Printf("Win Rate: %.2f%%\n", winRate.InexactFloat64())

	totalPnL := stats.FinalPortfolio.Sub(stats.InitialCapital)
	fmt.Printf("Total P&L: $%.2f\n", totalPnL.InexactFloat64())
	fmt.Printf("Total Commission: $%.2f\n", stats.TotalCommission.InexactFloat64())

	// 显示最近的交易
	if len(stats.Orders) > 0 {
		fmt.Println("\n📋 RECENT TRADES (Last 10)")
		fmt.Println("--------------------------------------------------------------------------------")
		fmt.Println("Time       Side Quantity   Price        P&L          Reason         ")
		fmt.Println("--------------------------------------------------------------------------------")

		displayCount := len(stats.Orders)
		if displayCount > 10 {
			displayCount = 10
		}

		for i := len(stats.Orders) - displayCount; i < len(stats.Orders); i++ {
			order := stats.Orders[i]
			pnlStr := "-"
			if order.Side == executor.OrderSideSell && i > 0 {
				// 简化的盈亏计算
				prevBuy := findPreviousBuyOrder(stats.Orders, i)
				if prevBuy != nil {
					pnl := order.Quantity.Mul(order.Price.Sub(prevBuy.Price))
					pnlStr = fmt.Sprintf("$%.2f", pnl.InexactFloat64())
				}
			}

			fmt.Printf("%s %4s %12.6f %12.2f %12s %s\n",
				order.Timestamp.Format("01-02 15:04"),
				order.Side,
				order.Quantity.InexactFloat64(),
				order.Price.InexactFloat64(),
				pnlStr,
				"", // reason 暂时为空
			)
		}
	}

	fmt.Println("\n============================================================")
}

// findPreviousBuyOrder 查找前一个买入订单
func findPreviousBuyOrder(orders []executor.OrderResult, currentIndex int) *executor.OrderResult {
	for i := currentIndex - 1; i >= 0; i-- {
		if orders[i].Side == executor.OrderSideBuy {
			return &orders[i]
		}
	}
	return nil
}
