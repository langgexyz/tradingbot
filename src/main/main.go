package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	tradingcmd "tradingbot/src/cmd"
	// 导入各模块配置，让 go-config 自动加载
	_ "tradingbot/src/cex/binance" // 导入 Binance 配置和工厂注册
	_ "tradingbot/src/database"
	_ "tradingbot/src/trading"

	"github.com/xpwu/go-cmd/arg"
	"github.com/xpwu/go-cmd/cmd"
	"github.com/xpwu/go-cmd/exe"
	"github.com/xpwu/go-config/configs"
	"github.com/xpwu/go-log/log"
)

func main() {
	// 注册默认命令
	cmd.RegisterCmd(cmd.DefaultCmdName, "start trading bot", func(args *arg.Arg) {
		arg.ReadConfig(args)
		args.Parse()

		_, logger := log.WithCtx(context.Background())
		logger.PushPrefix("TradingBot")
		logger.Info("交易机器人启动")

		// 注册交易相关命令
		tradingcmd.RegisterAllTradingCommands()

		// 运行命令行程序
		cmd.Run()
	})

	// 注册打印配置命令
	var configFile string = "config.json"
	cmd.RegisterCmd("print", "print config with json", func(args *arg.Arg) {
		args.String(&configFile, "c", "the file name of config file")
		args.Parse()
		if !filepath.IsAbs(configFile) {
			configFile = filepath.Join(exe.Exe.AbsDir, configFile)
		}
		configs.SetConfigurator(&configs.JsonConfig{PrintFile: configFile})
		err := configs.Print()
		if err != nil {
			fmt.Println(err)
			os.Exit(-1)
		}
	})

	cmd.Run()
}
