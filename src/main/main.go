package main

import (
	"context"
	tradingcmd "go-build-stream-gateway-go-server-main/src/cmd"
	"go-build-stream-gateway-go-server-main/src/config"
	"os"
	"path/filepath"

	"github.com/xpwu/go-cmd/cmd"
	"github.com/xpwu/go-config/configs"
	"github.com/xpwu/go-log/log"
)

func main() {
	// 设置 JSON 配置格式
	configs.SetConfigurator(&configs.JsonConfig{})

	// 智能查找配置文件
	setupConfigPath()

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

	// 交易对配置现在直接从主配置文件加载，无需单独的symbols.json文件

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

// setupConfigPath 智能设置配置文件路径
// 优先级: 1. bin/config.json 2. config.json 3. 生成默认配置
func setupConfigPath() {
	// 获取可执行文件所在目录
	execPath, err := os.Executable()
	if err != nil {
		return // 如果获取失败，使用默认行为
	}

	execDir := filepath.Dir(execPath)
	binConfigPath := filepath.Join(execDir, "config.json")

	// 检查 bin/config.json 是否存在
	if _, err := os.Stat(binConfigPath); err == nil {
		// 如果存在，切换工作目录到 bin 目录
		os.Chdir(execDir)
		return
	}

	// 检查当前目录的 config.json
	if _, err := os.Stat("config.json"); err == nil {
		return // 当前目录有配置文件，使用默认行为
	}

	// 都没有找到，保持当前目录不变，让程序生成默认配置
}
