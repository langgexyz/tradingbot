package main

import (
	"context"
	tradingcmd "go-build-stream-gateway-go-server-main/src/cmd"
	"go-build-stream-gateway-go-server-main/src/config"

	"github.com/xpwu/go-cmd/cmd"
	"github.com/xpwu/go-config/configs"
	"github.com/xpwu/go-log/log"
)

func main() {
	// 设置 JSON 配置格式
	configs.SetConfigurator(&configs.JsonConfig{})

	// 读取配置文件
	err := configs.ReadWithErr()
	if err != nil {
		// 如果读取失败，生成默认配置文件
		printErr := configs.Print()
		if printErr != nil {
			panic("生成默认配置文件失败: " + printErr.Error())
		}
		panic("请修改 config.json 配置文件后重新运行")
	}

	// 验证配置
	if err := config.AppConfig.Validate(); err != nil {
		panic("配置验证失败: " + err.Error())
	}

	// 创建带上下文的日志
	ctx := context.Background()
	ctx, logger := log.WithCtx(ctx)
	logger.PushPrefix("TradingBot")
	logger.Info("交易机器人启动")

	// 注册交易相关命令
	tradingcmd.RegisterAllTradingCommands()

	// 运行命令行程序
	cmd.Run()
}
