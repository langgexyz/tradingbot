package cmd

import (
	"context"
	"fmt"
	"time"

	"go-build-stream-gateway-go-server-main/src/binance"
	"go-build-stream-gateway-go-server-main/src/config"

	"github.com/shopspring/decimal"
	"github.com/xpwu/go-cmd/arg"
	"github.com/xpwu/go-cmd/cmd"
)

// RegisterKlineTestCmd 注册K线数据测试命令
func RegisterKlineTestCmd() {
	var symbol string
	var interval string
	var limit int
	var verbose bool

	cmd.RegisterCmd("kline", "test Kline data fetching from Binance API", func(args *arg.Arg) {
		args.String(&symbol, "s", "trading symbol (default: BTCUSDT)")
		args.String(&interval, "i", "kline interval (default: 1h)")
		args.Int(&limit, "l", "number of klines (default: 10, max: 1000)")
		args.Bool(&verbose, "v", "verbose output with detailed information")
		args.Parse()

		// 设置默认值
		if symbol == "" {
			symbol = "BTCUSDT"
		}
		if interval == "" {
			interval = "1h"
		}
		if limit <= 0 {
			limit = 10
		}
		if limit > 1000 {
			limit = 1000
		}

		err := runKlineTest(symbol, interval, limit, verbose)
		if err != nil {
			fmt.Printf("❌ K线数据测试失败: %v\n", err)
			return
		}
	})
}

// runKlineTest 执行K线数据测试
func runKlineTest(symbol, interval string, limit int, verbose bool) error {
	fmt.Printf("📊 K线数据获取测试\n")
	fmt.Printf("================================\n")
	fmt.Printf("🔸 交易对: %s\n", symbol)
	fmt.Printf("🔸 时间周期: %s\n", interval)
	fmt.Printf("🔸 数据条数: %d\n", limit)
	fmt.Printf("🔸 数据源: %s\n", config.AppConfig.CEX.Binance.BaseURL)
	fmt.Println()

	// 创建币安客户端（K线数据获取无需API密钥）
	client := binance.NewClient("", "", config.AppConfig.CEX.Binance.BaseURL)

	// 创建上下文
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Print("🔄 正在获取K线数据...")
	startTime := time.Now()

	// 获取K线数据
	klines, err := client.GetKlines(ctx, symbol, interval, limit)
	if err != nil {
		fmt.Printf("\n❌ 获取失败: %v\n", err)
		return err
	}

	duration := time.Since(startTime)
	fmt.Printf(" 完成! (耗时: %v)\n", duration)

	if len(klines) == 0 {
		fmt.Println("⚠️ 未获取到数据")
		return nil
	}

	fmt.Printf("✅ 成功获取 %d 条K线数据\n\n", len(klines))

	// 显示数据概览
	fmt.Println("📈 数据概览:")
	fmt.Printf("├─ 最新时间: %s\n", formatTime(klines[len(klines)-1].OpenTime))
	fmt.Printf("├─ 最早时间: %s\n", formatTime(klines[0].OpenTime))
	fmt.Printf("├─ 最新价格: %s USDT\n", klines[len(klines)-1].Close.String())
	fmt.Printf("└─ 最新成交量: %s %s\n", klines[len(klines)-1].Volume.String(), getBaseCurrency(symbol))
	fmt.Println()

	if verbose {
		// 显示详细数据
		fmt.Println("📋 详细K线数据 (最近5条):")
		fmt.Println("时间                  | 开盘价    | 最高价    | 最低价    | 收盘价    | 成交量")
		fmt.Println("---------------------|----------|----------|----------|----------|----------")

		displayCount := 5
		if len(klines) < 5 {
			displayCount = len(klines)
		}

		for i := len(klines) - displayCount; i < len(klines); i++ {
			kline := klines[i]
			fmt.Printf("%s | %8s | %8s | %8s | %8s | %8s\n",
				formatTime(kline.OpenTime),
				formatPrice(kline.Open),
				formatPrice(kline.High),
				formatPrice(kline.Low),
				formatPrice(kline.Close),
				formatVolume(kline.Volume),
			)
		}
		fmt.Println()

		// 计算价格统计
		if len(klines) >= 2 {
			latest := klines[len(klines)-1]
			previous := klines[len(klines)-2]

			priceChange := latest.Close.Sub(previous.Close)
			priceChangePercent := priceChange.Div(previous.Close).Mul(decimal.NewFromFloat(100))

			fmt.Println("📊 价格变化:")
			fmt.Printf("├─ 价格变化: %s USDT\n", priceChange.String())
			fmt.Printf("├─ 变化幅度: %s%%\n", priceChangePercent.StringFixed(2))

			if priceChange.IsPositive() {
				fmt.Printf("└─ 趋势: 📈 上涨\n")
			} else if priceChange.IsNegative() {
				fmt.Printf("└─ 趋势: 📉 下跌\n")
			} else {
				fmt.Printf("└─ 趋势: ➡️ 平盘\n")
			}
		}
	}

	fmt.Println("\n✅ K线数据测试完成!")
	return nil
}

// formatTime 格式化时间戳
func formatTime(timestamp int64) string {
	return time.Unix(timestamp/1000, 0).Format("01-02 15:04")
}

// formatPrice 格式化价格
func formatPrice(price decimal.Decimal) string {
	return price.StringFixed(2)
}

// formatVolume 格式化成交量
func formatVolume(volume decimal.Decimal) string {
	if volume.GreaterThan(decimal.NewFromFloat(1000)) {
		return volume.Div(decimal.NewFromFloat(1000)).StringFixed(1) + "K"
	}
	return volume.StringFixed(2)
}

// getBaseCurrency 获取基础货币
func getBaseCurrency(symbol string) string {
	if len(symbol) >= 3 {
		// 简单的货币对解析，假设以USDT结尾
		if symbol[len(symbol)-4:] == "USDT" {
			return symbol[:len(symbol)-4]
		}
		if symbol[len(symbol)-3:] == "BTC" {
			return symbol[:len(symbol)-3]
		}
	}
	return "BASE"
}
