package engine

import (
	"context"
	"fmt"
	"sort"
	"time"

	"go-build-stream-gateway-go-server-main/src/binance"
	"go-build-stream-gateway-go-server-main/src/database"
	"go-build-stream-gateway-go-server-main/src/executor"
	"go-build-stream-gateway-go-server-main/src/strategy"
	"go-build-stream-gateway-go-server-main/src/timeframes"

	"github.com/shopspring/decimal"
	"github.com/xpwu/go-log/log"
)

// TradingEngine 统一的交易引擎（支持回测和实盘）
type TradingEngine struct {
	symbol        string
	timeframe     timeframes.Timeframe
	strategy      strategy.Strategy
	executor      executor.Executor
	klineManager  *database.KlineManager
	binanceClient *binance.Client

	// 配置
	positionSizePercent decimal.Decimal
	minTradeAmount      decimal.Decimal

	// 运行状态
	isRunning bool
	stopChan  chan struct{}
}

// NewTradingEngine 创建交易引擎
func NewTradingEngine(
	symbol string,
	timeframe timeframes.Timeframe,
	strategy strategy.Strategy,
	executor executor.Executor,
	klineManager *database.KlineManager,
	binanceClient *binance.Client,
) *TradingEngine {
	return &TradingEngine{
		symbol:              symbol,
		timeframe:           timeframe,
		strategy:            strategy,
		executor:            executor,
		klineManager:        klineManager,
		binanceClient:       binanceClient,
		positionSizePercent: decimal.NewFromFloat(0.95),
		minTradeAmount:      decimal.NewFromFloat(10.0),
		stopChan:            make(chan struct{}),
	}
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

	logger.Info("开始回测",
		"symbol", e.symbol,
		"timeframe", e.timeframe.String(),
		"start", startTime.Format("2006-01-02"),
		"end", endTime.Format("2006-01-02"))

	// 获取历史数据
	startTimeMs := startTime.Unix() * 1000
	endTimeMs := endTime.Unix() * 1000

	var klines []*binance.KlineData
	var err error

	if e.klineManager != nil {
		klines, err = e.klineManager.GetKlinesInRange(ctx, e.symbol, e.timeframe.GetBinanceInterval(), startTimeMs, endTimeMs)
	} else if e.binanceClient != nil {
		// 如果没有KlineManager，直接从网络获取最近100条数据
		logger.Info("KlineManager不可用，使用网络获取数据")
		klines, err = e.binanceClient.GetKlines(ctx, e.symbol, e.timeframe.GetBinanceInterval(), 100)
	} else {
		return fmt.Errorf("no data source available for backtest")
	}

	if err != nil {
		return fmt.Errorf("failed to load historical data: %w", err)
	}

	if len(klines) == 0 {
		return fmt.Errorf("no historical data available")
	}

	// 按时间排序
	sort.Slice(klines, func(i, j int) bool {
		return klines[i].OpenTime < klines[j].OpenTime
	})

	logger.Info("加载历史数据完成", "klines", len(klines))

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
		portfolio.Timestamp = time.Unix(kline.OpenTime/1000, 0)

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
			logger.Info("回测进度", "progress", fmt.Sprintf("%.1f%%", progress), "kline", i+1, "total", len(klines))
		}
	}

	logger.Info("回测完成")
	return nil
}

// RunLive 运行实盘交易
func (e *TradingEngine) RunLive(ctx context.Context) error {
	ctx, logger := log.WithCtx(ctx)
	logger.PushPrefix("TradingEngine")

	logger.Info("开始实盘交易", "symbol", e.symbol, "timeframe", e.timeframe.String())

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

	// 获取最新K线数据
	var klines []*binance.KlineData
	var err error

	if e.klineManager != nil {
		klines, err = e.klineManager.GetKlines(ctx, e.symbol, e.timeframe.GetBinanceInterval(), 1)
	} else {
		return fmt.Errorf("kline manager is required")
	}

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
	portfolio.Timestamp = time.Unix(kline.OpenTime/1000, 0)

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
func (e *TradingEngine) processSignal(ctx context.Context, signal *strategy.Signal, kline *binance.KlineData, portfolio *executor.Portfolio) error {
	ctx, logger := log.WithCtx(ctx)

	logger.Info("处理交易信号",
		"type", signal.Type,
		"reason", signal.Reason,
		"strength", signal.Strength)

	var order *executor.Order

	switch signal.Type {
	case "BUY":
		// 计算买入数量
		availableCash := portfolio.Cash
		tradeAmount := availableCash.Mul(e.positionSizePercent)

		if tradeAmount.LessThan(e.minTradeAmount) {
			logger.Info("交易金额过小，跳过买入", "amount", tradeAmount.String(), "min", e.minTradeAmount.String())
			return nil
		}

		quantity := tradeAmount.Div(kline.Close)

		order = &executor.Order{
			Symbol:    e.symbol,
			Side:      executor.OrderSideBuy,
			Type:      executor.OrderTypeMarket,
			Quantity:  quantity,
			Price:     kline.Close,
			Timestamp: time.Unix(signal.Timestamp/1000, 0),
			Reason:    signal.Reason,
		}

	case "SELL":
		// 卖出全部持仓
		if portfolio.Position.IsZero() {
			logger.Info("无持仓，跳过卖出")
			return nil
		}

		order = &executor.Order{
			Symbol:    e.symbol,
			Side:      executor.OrderSideSell,
			Type:      executor.OrderTypeMarket,
			Quantity:  portfolio.Position,
			Price:     kline.Close,
			Timestamp: time.Unix(signal.Timestamp/1000, 0),
			Reason:    signal.Reason,
		}

	default:
		logger.Error("未知信号类型", "type", signal.Type)
		return nil
	}

	// 执行订单
	result, err := e.executor.ExecuteOrder(ctx, order)
	if err != nil {
		return fmt.Errorf("failed to execute order: %w", err)
	}

	if result.Success {
		logger.Info("订单执行成功",
			"order_id", result.OrderID,
			"side", result.Side,
			"quantity", result.Quantity.String(),
			"price", result.Price.String())
	} else {
		logger.Error("订单执行失败", "error", result.Error)
	}

	return nil
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
