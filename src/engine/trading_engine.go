package engine

import (
	"context"
	"fmt"
	"sort"
	"time"

	"tradingbot/src/cex"
	"tradingbot/src/executor"
	"tradingbot/src/strategy"
	"tradingbot/src/timeframes"

	"github.com/shopspring/decimal"
	"github.com/xpwu/go-log/log"
)

// TradingEngine 统一的交易引擎（支持回测和实盘）
type TradingEngine struct {
	tradingPair cex.TradingPair
	timeframe   timeframes.Timeframe
	strategy    strategy.Strategy
	executor    executor.Executor
	cexClient   cex.CEXClient

	// 配置
	positionSizePercent decimal.Decimal
	minTradeAmount      decimal.Decimal

	// 信号处理器
	signalRegistry *SignalHandlerRegistry

	// 运行状态
	isRunning bool
	stopChan  chan struct{}
}

// NewTradingEngine 创建交易引擎
func NewTradingEngine(
	pair cex.TradingPair,
	timeframe timeframes.Timeframe,
	strategy strategy.Strategy,
	executor executor.Executor,
	cexClient cex.CEXClient,
) *TradingEngine {
	engine := &TradingEngine{
		tradingPair:         pair,
		timeframe:           timeframe,
		strategy:            strategy,
		executor:            executor,
		cexClient:           cexClient,
		positionSizePercent: decimal.NewFromFloat(0.95),
		minTradeAmount:      decimal.NewFromFloat(10.0),
		stopChan:            make(chan struct{}),
	}

	// 初始化信号处理器注册表
	engine.signalRegistry = NewSignalHandlerRegistry()

	// 注册默认的信号处理器
	buyHandler := NewBuySignalHandler(executor, pair, engine.positionSizePercent, engine.minTradeAmount)
	sellHandler := NewSellSignalHandler(executor, pair)

	engine.signalRegistry.RegisterHandler("BUY", buyHandler)
	engine.signalRegistry.RegisterHandler("SELL", sellHandler)

	return engine
}

// SetPositionSizePercent 设置仓位比例
func (e *TradingEngine) SetPositionSizePercent(percent float64) {
	e.positionSizePercent = decimal.NewFromFloat(percent)
}

// SetMinTradeAmount 设置最小交易金额
func (e *TradingEngine) SetMinTradeAmount(amount float64) {
	e.minTradeAmount = decimal.NewFromFloat(amount)
}

// RunBacktest 运行回测
func (e *TradingEngine) RunBacktest(ctx context.Context, startTime, endTime time.Time) error {
	ctx, logger := log.WithCtx(ctx)
	logger.PushPrefix("TradingEngine")

	logger.Info(fmt.Sprintf("开始回测: symbol=%s, timeframe=%s, start=%s, end=%s",
		e.tradingPair.String(),
		e.timeframe.String(),
		startTime.Format("2006-01-02"),
		endTime.Format("2006-01-02")))

	// 获取历史数据
	startTimeMs := startTime.Unix() * 1000
	endTimeMs := endTime.Unix() * 1000

	// 直接从 CEX 客户端获取数据
	logger.Info("使用CEX客户端获取数据")
	tradingPair := e.tradingPair
	klines, err := e.cexClient.GetKlinesWithTimeRange(ctx, tradingPair, e.timeframe.GetBinanceInterval(),
		time.Unix(startTimeMs/1000, 0), time.Unix(endTimeMs/1000, 0), 1000)

	if err != nil {
		return fmt.Errorf("failed to load historical data: %w", err)
	}

	if len(klines) == 0 {
		return fmt.Errorf("no historical data available")
	}

	// 按时间排序
	sort.Slice(klines, func(i, j int) bool {
		return klines[i].OpenTime.Before(klines[j].OpenTime)
	})

	logger.Info(fmt.Sprintf("加载历史数据完成: klines=%d", len(klines)))

	// 逐个处理K线数据
	for i, kline := range klines {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// 获取当前投资组合状态
		portfolio, err := e.executor.GetPortfolio(ctx)
		if err != nil {
			logger.Error("获取投资组合失败", "error", err)
			continue
		}

		// 更新当前价格
		portfolio.CurrentPrice = kline.Close
		portfolio.Timestamp = kline.OpenTime

		// 执行策略
		signals, err := e.strategy.OnData(ctx, kline, portfolio)
		if err != nil {
			logger.Error("策略执行失败", "error", err)
			continue
		}

		// 处理交易信号
		for _, signal := range signals {
			err := e.processSignal(ctx, signal, kline, portfolio)
			if err != nil {
				logger.Error("处理交易信号失败", "error", err)
			}
		}

		// 定期输出进度
		if i%100 == 0 || i == len(klines)-1 {
			progress := float64(i+1) / float64(len(klines)) * 100
			logger.Info(fmt.Sprintf("回测进度: progress=%.1f%%, kline=%d, total=%d", progress, i+1, len(klines)))
		}
	}

	logger.Info("回测完成")
	return nil
}

// RunLive 运行实盘交易
func (e *TradingEngine) RunLive(ctx context.Context) error {
	ctx, logger := log.WithCtx(ctx)
	logger.PushPrefix("TradingEngine")

	logger.Info("开始实盘交易", "symbol", e.tradingPair.String(), "timeframe", e.timeframe.String())

	e.isRunning = true
	defer func() { e.isRunning = false }()

	// 获取时间间隔
	interval := e.getTimeframeInterval()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("收到停止信号，退出实盘交易")
			return ctx.Err()

		case <-e.stopChan:
			logger.Info("手动停止实盘交易")
			return nil

		case <-ticker.C:
			err := e.processLiveTick(ctx)
			if err != nil {
				logger.Error("处理实时数据失败", "error", err)
				// 继续运行，不因单次错误退出
			}
		}
	}
}

// Stop 停止交易引擎
func (e *TradingEngine) Stop() {
	if e.isRunning {
		close(e.stopChan)
	}
}

// processLiveTick 处理实时数据
func (e *TradingEngine) processLiveTick(ctx context.Context) error {
	ctx, logger := log.WithCtx(ctx)

	// 直接从 CEX 客户端获取最新数据
	tradingPair := e.tradingPair
	klines, err := e.cexClient.GetKlines(ctx, tradingPair, e.timeframe.GetBinanceInterval(), 1)

	if err != nil {
		return fmt.Errorf("failed to get latest kline: %w", err)
	}

	if len(klines) == 0 {
		return fmt.Errorf("no kline data available")
	}

	kline := klines[0]

	// 获取当前投资组合状态
	portfolio, err := e.executor.GetPortfolio(ctx)
	if err != nil {
		return fmt.Errorf("failed to get portfolio: %w", err)
	}

	// 更新当前价格
	portfolio.CurrentPrice = kline.Close
	portfolio.Timestamp = kline.OpenTime

	// 执行策略
	signals, err := e.strategy.OnData(ctx, kline, portfolio)
	if err != nil {
		return fmt.Errorf("strategy execution failed: %w", err)
	}

	// 处理交易信号
	for _, signal := range signals {
		err := e.processSignal(ctx, signal, kline, portfolio)
		if err != nil {
			logger.Error("处理交易信号失败", "error", err)
		}
	}

	return nil
}

// processSignal 处理交易信号
func (e *TradingEngine) processSignal(ctx context.Context, signal *strategy.Signal, kline *cex.KlineData, portfolio *executor.Portfolio) error {
	ctx, logger := log.WithCtx(ctx)

	logger.Info("处理交易信号",
		"type", signal.Type,
		"reason", signal.Reason,
		"strength", signal.Strength)

	// 使用信号处理器注册表来处理信号
	return e.signalRegistry.HandleSignal(ctx, signal, kline, portfolio)
}

// getTimeframeInterval 获取时间周期对应的时间间隔
func (e *TradingEngine) getTimeframeInterval() time.Duration {
	intervals := map[string]time.Duration{
		"1s":  1 * time.Second,
		"1m":  1 * time.Minute,
		"3m":  3 * time.Minute,
		"5m":  5 * time.Minute,
		"15m": 15 * time.Minute,
		"30m": 30 * time.Minute,
		"1h":  1 * time.Hour,
		"2h":  2 * time.Hour,
		"4h":  4 * time.Hour,
		"6h":  6 * time.Hour,
		"8h":  8 * time.Hour,
		"12h": 12 * time.Hour,
		"1d":  24 * time.Hour,
		"3d":  3 * 24 * time.Hour,
		"1w":  7 * 24 * time.Hour,
		"1M":  30 * 24 * time.Hour,
	}

	if interval, ok := intervals[e.timeframe.GetBinanceInterval()]; ok {
		return interval
	}

	return 1 * time.Minute // 默认值
}

// Close 关闭交易引擎
func (e *TradingEngine) Close() error {
	e.Stop()
	return e.executor.Close()
}
