package trading

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"tradingbot/src/cex"
	"tradingbot/src/engine"
	"tradingbot/src/executor"
	"tradingbot/src/strategies"
	"tradingbot/src/strategy"
	"tradingbot/src/timeframes"

	"github.com/shopspring/decimal"
)

// TradingSystem 交易系统（重构版）
type TradingSystem struct {
	cexClient     cex.CEXClient
	tradingEngine *engine.TradingEngine
	ctx           context.Context
	cancel        context.CancelFunc
}

// NewTradingSystem 创建新的交易系统
func NewTradingSystem() (*TradingSystem, error) {
	ctx, cancel := context.WithCancel(context.Background())

	return &TradingSystem{
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

// SetTradingPairAndTimeframe 设置交易对和时间周期
func (ts *TradingSystem) SetTradingPairAndTimeframe(pair cex.TradingPair, timeframe string) error {
	return ts.SetTradingPairTimeframeAndCEX(pair, timeframe, "binance")
}

// SetTradingPairTimeframeAndCEX 设置交易对、时间周期和交易所
func (ts *TradingSystem) SetTradingPairTimeframeAndCEX(pair cex.TradingPair, timeframe, cexName string) error {
	// 验证时间周期格式
	_, err := timeframes.ParseTimeframe(timeframe)
	if err != nil {
		return fmt.Errorf("invalid timeframe: %s", timeframe)
	}

	// 设置时间周期到交易配置
	TradingConfigValue.Timeframe = timeframe

	// 初始化 CEX 客户端
	if err := ts.initializeCEX(cexName); err != nil {
		return fmt.Errorf("failed to initialize CEX: %w", err)
	}

	return nil
}

// SetTradingPairFromStrings 从字符串创建交易对并设置
func (ts *TradingSystem) SetTradingPairFromStrings(base, quote, timeframe, cexName string) error {
	pair := cex.TradingPair{
		Base:  strings.ToUpper(base),
		Quote: strings.ToUpper(quote),
	}
	return ts.SetTradingPairTimeframeAndCEX(pair, timeframe, cexName)
}

// RunBacktestFromStrings 从字符串参数运行回测
func (ts *TradingSystem) RunBacktestFromStrings(base, quote, startDate, endDate string, initialCapital float64, strategyParams strategy.StrategyParams) (*BacktestStatistics, error) {
	pair := cex.TradingPair{
		Base:  strings.ToUpper(base),
		Quote: strings.ToUpper(quote),
	}
	return ts.RunBacktestWithParamsAndCapital(pair, startDate, endDate, initialCapital, strategyParams)
}

// RunLiveTradingFromStrings 从字符串参数运行实盘交易
func (ts *TradingSystem) RunLiveTradingFromStrings(base, quote string, strategyParams strategy.StrategyParams) error {
	pair := cex.TradingPair{
		Base:  strings.ToUpper(base),
		Quote: strings.ToUpper(quote),
	}
	return ts.RunLiveTradingWithParams(pair, strategyParams)
}

// PrintBacktestResultsFromStrings 从字符串参数打印回测结果
func (ts *TradingSystem) PrintBacktestResultsFromStrings(base, quote string, stats *BacktestStatistics) {
	pair := cex.TradingPair{
		Base:  strings.ToUpper(base),
		Quote: strings.ToUpper(quote),
	}
	ts.PrintBacktestResults(pair, stats)
}

// min 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// initializeCEX 初始化 CEX 客户端和数据库连接
func (ts *TradingSystem) initializeCEX(cexName string) error {
	// 使用工厂模式创建 CEX 客户端（客户端内部已经初始化了数据库连接）
	client, err := cex.CreateCEXClient(cexName)
	if err != nil {
		return fmt.Errorf("failed to create CEX client: %w", err)
	}

	ts.cexClient = client

	return nil
}

// Initialize 初始化系统（保持向后兼容）
func (ts *TradingSystem) Initialize() error {
	// 如果 CEX 客户端已经初始化，则跳过
	if ts.cexClient != nil {
		return nil
	}

	// 默认使用 binance
	return ts.initializeCEX("binance")
}

// RunBacktest 运行回测
func (ts *TradingSystem) RunBacktest(pair cex.TradingPair, startDate, endDate string) (*BacktestStatistics, error) {
	return ts.RunBacktestWithParams(pair, startDate, endDate, nil)
}

// RunBacktestWithParams 使用指定策略参数运行回测
func (ts *TradingSystem) RunBacktestWithParams(pair cex.TradingPair, startDate, endDate string, strategyParams strategy.StrategyParams) (*BacktestStatistics, error) {
	return ts.RunBacktestWithParamsAndCapital(pair, startDate, endDate, 10000.0, strategyParams)
}

// RunBacktestWithParamsAndCapital 使用指定策略参数和初始资金运行回测
func (ts *TradingSystem) RunBacktestWithParamsAndCapital(pair cex.TradingPair, startDate, endDate string, initialCapital float64, strategyParams strategy.StrategyParams) (*BacktestStatistics, error) {

	// 初始化系统
	err := ts.Initialize()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize trading system: %w", err)
	}

	fmt.Println("🔄 Starting backtest...")

	// 创建策略（目前只支持布林道策略）
	strategyImpl := strategies.NewBollingerBandsStrategy()

	// 使用传入的参数或默认参数
	var params strategy.StrategyParams
	if strategyParams != nil {
		params = strategyParams
	} else {
		// 使用默认参数
		params = strategy.GetDefaultBollingerBandsParams()
	}

	// 验证参数
	if err := params.Validate(); err != nil {
		return nil, fmt.Errorf("invalid strategy parameters: %w", err)
	}

	err = strategyImpl.SetParams(params)
	if err != nil {
		return nil, fmt.Errorf("failed to set strategy parameters: %w", err)
	}
	fmt.Printf("✓ Initialized %s with params: %+v\n", strategyImpl.GetName(), strategyImpl.GetParams())

	// 创建回测执行器
	initialCapitalDecimal := decimal.NewFromFloat(initialCapital)
	// backtestExecutor := executor.NewBacktestExecutor(pair, initialCapitalDecimal)

	// 设置手续费（从CEX客户端获取）
	fee := ts.cexClient.GetTradingFee()
	backtestExecutor.SetCommission(fee)

	// 获取时间周期
	timeframe, err := timeframes.ParseTimeframe(TradingConfigValue.Timeframe)
	if err != nil {
		return nil, fmt.Errorf("invalid timeframe: %w", err)
	}

	// 解析时间范围
	startTime, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return nil, fmt.Errorf("invalid start date format (expected YYYY-MM-DD): %w", err)
	}

	endTime, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return nil, fmt.Errorf("invalid end date format (expected YYYY-MM-DD): %w", err)
	}

	// 🔄 获取历史数据用于回测
	fmt.Println("📊 Loading historical data...")
	klines, err := ts.cexClient.GetKlinesWithTimeRange(ts.ctx, pair, timeframe.GetBinanceInterval(),
		startTime, endTime, 1000)
	if err != nil {
		return nil, fmt.Errorf("failed to load historical data: %w", err)
	}

	if len(klines) == 0 {
		return nil, fmt.Errorf("no historical data available")
	}

	fmt.Printf("✓ Loaded %d klines for %s\n", len(klines), pair.String())

	// 🎯 创建回测数据喂入器
	dataFeed := engine.NewBacktestDataFeed(klines)

	// 🎯 创建回测挂单管理器
	orderManager := engine.NewBacktestOrderManager(backtestExecutor)

	// 创建交易引擎
	ts.tradingEngine = engine.NewTradingEngine(
		pair,
		timeframe,
		strategyImpl,
		backtestExecutor,
		ts.cexClient,
		dataFeed,
		orderManager,
	)

	// 设置交易参数
	ts.tradingEngine.SetPositionSizePercent(TradingConfigValue.PositionSizePercent)
	ts.tradingEngine.SetMinTradeAmount(TradingConfigValue.MinTradeAmount)

	// 🚀 运行统一的tick-by-tick回测
	fmt.Println("🎮 Starting tick-by-tick backtest simulation...")
	err = ts.tradingEngine.RunBacktest(ts.ctx, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("backtest failed: %w", err)
	}

	fmt.Println("✅ Backtest completed")

	// 获取回测统计
	stats := backtestExecutor.GetStatistics()
	orders := backtestExecutor.GetOrders()

	// 进行详细交易分析
	trades, openPositions, avgHoldingTime, maxHoldingTime, minHoldingTime, avgWinningPnL, avgLosingPnL, maxWin, maxLoss, profitFactor := AnalyzeTrades(orders)

	// 计算最大回撤 - 使用真实K线数据
	capitalForDrawdown := stats["initial_capital"].(decimal.Decimal)
	klines = ts.tradingEngine.GetKlines() // 获取回测过程中的K线数据
	drawdownInfo := CalculateDrawdownWithKlines(orders, klines, capitalForDrawdown)

	// 计算年化收益率 (APR)
	backtestDays := int(endTime.Sub(startTime).Hours() / 24)
	if backtestDays == 0 {
		backtestDays = 1 // 避免除零
	}

	var annualReturn decimal.Decimal
	if backtestDays > 0 {
		// APR = ((Final / Initial)^(365/Days) - 1) * 100
		initialCap := stats["initial_capital"].(decimal.Decimal)
		finalPort := stats["final_portfolio"].(decimal.Decimal)

		if initialCap.IsPositive() {
			totalReturn := finalPort.Div(initialCap) // Final/Initial
			daysInYear := decimal.NewFromFloat(365.0)
			backtestDaysDecimal := decimal.NewFromInt(int64(backtestDays))
			yearFraction := daysInYear.Div(backtestDaysDecimal) // 365/Days

			// 计算 totalReturn^yearFraction
			// 由于decimal包不支持指数运算，使用浮点数计算然后转回decimal
			totalReturnFloat := totalReturn.InexactFloat64()
			yearFractionFloat := yearFraction.InexactFloat64()

			if totalReturnFloat > 0 {
				annualizedMultiplier := math.Pow(totalReturnFloat, yearFractionFloat)
				annualReturn = decimal.NewFromFloat((annualizedMultiplier - 1.0) * 100.0)
			}
		}
	}

	return &BacktestStatistics{
		InitialCapital:  stats["initial_capital"].(decimal.Decimal),
		FinalPortfolio:  stats["final_portfolio"].(decimal.Decimal),
		TotalReturn:     stats["total_return"].(decimal.Decimal),
		TotalTrades:     stats["total_trades"].(int),
		WinningTrades:   stats["winning_trades"].(int),
		LosingTrades:    stats["losing_trades"].(int),
		TotalCommission: stats["total_commission"].(decimal.Decimal),
		Orders:          orders,

		// 新增的详细分析
		Trades:         trades,
		OpenPositions:  openPositions,
		AvgHoldingTime: avgHoldingTime,
		MaxHoldingTime: maxHoldingTime,
		MinHoldingTime: minHoldingTime,
		AvgWinningPnL:  avgWinningPnL,
		AvgLosingPnL:   avgLosingPnL,
		MaxWin:         maxWin,
		MaxLoss:        maxLoss,
		ProfitFactor:   profitFactor,

		// 最大回撤统计
		MaxDrawdown:        drawdownInfo.MaxDrawdown,
		MaxDrawdownPercent: drawdownInfo.MaxDrawdownPercent,
		DrawdownDuration:   drawdownInfo.DrawdownDuration,
		CurrentDrawdown:    drawdownInfo.CurrentDrawdown,
		PeakPortfolioValue: drawdownInfo.PeakValue,

		// 年化收益率统计
		AnnualReturn: annualReturn,
		BacktestDays: backtestDays,
	}, nil
}

// RunLiveTrading 运行实时交易
func (ts *TradingSystem) RunLiveTrading(pair cex.TradingPair) error {
	return ts.RunLiveTradingWithParams(pair, nil)
}

// RunLiveTradingWithParams 使用指定策略参数运行实时交易
func (ts *TradingSystem) RunLiveTradingWithParams(pair cex.TradingPair, strategyParams strategy.StrategyParams) error {
	// 测试 CEX 连接
	err := ts.cexClient.Ping(ts.ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to CEX: %w", err)
	}
	fmt.Println("✓ Connected to CEX API")

	// 初始化系统
	err = ts.Initialize()
	if err != nil {
		return fmt.Errorf("failed to initialize trading system: %w", err)
	}

	fmt.Println("🔴 Starting live trading...")

	// 创建策略（目前只支持布林道策略）
	strategyImpl := strategies.NewBollingerBandsStrategy()

	// 使用传入的参数或默认参数
	var params strategy.StrategyParams
	if strategyParams != nil {
		params = strategyParams
	} else {
		// 使用默认参数
		params = strategy.GetDefaultBollingerBandsParams()
	}

	// 验证参数
	if err := params.Validate(); err != nil {
		return fmt.Errorf("invalid strategy parameters: %w", err)
	}

	err = strategyImpl.SetParams(params)
	if err != nil {
		return fmt.Errorf("failed to set strategy parameters: %w", err)
	}
	fmt.Printf("✓ Initialized %s with params: %+v\n", strategyImpl.GetName(), strategyImpl.GetParams())

	// 创建实盘执行器
	liveExecutor := executor.NewLiveExecutor(ts.cexClient, pair)

	// 获取时间周期
	timeframe, err := timeframes.ParseTimeframe(TradingConfigValue.Timeframe)
	if err != nil {
		return fmt.Errorf("invalid timeframe: %w", err)
	}

	// 🎯 创建实盘数据喂入器
	tickerInterval, err := timeframe.GetDuration() // 根据时间框架设置数据获取频率
	if err != nil {
		return fmt.Errorf("invalid timeframe duration: %w", err)
	}
	dataFeed := engine.NewLiveDataFeed(ts.cexClient, pair, timeframe.GetBinanceInterval(), tickerInterval)

	// 🎯 创建实盘挂单管理器
	orderManager := engine.NewLiveOrderManager(ts.cexClient)

	// 创建交易引擎
	ts.tradingEngine = engine.NewTradingEngine(
		pair,
		timeframe,
		strategyImpl,
		liveExecutor,
		ts.cexClient,
		dataFeed,
		orderManager,
	)

	// 设置交易参数
	ts.tradingEngine.SetPositionSizePercent(TradingConfigValue.PositionSizePercent)
	ts.tradingEngine.SetMinTradeAmount(TradingConfigValue.MinTradeAmount)

	// 🚀 运行统一的tick-by-tick实盘交易
	fmt.Println("🔴 Starting tick-by-tick live trading...")
	return ts.tradingEngine.RunLive(ts.ctx)
}

// Stop 停止交易系统
func (ts *TradingSystem) Stop() {
	if ts.tradingEngine != nil {
		ts.tradingEngine.Stop()
	}
	ts.cancel()
	fmt.Println("Trading system stopped")
}

// TradeAnalysis 单笔交易分析
type TradeAnalysis struct {
	BuyOrder   executor.OrderResult  `json:"buy_order"`
	SellOrder  *executor.OrderResult `json:"sell_order,omitempty"`
	Duration   time.Duration         `json:"duration"`
	PnL        decimal.Decimal       `json:"pnl"`
	PnLPercent decimal.Decimal       `json:"pnl_percent"`
	Commission decimal.Decimal       `json:"commission"`
	IsOpen     bool                  `json:"is_open"`
	BuyReason  string                `json:"buy_reason"`
	SellReason string                `json:"sell_reason,omitempty"`
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

	// 新增的详细分析
	Trades         []TradeAnalysis `json:"trades"`
	OpenPositions  []TradeAnalysis `json:"open_positions"`
	AvgHoldingTime time.Duration   `json:"avg_holding_time"`
	MaxHoldingTime time.Duration   `json:"max_holding_time"`
	MinHoldingTime time.Duration   `json:"min_holding_time"`
	AvgWinningPnL  decimal.Decimal `json:"avg_winning_pnl"`
	AvgLosingPnL   decimal.Decimal `json:"avg_losing_pnl"`
	MaxWin         decimal.Decimal `json:"max_win"`
	MaxLoss        decimal.Decimal `json:"max_loss"`
	ProfitFactor   decimal.Decimal `json:"profit_factor"`

	// 最大回撤相关统计
	MaxDrawdown        decimal.Decimal `json:"max_drawdown"`         // 最大回撤金额
	MaxDrawdownPercent decimal.Decimal `json:"max_drawdown_percent"` // 最大回撤百分比
	DrawdownDuration   time.Duration   `json:"drawdown_duration"`    // 最大回撤持续时间
	CurrentDrawdown    decimal.Decimal `json:"current_drawdown"`     // 当前回撤
	PeakPortfolioValue decimal.Decimal `json:"peak_portfolio_value"` // 历史最高组合价值

	// 年化收益率统计
	AnnualReturn decimal.Decimal `json:"annual_return"` // 年化收益率 (APR)
	BacktestDays int             `json:"backtest_days"` // 回测天数
}

// PrintBacktestResults 打印回测结果
func (ts *TradingSystem) PrintBacktestResults(pair cex.TradingPair, stats *BacktestStatistics) {
	fmt.Println("\n============================================================")
	fmt.Println("📊 BACKTEST RESULTS")
	fmt.Println("============================================================")
	fmt.Printf("Strategy: Bollinger Bands Strategy\n")
	fmt.Printf("Symbol: %s\n", pair.String())
	fmt.Printf("Timeframe: %s\n", TradingConfigValue.Timeframe)
	fmt.Printf("Initial Capital: $%.2f\n", stats.InitialCapital.InexactFloat64())

	fmt.Println("\n📈 PERFORMANCE METRICS")
	fmt.Println("------------------------------")
	totalReturnPercent := stats.TotalReturn.Mul(decimal.NewFromInt(100))
	fmt.Printf("Total Return: %.2f%%\n", totalReturnPercent.InexactFloat64())
	fmt.Printf("Annual Return (APR): %.2f%%\n", stats.AnnualReturn.InexactFloat64())
	fmt.Printf("Backtest Period: %d days\n", stats.BacktestDays)

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
		fmt.Println("================================================================================================")
		fmt.Println("Time       Side Quantity     Price     Amount($)     P&L          Reason         ")
		fmt.Println("================================================================================================")

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

			// 计算交易金额 (数量 × 价格)
			amount := order.Quantity.Mul(order.Price)

			fmt.Printf("%s %4s %12.0f %9.4f %10.2f %12s %s\n",
				order.Timestamp.Format("01-02 15:04"),
				order.Side,
				order.Quantity.InexactFloat64(),
				order.Price.InexactFloat64(),
				amount.InexactFloat64(),
				pnlStr,
				"", // reason 暂时为空
			)
		}
	}

	// 显示详细分析
	fmt.Println("\n🔍 DETAILED ANALYSIS")
	fmt.Println("------------------------------")

	if len(stats.Trades) > 0 {
		fmt.Printf("Completed Trades: %d\n", len(stats.Trades))
		fmt.Printf("Average Holding Time: %v\n", formatDuration(stats.AvgHoldingTime))
		fmt.Printf("Max Holding Time: %v\n", formatDuration(stats.MaxHoldingTime))
		fmt.Printf("Min Holding Time: %v\n", formatDuration(stats.MinHoldingTime))

		if !stats.AvgWinningPnL.IsZero() {
			fmt.Printf("Average Winning P&L: $%.2f\n", stats.AvgWinningPnL.InexactFloat64())
		}
		if !stats.AvgLosingPnL.IsZero() {
			fmt.Printf("Average Losing P&L: $%.2f\n", stats.AvgLosingPnL.InexactFloat64())
		}
		if !stats.MaxWin.IsZero() {
			fmt.Printf("Max Win: $%.2f\n", stats.MaxWin.InexactFloat64())
		}
		if !stats.MaxLoss.IsZero() {
			fmt.Printf("Max Loss: $%.2f\n", stats.MaxLoss.InexactFloat64())
		}
		if !stats.ProfitFactor.IsZero() {
			fmt.Printf("Profit Factor: %.2f\n", stats.ProfitFactor.InexactFloat64())
		}
	}

	// 显示未平仓订单
	if len(stats.OpenPositions) > 0 {
		fmt.Printf("\n🔓 OPEN POSITIONS: %d\n", len(stats.OpenPositions))
		fmt.Println("--------------------------------------------------------------------------------")
		fmt.Println("Buy Time   Buy Price    Quantity     Cost         Reason")
		fmt.Println("--------------------------------------------------------------------------------")

		for _, pos := range stats.OpenPositions {
			cost := pos.BuyOrder.Price.Mul(pos.BuyOrder.Quantity)
			fmt.Printf("%s %12.6f %12.6f $%10.2f %s\n",
				pos.BuyOrder.Timestamp.Format("01-02 15:04"),
				pos.BuyOrder.Price.InexactFloat64(),
				pos.BuyOrder.Quantity.InexactFloat64(),
				cost.InexactFloat64(),
				pos.BuyReason,
			)
		}
	}

	// 显示每笔交易的详细情况
	if len(stats.Trades) > 0 {
		fmt.Printf("\n📊 ALL COMPLETED TRADES: %d\n", len(stats.Trades))
		fmt.Println("================================================================================================================================================")
		fmt.Println("序号 买入时间      买入价格     买入金额     卖出时间      卖出价格     卖出金额      盈利%   净盈利$     持仓时间    卖出原因")
		fmt.Println("================================================================================================================================================")

		for i, trade := range stats.Trades {
			// 计算盈利百分比
			profitPercent := trade.PnL.Div(trade.BuyOrder.Price.Mul(trade.BuyOrder.Quantity)).Mul(decimal.NewFromInt(100))

			// 计算买入金额和卖出金额
			buyAmount := trade.BuyOrder.Quantity.Mul(trade.BuyOrder.Price)
			sellAmount := trade.SellOrder.Quantity.Mul(trade.SellOrder.Price)

			// 确定卖出原因
			sellReason := "触及上轨"
			if trade.SellReason != "" {
				if trade.SellReason == "strategy signal" {
					sellReason = "触及上轨"
				} else {
					sellReason = trade.SellReason
				}
			}

			fmt.Printf("%2d   %s %10.6f  $%8.2f  %s %10.6f  $%8.2f   %6.2f%%  $%8.2f  %8s   %s\n",
				i+1,
				trade.BuyOrder.Timestamp.Format("01-02 15:04"),
				trade.BuyOrder.Price.InexactFloat64(),
				buyAmount.InexactFloat64(),
				trade.SellOrder.Timestamp.Format("01-02 15:04"),
				trade.SellOrder.Price.InexactFloat64(),
				sellAmount.InexactFloat64(),
				profitPercent.InexactFloat64(),
				trade.PnL.InexactFloat64(),
				formatDuration(trade.Duration),
				sellReason,
			)
		}

		fmt.Println("================================================================================================================================================")

		// 统计不同盈利范围的交易
		fmt.Println("\n📈 PROFIT DISTRIBUTION")
		fmt.Println("------------------------------")

		ranges := map[string][2]float64{
			"1-5%":   {1.0, 5.0},
			"5-10%":  {5.0, 10.0},
			"10-20%": {10.0, 20.0},
			"20-30%": {20.0, 30.0},
			"30%+":   {30.0, 1000.0},
		}

		for rangeName, bounds := range ranges {
			count := 0
			totalProfit := decimal.Zero

			for _, trade := range stats.Trades {
				profitPercent := trade.PnL.Div(trade.BuyOrder.Price.Mul(trade.BuyOrder.Quantity)).Mul(decimal.NewFromInt(100))
				percent := profitPercent.InexactFloat64()

				if percent >= bounds[0] && percent < bounds[1] {
					count++
					totalProfit = totalProfit.Add(trade.PnL)
				}
			}

			if count > 0 {
				avgProfit := totalProfit.Div(decimal.NewFromInt(int64(count)))
				fmt.Printf("%s: %2d笔交易, 总盈利: $%8.2f, 平均: $%7.2f\n",
					rangeName, count, totalProfit.InexactFloat64(), avgProfit.InexactFloat64())
			}
		}
	}

	// 显示最佳和最差交易
	if len(stats.Trades) > 0 {
		fmt.Println("\n🏆 BEST & WORST TRADES")
		fmt.Println("--------------------------------------------------------------------------------")

		var bestTrade, worstTrade *TradeAnalysis
		for i := range stats.Trades {
			trade := &stats.Trades[i]
			if bestTrade == nil || trade.PnL.GreaterThan(bestTrade.PnL) {
				bestTrade = trade
			}
			if worstTrade == nil || trade.PnL.LessThan(worstTrade.PnL) {
				worstTrade = trade
			}
		}

		if bestTrade != nil {
			fmt.Printf("🥇 Best Trade: %s -> %s (%.2f%%) P&L: $%.2f Duration: %v\n",
				bestTrade.BuyOrder.Timestamp.Format("01-02 15:04"),
				bestTrade.SellOrder.Timestamp.Format("01-02 15:04"),
				bestTrade.PnLPercent.InexactFloat64(),
				bestTrade.PnL.InexactFloat64(),
				formatDuration(bestTrade.Duration),
			)
		}

		if worstTrade != nil {
			fmt.Printf("🥉 Worst Trade: %s -> %s (%.2f%%) P&L: $%.2f Duration: %v\n",
				worstTrade.BuyOrder.Timestamp.Format("01-02 15:04"),
				worstTrade.SellOrder.Timestamp.Format("01-02 15:04"),
				worstTrade.PnLPercent.InexactFloat64(),
				worstTrade.PnL.InexactFloat64(),
				formatDuration(worstTrade.Duration),
			)
		}
	}

	// 显示最大回撤信息
	fmt.Println("\n📉 RISK METRICS")
	fmt.Println("--------------------------------------------------------------------------------")
	fmt.Printf("Max Drawdown: $%.2f (%.2f%%)\n",
		stats.MaxDrawdown.InexactFloat64(),
		stats.MaxDrawdownPercent.InexactFloat64())

	if stats.DrawdownDuration > 0 {
		fmt.Printf("Drawdown Duration: %v\n", formatDuration(stats.DrawdownDuration))
	}

	fmt.Printf("Peak Portfolio Value: $%.2f\n", stats.PeakPortfolioValue.InexactFloat64())

	if stats.CurrentDrawdown.IsPositive() {
		currentDrawdownPercent := decimal.Zero
		if stats.PeakPortfolioValue.IsPositive() {
			currentDrawdownPercent = stats.CurrentDrawdown.Div(stats.PeakPortfolioValue).Mul(decimal.NewFromInt(100))
		}
		fmt.Printf("Current Drawdown: $%.2f (%.2f%%)\n",
			stats.CurrentDrawdown.InexactFloat64(),
			currentDrawdownPercent.InexactFloat64())
	} else {
		fmt.Printf("Current Drawdown: $0.00 (0.00%%)\n")
	}

	fmt.Println("\n============================================================")
}

// formatDuration 格式化时间间隔
func formatDuration(d time.Duration) string {
	if d == 0 {
		return "0"
	}

	hours := int(d.Hours())
	days := hours / 24
	hours = hours % 24

	if days > 0 {
		return fmt.Sprintf("%dd %dh", days, hours)
	} else if hours > 0 {
		return fmt.Sprintf("%dh", hours)
	} else {
		minutes := int(d.Minutes())
		return fmt.Sprintf("%dm", minutes)
	}
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

// AnalyzeTrades 分析交易数据，计算详细统计信息
func AnalyzeTrades(orders []executor.OrderResult) ([]TradeAnalysis, []TradeAnalysis, time.Duration, time.Duration, time.Duration, decimal.Decimal, decimal.Decimal, decimal.Decimal, decimal.Decimal, decimal.Decimal) {
	var trades []TradeAnalysis
	var openPositions []TradeAnalysis
	var holdingTimes []time.Duration
	var winningPnLs []decimal.Decimal
	var losingPnLs []decimal.Decimal

	// 配对买入和卖出订单
	var pendingBuys []executor.OrderResult

	for _, order := range orders {
		if order.Side == executor.OrderSideBuy {
			pendingBuys = append(pendingBuys, order)
		} else if order.Side == executor.OrderSideSell && len(pendingBuys) > 0 {
			// 找到对应的买入订单（FIFO）
			buyOrder := pendingBuys[0]
			pendingBuys = pendingBuys[1:]

			// 计算持仓时间
			duration := order.Timestamp.Sub(buyOrder.Timestamp)

			// 计算盈亏
			buyValue := buyOrder.Price.Mul(buyOrder.Quantity)
			sellValue := order.Price.Mul(order.Quantity)
			pnl := sellValue.Sub(buyValue)
			pnlPercent := pnl.Div(buyValue).Mul(decimal.NewFromInt(100))

			// 计算手续费
			commission := buyOrder.Commission.Add(order.Commission)

			trade := TradeAnalysis{
				BuyOrder:   buyOrder,
				SellOrder:  &order,
				Duration:   duration,
				PnL:        pnl.Sub(commission),
				PnLPercent: pnlPercent,
				Commission: commission,
				IsOpen:     false,
				BuyReason:  "strategy signal", // 默认原因
				SellReason: "strategy signal", // 默认原因
			}

			trades = append(trades, trade)
			holdingTimes = append(holdingTimes, duration)

			if trade.PnL.IsPositive() {
				winningPnLs = append(winningPnLs, trade.PnL)
			} else {
				losingPnLs = append(losingPnLs, trade.PnL)
			}
		}
	}

	// 处理未平仓订单
	for _, buyOrder := range pendingBuys {
		trade := TradeAnalysis{
			BuyOrder:   buyOrder,
			SellOrder:  nil,
			Duration:   0,
			PnL:        decimal.Zero,
			PnLPercent: decimal.Zero,
			Commission: buyOrder.Commission,
			IsOpen:     true,
			BuyReason:  "strategy signal", // 默认原因
			SellReason: "",
		}
		openPositions = append(openPositions, trade)
	}

	// 计算统计信息
	var avgHoldingTime, maxHoldingTime, minHoldingTime time.Duration
	var avgWinningPnL, avgLosingPnL, maxWin, maxLoss, profitFactor decimal.Decimal

	if len(holdingTimes) > 0 {
		var totalDuration time.Duration
		maxHoldingTime = holdingTimes[0]
		minHoldingTime = holdingTimes[0]

		for _, duration := range holdingTimes {
			totalDuration += duration
			if duration > maxHoldingTime {
				maxHoldingTime = duration
			}
			if duration < minHoldingTime {
				minHoldingTime = duration
			}
		}
		avgHoldingTime = totalDuration / time.Duration(len(holdingTimes))
	}

	if len(winningPnLs) > 0 {
		var totalWinning decimal.Decimal
		maxWin = winningPnLs[0]
		for _, pnl := range winningPnLs {
			totalWinning = totalWinning.Add(pnl)
			if pnl.GreaterThan(maxWin) {
				maxWin = pnl
			}
		}
		avgWinningPnL = totalWinning.Div(decimal.NewFromInt(int64(len(winningPnLs))))
	}

	if len(losingPnLs) > 0 {
		var totalLosing decimal.Decimal
		maxLoss = losingPnLs[0]
		for _, pnl := range losingPnLs {
			totalLosing = totalLosing.Add(pnl)
			if pnl.LessThan(maxLoss) {
				maxLoss = pnl
			}
		}
		avgLosingPnL = totalLosing.Div(decimal.NewFromInt(int64(len(losingPnLs))))
	}

	// 计算盈利因子
	if len(winningPnLs) > 0 && len(losingPnLs) > 0 {
		totalWinning := decimal.Zero
		totalLosing := decimal.Zero
		for _, pnl := range winningPnLs {
			totalWinning = totalWinning.Add(pnl)
		}
		for _, pnl := range losingPnLs {
			totalLosing = totalLosing.Add(pnl.Abs())
		}
		if totalLosing.IsPositive() {
			profitFactor = totalWinning.Div(totalLosing)
		}
	}

	return trades, openPositions, avgHoldingTime, maxHoldingTime, minHoldingTime, avgWinningPnL, avgLosingPnL, maxWin, maxLoss, profitFactor
}

// DrawdownInfo 回撤信息结构
type DrawdownInfo struct {
	MaxDrawdown        decimal.Decimal // 最大回撤金额
	MaxDrawdownPercent decimal.Decimal // 最大回撤百分比
	DrawdownDuration   time.Duration   // 最大回撤持续时间
	CurrentDrawdown    decimal.Decimal // 当前回撤
	PeakValue          decimal.Decimal // 历史最高价值
}

// CalculateDrawdownWithKlines 计算最大回撤（使用K线数据获取实时价格）
func CalculateDrawdownWithKlines(orders []executor.OrderResult, klines []*cex.KlineData, initialCapital decimal.Decimal) DrawdownInfo {
	if len(orders) == 0 || len(klines) == 0 {
		return DrawdownInfo{
			PeakValue: initialCapital,
		}
	}

	// 按时间排序订单
	ordersCopy := make([]executor.OrderResult, len(orders))
	copy(ordersCopy, orders)
	for i := 0; i < len(ordersCopy)-1; i++ {
		for j := i + 1; j < len(ordersCopy); j++ {
			if ordersCopy[i].Timestamp.After(ordersCopy[j].Timestamp) {
				ordersCopy[i], ordersCopy[j] = ordersCopy[j], ordersCopy[i]
			}
		}
	}

	currentCash := initialCapital
	peakValue := initialCapital
	maxDrawdown := decimal.Zero
	maxDrawdownPercent := decimal.Zero

	// 跟踪当前持仓
	var currentPositions []executor.OrderResult // 所有未平仓的买入订单
	orderIndex := 0

	// 🔥 关键修复：遍历每个K线时间点，而不是只在订单时间点
	for _, kline := range klines {
		// 处理当前K线时间之前的所有订单
		for orderIndex < len(ordersCopy) && !ordersCopy[orderIndex].Timestamp.After(kline.CloseTime) {
			order := ordersCopy[orderIndex]

			if order.Side == executor.OrderSideBuy {
				// 买入：现金减少，记录持仓
				currentCash = currentCash.Sub(order.Price.Mul(order.Quantity)).Sub(order.Commission)
				currentPositions = append(currentPositions, order)
			} else if order.Side == executor.OrderSideSell && len(currentPositions) > 0 {
				// 卖出：现金增加，移除第一个持仓（FIFO）
				sellValue := order.Price.Mul(order.Quantity)
				currentCash = currentCash.Add(sellValue).Sub(order.Commission)

				// 移除对应的买入订单（简化处理：FIFO）
				if len(currentPositions) > 0 {
					currentPositions = currentPositions[1:]
				}
			}
			orderIndex++
		}

		// 🔥 使用当前K线的收盘价估值所有持仓
		currentValue := currentCash
		marketPrice := kline.Close

		for _, position := range currentPositions {
			positionValue := position.Quantity.Mul(marketPrice)
			currentValue = currentValue.Add(positionValue)
		}

		// 更新峰值
		if currentValue.GreaterThan(peakValue) {
			peakValue = currentValue
		}

		// 计算当前回撤
		currentDrawdown := peakValue.Sub(currentValue)
		currentDrawdownPercent := decimal.Zero
		if peakValue.IsPositive() {
			currentDrawdownPercent = currentDrawdown.Div(peakValue).Mul(decimal.NewFromInt(100))
		}

		// 更新最大回撤
		if currentDrawdown.GreaterThan(maxDrawdown) {
			maxDrawdown = currentDrawdown
			maxDrawdownPercent = currentDrawdownPercent
		}
	}

	// 计算最终状态
	finalCash := currentCash
	for _, position := range currentPositions {
		// 使用最后一个K线价格估值剩余持仓
		if len(klines) > 0 {
			lastPrice := klines[len(klines)-1].Close
			finalCash = finalCash.Add(position.Quantity.Mul(lastPrice))
		}
	}

	currentDrawdown := peakValue.Sub(finalCash)

	return DrawdownInfo{
		MaxDrawdown:        maxDrawdown,
		MaxDrawdownPercent: maxDrawdownPercent,
		DrawdownDuration:   time.Duration(0), // 简化，暂不计算持续时间
		CurrentDrawdown:    currentDrawdown,
		PeakValue:          peakValue,
	}
}
