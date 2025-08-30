package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"go-build-stream-gateway-go-server-main/src/config"
	"go-build-stream-gateway-go-server-main/src/trading"

	"github.com/xpwu/go-cmd/arg"
	"github.com/xpwu/go-cmd/cmd"
	"github.com/xpwu/go-cmd/exe"
)

// RegisterBollingerTradingCmd 注册布林道交易命令
func RegisterBollingerTradingCmd() {
	var configFile string
	var createConfig bool
	var symbol string
	var timeframe string
	var cex string
	var listSymbols bool

	cmd.RegisterCmd("bollinger", "run Bollinger Bands trading strategy", func(args *arg.Arg) {
		args.String(&configFile, "c", "config file path")
		args.Bool(&createConfig, "create-config", "create default config file and exit")
		args.String(&symbol, "s", "trading symbol (e.g., BTCUSDT, WIFUSDT, ETHUSDT)")
		args.String(&timeframe, "t", "timeframe (e.g., 1h, 4h, 1d)")
		args.String(&cex, "cex", "centralized exchange (currently only supports: binance)")
		args.Bool(&listSymbols, "list", "list all supported trading pairs and exit")
		args.Parse()

		// 确保配置文件路径是绝对路径
		if !filepath.IsAbs(configFile) {
			configFile = filepath.Join(exe.Exe.AbsDir, configFile)
		}

		// 如果只是创建配置文件
		if createConfig {
			err := createDefaultConfig(configFile)
			if err != nil {
				fmt.Printf("❌ Failed to create config: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("✅ Default config created at: %s\n", configFile)
			fmt.Println("📝 Please edit the config file and set your Binance API credentials")
			os.Exit(0)
		}

		// 如果只是列出支持的交易对
		if listSymbols {
			err := listSupportedSymbols()
			if err != nil {
				fmt.Printf("❌ Failed to list symbols: %v\n", err)
				os.Exit(1)
			}
			os.Exit(0)
		}

		// 设置默认值
		if symbol == "" {
			symbol = "BTCUSDT" // 默认交易对
		}
		if timeframe == "" {
			timeframe = "4h" // 默认时间周期
		}
		if cex == "" {
			cex = "binance" // 默认交易所
		}

		// 运行交易系统
		err := runBollingerTrading(configFile, symbol, timeframe, cex)
		if err != nil {
			fmt.Printf("❌ Trading system error: %v\n", err)
			os.Exit(1)
		}
	})
}

// createDefaultConfig 创建默认配置文件
func createDefaultConfig(configPath string) error {
	// 这里可以使用config包的默认配置
	configContent := `{
  "binance": {
    "api_key": "YOUR_API_KEY_HERE",
    "secret_key": "YOUR_SECRET_KEY_HERE"
  },
  "trading": {
    "symbol": "BTCUSDT",
    "timeframe": "4h",
    "initial_capital": 10000,
    "mode": "backtest",
    "max_positions": 1,
    "position_size_percent": 0.95,
    "min_trade_amount": 10
  },
  "strategy": {
    "name": "bollinger_bands",
    "parameters": {
      "period": 20,
      "multiplier": 2.0,
      "position_size_percent": 0.95,
      "min_trade_amount": 10
    }
  },
  "backtest": {
    "start_date": "2023-01-01",
    "end_date": "2023-12-31",
    "fee": 0.001,
    "slippage": 0.0001,
    "data_source": "binance"
  },
  "logging": {
    "level": "info",
    "output": "both",
    "file_path": "./logs/trading.log",
    "max_age": 30
  }
}`

	// 创建目录
	dir := filepath.Dir(configPath)
	if dir != "" {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}
	}

	// 写入文件
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// listSupportedSymbols 列出所有支持的交易对
func listSupportedSymbols() error {
	fmt.Println("📊 Supported Trading Pairs")
	fmt.Println(strings.Repeat("=", 50))

	// 从主配置获取支持的交易对
	symbols := make([]string, 0, len(config.AppConfig.Symbols))
	for _, s := range config.AppConfig.Symbols {
		symbols = append(symbols, s.Symbol)
	}

	if len(symbols) == 0 {
		fmt.Println("❌ No trading pairs found in configuration")
		fmt.Println("💡 Please check symbols section in bin/config.json file")
		return nil
	}

	// 按类别分组显示
	fmt.Printf("✅ Found %d supported trading pairs:\n\n", len(symbols))

	// 分类显示
	btcPairs := []string{}
	ethPairs := []string{}
	usdtPairs := []string{}
	usdcPairs := []string{}
	otherPairs := []string{}

	for _, symbol := range symbols {
		switch {
		case strings.HasPrefix(symbol, "BTC"):
			btcPairs = append(btcPairs, symbol)
		case strings.HasPrefix(symbol, "ETH"):
			ethPairs = append(ethPairs, symbol)
		case strings.HasSuffix(symbol, "USDT"):
			usdtPairs = append(usdtPairs, symbol)
		case strings.HasSuffix(symbol, "USDC"):
			usdcPairs = append(usdcPairs, symbol)
		default:
			otherPairs = append(otherPairs, symbol)
		}
	}

	if len(btcPairs) > 0 {
		fmt.Printf("🟠 BTC Pairs: %s\n", strings.Join(btcPairs, ", "))
	}
	if len(ethPairs) > 0 {
		fmt.Printf("🔵 ETH Pairs: %s\n", strings.Join(ethPairs, ", "))
	}
	if len(usdtPairs) > 0 {
		fmt.Printf("🟢 USDT Pairs: %s\n", strings.Join(usdtPairs, ", "))
	}
	if len(usdcPairs) > 0 {
		fmt.Printf("🟡 USDC Pairs: %s\n", strings.Join(usdcPairs, ", "))
	}
	if len(otherPairs) > 0 {
		fmt.Printf("⚪ Other Pairs: %s\n", strings.Join(otherPairs, ", "))
	}

	fmt.Println("\n💡 Usage: ./bin/tradingbot bollinger -s <SYMBOL>")
	fmt.Println("   Example: ./bin/tradingbot bollinger -s BTCUSDT")

	return nil
}

// runBollingerTrading 运行布林道交易系统
func runBollingerTrading(configPath, symbol, timeframe, cex string) error {
	fmt.Println("🤖 Bollinger Bands Trading System")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("📊 Trading Pair: %s\n", symbol)
	fmt.Printf("⏰ Timeframe: %s\n", timeframe)
	fmt.Printf("🏢 Exchange: %s\n", cex)

	// 创建交易系统
	fmt.Println("📋 Using global config")
	tradingSystem, err := trading.NewTradingSystem()
	if err != nil {
		return fmt.Errorf("failed to create trading system: %w", err)
	}

	// 设置交易对、时间周期和交易所
	err = tradingSystem.SetSymbolTimeframeAndCEX(symbol, timeframe, cex)
	if err != nil {
		return fmt.Errorf("failed to set symbol, timeframe and CEX: %w", err)
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

	// 根据配置运行不同模式
	config := tradingSystem.GetConfig()

	switch {
	case config.IsBacktestMode():
		fmt.Println("📊 Running in backtest mode...")
		stats, err := tradingSystem.RunBacktest()
		if err != nil {
			return fmt.Errorf("backtest failed: %w", err)
		}

		// 打印结果
		tradingSystem.PrintBacktestResults(stats)

	case config.IsPaperMode():
		fmt.Println("📝 Paper trading mode not implemented yet")
		return fmt.Errorf("paper trading not implemented")

	case config.IsLiveMode():
		fmt.Println("🔴 Live trading mode")
		fmt.Println("⚠️  WARNING: This will use real money!")
		fmt.Println("Press Ctrl+C to stop...")

		err := tradingSystem.RunLiveTrading()
		if err != nil {
			return fmt.Errorf("live trading failed: %w", err)
		}

	default:
		return fmt.Errorf("unknown trading mode: %s", config.Trading.Mode)
	}

	return nil
}

// RegisterAllTradingCommands 注册所有交易相关命令
func RegisterAllTradingCommands() {
	RegisterBollingerTradingCmd()
	RegisterPingCmd()
	RegisterKlineTestCmd()

	// 可以添加其他交易策略命令
	// RegisterMACDTradingCmd()
	// RegisterRSITradingCmd()
}
