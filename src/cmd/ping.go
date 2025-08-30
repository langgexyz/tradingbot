package cmd

import (
	"context"
	"fmt"
	"math"
	"time"

	"go-build-stream-gateway-go-server-main/src/binance"
	"go-build-stream-gateway-go-server-main/src/config"

	"github.com/xpwu/go-cmd/arg"
	"github.com/xpwu/go-cmd/cmd"
)

// RegisterPingCmd 注册ping测试命令
func RegisterPingCmd() {
	var verbose bool
	var timeout int

	cmd.RegisterCmd("ping", "test connectivity to Binance API server", func(args *arg.Arg) {
		args.Bool(&verbose, "v", "verbose output with detailed information")
		args.Int(&timeout, "t", "timeout in seconds (default: 10)")
		args.Parse()

		// 设置默认超时
		if timeout <= 0 {
			timeout = 10
		}

		err := runPingTest(verbose, timeout)
		if err != nil {
			fmt.Printf("❌ Ping test failed: %v\n", err)
			return
		}
		fmt.Println("✅ Ping test successful!")
	})
}

// runPingTest 执行ping测试
func runPingTest(verbose bool, timeoutSeconds int) error {
	if verbose {
		fmt.Println("🌐 币安API连通性测试")
		fmt.Println("================================")
		fmt.Printf("📡 目标服务器: %s\n", config.AppConfig.Binance.BaseURL)
		fmt.Printf("⏰ 超时时间: %d秒\n", timeoutSeconds)
		fmt.Println()
	}

	// 创建币安客户端（不需要API密钥进行ping测试）
	client := binance.NewClient("", "", config.AppConfig.Binance.BaseURL)

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	if verbose {
		fmt.Print("🔄 正在测试连接...")
	}

	// 记录开始时间
	startTime := time.Now()

	// 执行ping测试
	err := client.Ping(ctx)

	// 计算延迟
	latency := time.Since(startTime)

	if err != nil {
		if verbose {
			fmt.Printf("\n❌ 连接失败: %v\n", err)
			fmt.Printf("⏱️ 测试耗时: %v\n", latency)
		}
		return err
	}

	if verbose {
		fmt.Printf(" 完成!\n")
		fmt.Printf("✅ 服务器响应正常\n")
		fmt.Printf("⏱️ 响应延迟: %v\n", latency)
		fmt.Println()

		// 获取服务器时间进行额外验证
		fmt.Print("🕐 获取服务器时间...")
		serverTime, timeErr := client.GetServerTime(ctx)
		if timeErr == nil {
			fmt.Printf(" %v\n", serverTime.Format("2006-01-02 15:04:05 MST"))

			// 计算时间差
			localTime := time.Now()
			timeDiff := int64(math.Abs(float64(serverTime.Unix() - localTime.Unix())))
			fmt.Printf("⏰ 本地时间差: %ds", timeDiff)

			if timeDiff > 60 {
				fmt.Printf(" ⚠️ 时间差较大，可能影响API调用")
			}
			fmt.Println()
		} else {
			fmt.Printf(" 失败: %v\n", timeErr)
		}

		fmt.Println()
		fmt.Println("📊 连接状态: 正常")
		fmt.Printf("🌍 网络质量: ")
		if latency < 100*time.Millisecond {
			fmt.Println("优秀")
		} else if latency < 300*time.Millisecond {
			fmt.Println("良好")
		} else if latency < 1000*time.Millisecond {
			fmt.Println("一般")
		} else {
			fmt.Println("较差")
		}
	}

	return nil
}
