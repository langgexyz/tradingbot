package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"tradingbot/src/strategy"
	"tradingbot/src/trading"

	"github.com/xpwu/go-cmd/arg"
	"github.com/xpwu/go-cmd/cmd"
	"github.com/xpwu/go-cmd/exe"
)

// RegisterBollingerTradingCmd 注册布林道交易命令
func RegisterBollingerTradingCmd() {
	RegisterBollingerCmd()
}

// RegisterBollingerCmd 注册统一的布林道交易命令
func RegisterBollingerCmd() {
	var configFile string
	var base string
	var quote string
	var timeframe string
	var cex string
	var live bool // 是否实盘交易
	var dry bool  // 是否Dry Run模式（实时运行但不真实下单）

	var startDate string
	var endDate string
	var initialCapital float64

	// 策略参数
	var period int
	var multiplier float64
	var positionSizePercent float64
	var minTradeAmount float64
	var stopLossPercent float64
	var takeProfitPercent float64
	var cooldownBars int

	// 卖出策略参数
	var sellStrategy string
	var sellStrategyParams string
	var listSellStrategies bool

	cmd.RegisterCmd("bollinger", "run Bollinger Bands trading (default: backtest)", func(args *arg.Arg) {
		args.String(&configFile, "c", "config file path")
		args.String(&base, "base", "base currency (e.g., BTC, ETH, PEPE, WIF)")
		args.String(&quote, "quote", "quote currency (e.g., USDT, USDC, BTC)")
		args.String(&timeframe, "t", "timeframe (e.g., 1h, 4h, 1d)")
		args.String(&cex, "cex", "centralized exchange (default: binance, currently only supports: binance)")
		args.Bool(&live, "live", "run in live trading mode (default: false, backtest mode)")
		args.Bool(&dry, "dry", "run in dry run mode (live data but no real orders)")

		// 回测参数
		args.String(&startDate, "start", "backtest start date (YYYY-MM-DD HH:MM:SS or YYYY-MM-DD, e.g., 2024-01-01 14:30:00) - required for backtest")
		args.String(&endDate, "end", "backtest end date (YYYY-MM-DD HH:MM:SS or YYYY-MM-DD, e.g., 2024-08-30)")
		args.Float64(&initialCapital, "capital", "initial capital (default: 10000.0)")

		// 策略参数
		args.Int(&period, "period", "Bollinger Bands period (default: 20)")
		args.Float64(&multiplier, "multiplier", "Bollinger Bands multiplier (default: 2.0)")
		args.Float64(&positionSizePercent, "position-size", "position size percent (default: 0.95)")
		args.Float64(&minTradeAmount, "min-trade", "minimum trade amount (default: 10.0)")
		args.Float64(&stopLossPercent, "stop-loss", "stop loss percent (default: 1.0, means no stop loss)")
		args.Float64(&takeProfitPercent, "take-profit", "take profit percent (default: 0.2)")
		args.Int(&cooldownBars, "cooldown", "cooldown bars (default: 1)")

		// 卖出策略参数
		args.String(&sellStrategy, "sell-strategy", "sell strategy (conservative, moderate, aggressive, trailing_5, trailing_10, combo_smart, partial_pyramid)")
		args.String(&sellStrategyParams, "sell-strategy-params", "sell strategy parameters (e.g., 'take_profit=0.25' for 25% fixed profit)")
		args.Bool(&listSellStrategies, "list-sell-strategies", "list all available sell strategies")

		args.Parse()

		// 如果只是列出卖出策略
		if listSellStrategies {
			listAvailableSellStrategies()
			return
		}

		// 设置策略参数默认值
		if period == 0 {
			period = 20
		}
		if multiplier == 0 {
			multiplier = 2.0
		}
		if positionSizePercent == 0 {
			positionSizePercent = 0.95
		}
		if minTradeAmount == 0 {
			minTradeAmount = 10.0
		}
		if stopLossPercent == 0 {
			stopLossPercent = 1.0 // 100% = 不止损
		}
		if takeProfitPercent == 0 {
			takeProfitPercent = 0.2 // 20%
		}
		if cooldownBars == 0 {
			cooldownBars = 1
		}

		// 设置卖出策略默认值
		if sellStrategy == "" {
			sellStrategy = "moderate" // 默认使用适中策略
		}

		// 设置默认配置文件路径
		if configFile == "" {
			configFile = "config.json"
		}

		// 确保配置文件路径是绝对路径
		if !filepath.IsAbs(configFile) {
			configFile = filepath.Join(exe.Exe.AbsDir, configFile)
		}

		// 验证必需参数
		if base == "" {
			fmt.Printf("❌ Error: base currency is required\n")
			if live {
				fmt.Printf("💡 Usage: ./bin/tradingbot bollinger -base BASE -quote QUOTE --live\n")
				fmt.Printf("   Example: ./bin/tradingbot bollinger -base PEPE -quote USDT --live\n")
			} else {
				fmt.Printf("💡 Usage: ./bin/tradingbot bollinger -base BASE -quote QUOTE -start YYYY-MM-DD [-end YYYY-MM-DD]\n")
				fmt.Printf("   Example: ./bin/tradingbot bollinger -base PEPE -quote USDT -start 2024-01-01\n")
			}
			os.Exit(1)
		}
		if quote == "" {
			fmt.Printf("❌ Error: quote currency is required\n")
			if live {
				fmt.Printf("💡 Usage: ./bin/tradingbot bollinger -base BASE -quote QUOTE --live\n")
				fmt.Printf("   Example: ./bin/tradingbot bollinger -base PEPE -quote USDT --live\n")
			} else {
				fmt.Printf("💡 Usage: ./bin/tradingbot bollinger -base BASE -quote QUOTE -start YYYY-MM-DD [-end YYYY-MM-DD]\n")
				fmt.Printf("   Example: ./bin/tradingbot bollinger -base PEPE -quote USDT -start 2024-01-01\n")
			}
			os.Exit(1)
		}

		// 回测模式需要开始日期（但实时dry run不需要）
		if !live && !dry && startDate == "" {
			fmt.Printf("❌ Error: start date is required for backtest mode\n")
			fmt.Printf("💡 Usage: ./bin/tradingbot bollinger -base BASE -quote QUOTE -start YYYY-MM-DD [-end YYYY-MM-DD]\n")
			fmt.Printf("   Example: ./bin/tradingbot bollinger -base PEPE -quote USDT -start 2024-01-01\n")
			fmt.Printf("🔴 For live trading: ./bin/tradingbot bollinger -base PEPE -quote USDT --live\n")
			fmt.Printf("📝 For dry run (real-time): ./bin/tradingbot bollinger -base PEPE -quote USDT --dry\n")
			fmt.Printf("📝 For dry run (historical): ./bin/tradingbot bollinger -base PEPE -quote USDT --dry -start 2024-01-01\n")
			os.Exit(1)
		}

		// 设置默认值
		if timeframe == "" {
			timeframe = "4h" // 默认时间周期
		}
		if cex == "" {
			cex = "binance" // 默认交易所
		}
		if initialCapital == 0 {
			initialCapital = 10000.0 // 默认初始资金
		}

		// 如果没有设置endDate，使用当前时间（回测模式或有start参数的dry模式）
		if !live && endDate == "" && startDate != "" {
			endDate = time.Now().Format("2006-01-02 15:04:05")
		}

		// 解析卖出策略参数
		var parsedSellParams map[string]float64
		var err error
		if sellStrategyParams != "" {
			parsedSellParams, err = strategy.ParseSellStrategyParams(sellStrategyParams)
			if err != nil {
				fmt.Printf("❌ Failed to parse sell strategy parameters: %v\n", err)
				os.Exit(1)
			}
		}

		// 创建策略参数
		strategyParams := &strategy.BollingerBandsParams{
			Period:              period,
			Multiplier:          multiplier,
			PositionSizePercent: positionSizePercent,
			MinTradeAmount:      minTradeAmount,
			StopLossPercent:     stopLossPercent,
			TakeProfitPercent:   takeProfitPercent,
			CooldownBars:        cooldownBars,
			SellStrategyName:    sellStrategy,
			SellStrategyParams:  parsedSellParams,
		}

		// 根据模式运行
		if live || (dry && startDate == "") {
			// 实时模式：真实交易或实时Dry Run
			err = runBollingerLiveWithPair(configFile, base, quote, timeframe, cex, initialCapital, strategyParams, dry)
		} else {
			// 回测模式：历史数据回测或Dry Run回测
			isDryBacktest := dry && startDate != ""
			err = runBollingerBacktestWithPair(configFile, base, quote, timeframe, cex, startDate, endDate, initialCapital, strategyParams, isDryBacktest)
		}

		if err != nil {
			fmt.Printf("❌ Trading system error: %v\n", err)
			os.Exit(1)
		}
	})
}

// runBollingerBacktestWithPair 运行布林道回测系统
func runBollingerBacktestWithPair(configPath, base, quote, timeframe, cex, startDate, endDate string, initialCapital float64, strategyParams *strategy.BollingerBandsParams, isDryBacktest bool) error {
	if isDryBacktest {
		fmt.Println("🤖 Bollinger Bands Dry Run System (Historical Data)")
	} else {
		fmt.Println("🤖 Bollinger Bands Trading System")
	}
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("📊 Trading Pair: %s/%s\n", base, quote)
	fmt.Printf("⏰ Timeframe: %s\n", timeframe)
	fmt.Printf("🏢 Exchange: %s\n", cex)

	// 创建交易系统
	fmt.Println("📋 Using global config")
	tradingSystem, err := trading.NewTradingSystem()
	if err != nil {
		return fmt.Errorf("failed to create trading system: %w", err)
	}

	// 设置交易对、时间周期和交易所
	pair := trading.CreateTradingPair(base, quote)
	err = tradingSystem.SetTradingPairTimeframeAndCEX(pair, timeframe, cex)
	if err != nil {
		return fmt.Errorf("failed to set trading pair, timeframe and CEX: %w", err)
	}

	// 设置信号处理
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-signalChan
		fmt.Println("\n🔄 Shutting down...")
		tradingSystem.Stop()
		os.Exit(0)
	}()

	// 运行回测
	if isDryBacktest {
		fmt.Printf("🧪 Running in dry run mode (historical data) from %s to %s...\n", startDate, endDate)
		fmt.Println("💡 Using historical data with simulated orders")
	} else {
		fmt.Printf("📊 Running in backtest mode from %s to %s...\n", startDate, endDate)
	}
	fmt.Printf("💰 Initial Capital: $%.2f\n", initialCapital)

	// 运行回测
	stats, err := tradingSystem.RunBacktestWithParamsAndCapital(pair, startDate, endDate, initialCapital, strategyParams)
	if err != nil {
		return fmt.Errorf("backtest failed: %w", err)
	}

	// 打印结果
	tradingSystem.PrintBacktestResults(pair, stats)

	return nil
}

// runBollingerLiveWithPair 运行布林道实盘交易
func runBollingerLiveWithPair(configFile, base, quote, timeframe, cex string, initialCapital float64, strategyParams *strategy.BollingerBandsParams, dryRun bool) error {
	fmt.Println("🤖 Bollinger Bands Live Trading System")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("📊 Trading Pair: %s/%s\n", base, quote)
	fmt.Printf("⏰ Timeframe: %s\n", timeframe)
	fmt.Printf("🏢 Exchange: %s\n", cex)
	fmt.Printf("📋 Using global config\n")

	// 创建交易系统
	fmt.Printf("🔧 Initializing trading system...\n")
	tradingSystem, err := trading.NewTradingSystem()
	if err != nil {
		return fmt.Errorf("failed to create trading system: %w", err)
	}
	defer tradingSystem.Stop()

	// 设置交易对和时间框架
	pair := trading.CreateTradingPair(base, quote)
	err = tradingSystem.SetTradingPairTimeframeAndCEX(pair, timeframe, cex)
	if err != nil {
		return fmt.Errorf("failed to set trading parameters: %w", err)
	}

	// 设置信号处理
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-signalChan
		fmt.Println("\n🔄 Shutting down...")
		tradingSystem.Stop()
		os.Exit(0)
	}()

	// 显示模式信息
	if dryRun {
		fmt.Println("🧪 Dry Run mode")
		fmt.Println("💡 Using real-time data with simulated orders")
	} else {
		fmt.Println("🔴 Live trading mode")
		fmt.Println("⚠️  WARNING: This will use real money!")
	}
	fmt.Println("Press Ctrl+C to stop...")

	// 运行实盘交易
	err = tradingSystem.RunLiveTradingWithParams(pair, strategyParams, dryRun)
	if err != nil {
		return fmt.Errorf("live trading failed: %w", err)
	}

	return nil
}

// listAvailableSellStrategies 列出所有可用的卖出策略
func listAvailableSellStrategies() {
	fmt.Printf("📋 Available Sell Strategies\n")
	fmt.Printf("==================================================\n")

	configs := strategy.GetDefaultSellStrategyConfigs()

	for name, config := range configs {
		fmt.Printf("🎯 %s\n", name)
		fmt.Printf("   Type: %s\n", config.Type)

		switch config.Type {
		case strategy.SellStrategyFixed:
			fmt.Printf("   Take Profit: %.1f%%\n", config.FixedTakeProfit*100)
			fmt.Printf("   Custom: --sell-strategy-params \"take_profit=0.25\" (for 25%%)\n")
		case strategy.SellStrategyTrailing:
			fmt.Printf("   Trailing: %.1f%% after %.1f%% profit\n",
				config.TrailingPercent*100, config.MinProfitForTrailing*100)
			fmt.Printf("   Custom: --sell-strategy-params \"trailing_percent=0.08,min_profit=0.18\"\n")
		case strategy.SellStrategyCombo:
			fmt.Printf("   Fixed: %.1f%%, Trailing: %.1f%% after %.1f%%\n",
				config.FixedTakeProfit*100, config.TrailingPercent*100, config.MinProfitForTrailing*100)
			fmt.Printf("   Max Holding: %d days\n", config.MaxHoldingDays)
			fmt.Printf("   Custom: --sell-strategy-params \"take_profit=0.22,trailing_percent=0.06\"\n")
		case strategy.SellStrategyPartial:
			fmt.Printf("   Levels: %d\n", len(config.PartialLevels))
			for i, level := range config.PartialLevels {
				fmt.Printf("     L%d: %.0f%% profit -> sell %.0f%%\n",
					i+1, level.ProfitPercent*100, level.SellPercent*100)
			}
			fmt.Printf("   Custom: (partial levels are complex, use defaults)\n")
		}
		fmt.Println()
	}

	fmt.Printf("💡 Parameter Usage Examples:\n")
	fmt.Printf("   Fixed 25%% profit: --sell-strategy conservative --sell-strategy-params \"take_profit=0.25\"\n")
	fmt.Printf("   Trailing 8%% after 18%% profit: --sell-strategy trailing_5 --sell-strategy-params \"trailing_percent=0.08,min_profit=0.18\"\n")
	fmt.Printf("   Custom aggressive 35%%: --sell-strategy aggressive --sell-strategy-params \"take_profit=0.35\"\n")
	fmt.Println()
}

// RegisterAllTradingCommands 注册所有交易相关命令
func RegisterAllTradingCommands() {
	RegisterBollingerTradingCmd()

	// 可以添加其他交易策略命令
	// RegisterMACDTradingCmd()
	// RegisterRSITradingCmd()
}
