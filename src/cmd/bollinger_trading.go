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
	RegisterBollingerBacktestCmd()
	RegisterBollingerLiveCmd()
}

// RegisterBollingerBacktestCmd 注册布林道回测命令
func RegisterBollingerBacktestCmd() {
	var configFile string
	var base string
	var quote string
	var timeframe string
	var cex string

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

	cmd.RegisterCmd("bollinger-backtest", "run Bollinger Bands backtest", func(args *arg.Arg) {
		args.String(&configFile, "c", "config file path")
		args.String(&base, "base", "base currency (e.g., BTC, ETH, PEPE, WIF)")
		args.String(&quote, "quote", "quote currency (e.g., USDT, USDC, BTC)")
		args.String(&timeframe, "t", "timeframe (e.g., 1h, 4h, 1d)")
		args.String(&cex, "cex", "centralized exchange (default: binance, currently only supports: binance)")
		args.String(&startDate, "start", "backtest start date (YYYY-MM-DD, e.g., 2024-01-01)")
		args.String(&endDate, "end", "backtest end date (YYYY-MM-DD, e.g., 2024-08-30)")
		args.Float64(&initialCapital, "capital", "initial capital for backtest (default: 10000.0)")

		// 策略参数
		args.Int(&period, "period", "Bollinger Bands period (default: 20)")
		args.Float64(&multiplier, "multiplier", "Bollinger Bands multiplier (default: 2.0)")
		args.Float64(&positionSizePercent, "position-size", "position size percent (default: 0.95)")
		args.Float64(&minTradeAmount, "min-trade", "minimum trade amount (default: 10.0)")
		args.Float64(&stopLossPercent, "stop-loss", "stop loss percent (default: 1.0, means no stop loss)")
		args.Float64(&takeProfitPercent, "take-profit", "take profit percent (default: 0.2)")
		args.Int(&cooldownBars, "cooldown", "cooldown bars (default: 1)")

		args.Parse()

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

		// 确保配置文件路径是绝对路径
		if !filepath.IsAbs(configFile) {
			configFile = filepath.Join(exe.Exe.AbsDir, configFile)
		}

		// 验证必需参数
		if base == "" {
			fmt.Printf("❌ Error: base currency is required\n")
			fmt.Printf("💡 Usage: ./bin/tradingbot bollinger -base BASE -quote QUOTE -start YYYY-MM-DD [-end YYYY-MM-DD]\n")
			fmt.Printf("   Example: ./bin/tradingbot bollinger -base PEPE -quote USDT -start 2024-01-01\n")
			os.Exit(1)
		}
		if quote == "" {
			fmt.Printf("❌ Error: quote currency is required\n")
			fmt.Printf("💡 Usage: ./bin/tradingbot bollinger -base BASE -quote QUOTE -start YYYY-MM-DD [-end YYYY-MM-DD]\n")
			fmt.Printf("   Example: ./bin/tradingbot bollinger -base PEPE -quote USDT -start 2024-01-01\n")
			os.Exit(1)
		}
		if startDate == "" {
			fmt.Printf("❌ Error: start date is required\n")
			fmt.Printf("💡 Usage: ./bin/tradingbot bollinger -base BASE -quote QUOTE -start YYYY-MM-DD [-end YYYY-MM-DD]\n")
			fmt.Printf("   Example: ./bin/tradingbot bollinger -base PEPE -quote USDT -start 2024-01-01\n")
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

		// 如果没有设置endDate，使用当前时间
		if endDate == "" {
			endDate = time.Now().Format("2006-01-02")
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
		}

		// 运行回测系统
		err := runBollingerBacktestWithPair(configFile, base, quote, timeframe, cex, startDate, endDate, initialCapital, strategyParams)
		if err != nil {
			fmt.Printf("❌ Trading system error: %v\n", err)
			os.Exit(1)
		}
	})
}

// runBollingerBacktestWithPair 运行布林道回测系统
func runBollingerBacktestWithPair(configPath, base, quote, timeframe, cex, startDate, endDate string, initialCapital float64, strategyParams *strategy.BollingerBandsParams) error {
	fmt.Println("🤖 Bollinger Bands Trading System")
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

	// 创建交易对
	pair := CreateTradingPair(base, quote)

	// 设置交易对、时间周期和交易所
	err = tradingSystem.SetTradingPairTimeframeAndCEX(pair, timeframe, cex)
	if err != nil {
		return fmt.Errorf("failed to set trading pair, timeframe and CEX: %w", err)
	}

	// 初始化
	fmt.Println("🔧 Initializing trading system...")
	err = tradingSystem.Initialize()
	if err != nil {
		return fmt.Errorf("failed to initialize trading system: %w", err)
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
	fmt.Printf("📊 Running in backtest mode from %s to %s...\n", startDate, endDate)
	fmt.Printf("💰 Initial Capital: $%.2f\n", initialCapital)
	stats, err := tradingSystem.RunBacktestWithParamsAndCapital(pair, startDate, endDate, initialCapital, strategyParams)
	if err != nil {
		return fmt.Errorf("backtest failed: %w", err)
	}

	// 打印结果
	tradingSystem.PrintBacktestResults(pair, stats)

	return nil
}

// RegisterBollingerLiveCmd 注册布林道实盘交易命令
func RegisterBollingerLiveCmd() {
	var configFile string
	var base string
	var quote string
	var timeframe string
	var cex string
	var initialCapital float64

	// 策略参数
	var period int
	var multiplier float64
	var positionSizePercent float64
	var minTradeAmount float64
	var stopLossPercent float64
	var takeProfitPercent float64
	var cooldownBars int

	cmd.RegisterCmd("bollinger-live", "run Bollinger Bands live trading", func(args *arg.Arg) {
		args.String(&configFile, "c", "config file path")
		args.String(&base, "base", "base currency (e.g., BTC, ETH, PEPE, WIF)")
		args.String(&quote, "quote", "quote currency (e.g., USDT, USDC, BTC)")
		args.String(&timeframe, "t", "timeframe (e.g., 1h, 4h, 1d)")
		args.String(&cex, "cex", "centralized exchange (default: binance, currently only supports: binance)")
		args.Float64(&initialCapital, "capital", "initial capital for trading (default: 10000.0)")

		// 策略参数
		args.Int(&period, "period", "Bollinger Bands period (default: 20)")
		args.Float64(&multiplier, "multiplier", "Bollinger Bands multiplier (default: 2.0)")
		args.Float64(&positionSizePercent, "position-size", "position size percent (default: 0.95)")
		args.Float64(&minTradeAmount, "min-trade", "minimum trade amount (default: 10.0)")
		args.Float64(&stopLossPercent, "stop-loss", "stop loss percent (default: 1.0, means no stop loss)")
		args.Float64(&takeProfitPercent, "take-profit", "take profit percent (default: 0.2)")
		args.Int(&cooldownBars, "cooldown", "cooldown bars (default: 1)")

		args.Parse()

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
			fmt.Printf("💡 Usage: ./bin/tradingbot bollinger-live -base BASE -quote QUOTE\n")
			fmt.Printf("📝 Example: ./bin/tradingbot bollinger-live -base PEPE -quote USDT\n")
			os.Exit(1)
		}

		if quote == "" {
			fmt.Printf("❌ Error: quote currency is required\n")
			fmt.Printf("💡 Usage: ./bin/tradingbot bollinger-live -base BASE -quote QUOTE\n")
			fmt.Printf("📝 Example: ./bin/tradingbot bollinger-live -base PEPE -quote USDT\n")
			os.Exit(1)
		}

		// 设置默认值
		if timeframe == "" {
			timeframe = "4h"
		}
		if cex == "" {
			cex = "binance"
		}
		if initialCapital == 0 {
			initialCapital = 10000.0
		}

		// Set default values if not provided
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
			stopLossPercent = 1.0
		} // 100% = 不止损
		if takeProfitPercent == 0 {
			takeProfitPercent = 0.2
		} // 20%
		if cooldownBars == 0 {
			cooldownBars = 1
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
		}

		// 运行实盘交易
		err := runBollingerLiveWithPair(configFile, base, quote, timeframe, cex, initialCapital, strategyParams)
		if err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			os.Exit(1)
		}
	})
}

// runBollingerLiveWithPair 运行布林道实盘交易
func runBollingerLiveWithPair(configFile, base, quote, timeframe, cex string, initialCapital float64, strategyParams *strategy.BollingerBandsParams) error {
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

	// 创建交易对
	pair := CreateTradingPair(base, quote)

	// 设置交易对和时间框架
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

	// 运行实盘交易
	fmt.Println("🔴 Live trading mode")
	fmt.Println("⚠️  WARNING: This will use real money!")
	fmt.Println("Press Ctrl+C to stop...")

	err = tradingSystem.RunLiveTradingWithParams(pair, strategyParams)
	if err != nil {
		return fmt.Errorf("live trading failed: %w", err)
	}

	return nil
}

// RegisterAllTradingCommands 注册所有交易相关命令
func RegisterAllTradingCommands() {
	RegisterBollingerTradingCmd()

	// 可以添加其他交易策略命令
	// RegisterMACDTradingCmd()
	// RegisterRSITradingCmd()
}
