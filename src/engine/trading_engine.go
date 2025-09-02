package engine

import (
	"context"
	"crypto/md5"
	"fmt"
	"time"

	"tradingbot/src/cex"
	"tradingbot/src/executor"
	"tradingbot/src/strategy"
	"tradingbot/src/timeframes"

	"github.com/shopspring/decimal"
	"github.com/xpwu/go-log/log"
)

// generateShortOrderID 生成简短的订单ID
func generateShortOrderID(prefix string, base string) string {
	fullID := fmt.Sprintf("%s_%d_%s", prefix, time.Now().UnixNano(), base)
	hash := md5.Sum([]byte(fullID))
	return fmt.Sprintf("%s_%x", prefix, hash[:4]) // 取前8个字符的hex
}

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

	// 统一数据喂入和挂单管理
	dataFeed     DataFeed
	orderManager OrderManager

	// 运行状态
	isRunning bool
	stopChan  chan struct{}

	// K线数据存储（用于回撤计算等）
	lastKlines []*cex.KlineData
}

// NewTradingEngine 创建交易引擎
func NewTradingEngine(
	pair cex.TradingPair,
	timeframe timeframes.Timeframe,
	strategy strategy.Strategy,
	executor executor.Executor,
	cexClient cex.CEXClient,
	dataFeed DataFeed,
	orderManager OrderManager,
) *TradingEngine {
	engine := &TradingEngine{
		tradingPair:         pair,
		timeframe:           timeframe,
		strategy:            strategy,
		executor:            executor,
		cexClient:           cexClient,
		dataFeed:            dataFeed,
		orderManager:        orderManager,
		positionSizePercent: decimal.NewFromFloat(0.95),
		minTradeAmount:      decimal.NewFromFloat(10.0),
		stopChan:            make(chan struct{}),
	}

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

// RunBacktest 运行回测（使用统一的数据喂入机制）
func (e *TradingEngine) RunBacktest(ctx context.Context, startTime, endTime time.Time) error {
	return e.Run(ctx)
}

// RunLive 运行实盘交易（使用统一的数据喂入机制）
func (e *TradingEngine) RunLive(ctx context.Context) error {
	return e.Run(ctx)
}

// Run 统一的运行方法（支持回测和实盘）
func (e *TradingEngine) Run(ctx context.Context) error {
	ctx, logger := log.WithCtx(ctx)
	logger.PushPrefix("TradingEngine")

	logger.Info("🚀 开始交易引擎",
		"trading_symbol", e.tradingPair.String(),
		"timeframe", e.timeframe.String())

	e.isRunning = true
	defer func() { e.isRunning = false }()

	// 启动数据喂入
	err := e.dataFeed.Start(ctx)
	if err != nil {
		return fmt.Errorf("启动数据喂入失败: %w", err)
	}
	defer e.dataFeed.Stop()

	var klineCount int
	var allKlines []*cex.KlineData

	for {
		select {
		case <-ctx.Done():
			logger.Info("收到停止信号，退出交易")
			return ctx.Err()

		case <-e.stopChan:
			logger.Info("手动停止交易")
			goto finished

		default:
			// 获取下一个K线数据
			kline, err := e.dataFeed.GetNext(ctx)
			if err != nil {
				logger.Error("获取K线数据失败", "error", err)
				continue
			}

			if kline == nil {
				logger.Info("数据流结束")
				goto finished
			}

			// 存储K线数据
			allKlines = append(allKlines, kline)
			klineCount++

			// 1️⃣ 首先检查并执行挂单
			_, err = e.orderManager.CheckAndExecuteOrders(ctx, kline)
			if err != nil {
				logger.Error("检查挂单失败", "error", err)
			}

			// 2️⃣ 获取当前投资组合状态
			portfolio, err := e.executor.GetPortfolio(ctx)
			if err != nil {
				logger.Error("获取投资组合失败", "error", err)
				continue
			}

			// 更新时间
			portfolio.Timestamp = kline.OpenTime

			// 3️⃣ 执行策略分析
			// 删除频繁的策略分析日志

			signals, err := e.strategy.OnData(ctx, kline, portfolio)
			if err != nil {
				logger.Error("❌ 策略执行失败", "error", err)
				continue
			}

			// 信号处理详情在下方的信号循环中记录

			// 4️⃣ 处理交易信号（生成新挂单）
			for _, signal := range signals {
				logger.Info("")  // 空行分隔
				logger.Info(fmt.Sprintf("🎯 %s信号: %s (强度%.1f)", 
					signal.Type, signal.Reason, signal.Strength))

				err := e.processSignal(ctx, signal, kline, portfolio)
				if err != nil {
					logger.Error("❌ 处理交易信号失败", "error", err)
				}
			}

			// 定期输出进度 - 降低频率，只在重要节点显示
			if klineCount%200 == 0 && klineCount > 0 {
				logger.Info("")  // 空行分隔
				logger.Info(fmt.Sprintf("📈 回测进度: %d根K线已处理, 时间: %s", 
					klineCount, e.dataFeed.GetCurrentTime().Format("2006-01-02")))
			}
		}
	}

finished:
	// 保存K线数据供后续使用（如回撤计算）
	e.lastKlines = allKlines
	logger.Info(fmt.Sprintf("交易完成: total_klines=%d", len(allKlines)))
	return nil
}

// Stop 停止交易引擎
func (e *TradingEngine) Stop() {
	if e.isRunning {
		close(e.stopChan)
	}
}

// GetKlines 获取最近处理的K线数据（用于回撤计算等）
func (e *TradingEngine) GetKlines() []*cex.KlineData {
	return e.lastKlines
}

// processSignal 处理交易信号（统一生成挂单）
func (e *TradingEngine) processSignal(ctx context.Context, signal *strategy.Signal, kline *cex.KlineData, portfolio *executor.Portfolio) error {
	ctx, logger := log.WithCtx(ctx)

	logger.Info(fmt.Sprintf("📋 处理交易信号: type=%s, reason=%s, strength=%.1f, price=%s", 
		signal.Type, signal.Reason, signal.Strength, kline.Close.String()))

	switch signal.Type {
	case "BUY":
		return e.handleBuySignal(ctx, signal, kline, portfolio)
	case "SELL":
		return e.handleSellSignal(ctx, signal, kline, portfolio)
	default:
		return fmt.Errorf("未知信号类型: %s", signal.Type)
	}
}

// handleBuySignal 处理买入信号 - 生成限价买单
func (e *TradingEngine) handleBuySignal(ctx context.Context, signal *strategy.Signal, kline *cex.KlineData, portfolio *executor.Portfolio) error {
	ctx, logger := log.WithCtx(ctx)

	// 计算买入数量
	availableCash := portfolio.Cash
	tradeAmount := availableCash.Mul(e.positionSizePercent)

	if tradeAmount.LessThan(e.minTradeAmount) {
		logger.Info(fmt.Sprintf("交易金额过小，跳过买入: amount=%s, min=%s", tradeAmount.String(), e.minTradeAmount.String()))
		return nil
	}

	// 设置买入限价：比当前价格低0.1%（更优价格）
	buySlippage := decimal.NewFromFloat(0.001) // 0.1%
	limitPrice := kline.Close.Mul(decimal.NewFromInt(1).Sub(buySlippage))
	quantity := tradeAmount.Div(limitPrice)

	// 创建挂单
	orderID := generateShortOrderID("buy", e.tradingPair.Base)
	expireTime := kline.OpenTime.Add(24 * time.Hour) // 24小时过期

	pendingOrder := &PendingOrder{
		ID:           orderID,
		Type:         PendingOrderTypeBuyLimit,
		TradingPair:  e.tradingPair,
		Quantity:     quantity,
		Price:        limitPrice,
		CreateTime:   kline.OpenTime,
		ExpireTime:   &expireTime,
		Reason:       signal.Reason,
		OriginSignal: signal.Type,
	}

	logger.Info(fmt.Sprintf("🔵 生成买入限价单: id=%s, limit_price=%s, qty=%s, current_price=%s", 
		orderID, limitPrice.String(), quantity.String(), kline.Close.String()))

	return e.orderManager.PlaceOrder(ctx, pendingOrder)
}

// handleSellSignal 处理卖出信号 - 生成限价卖单
func (e *TradingEngine) handleSellSignal(ctx context.Context, signal *strategy.Signal, kline *cex.KlineData, portfolio *executor.Portfolio) error {
	ctx, logger := log.WithCtx(ctx)

	if portfolio.Position.IsZero() {
		logger.Info("无持仓，跳过卖出信号")
		return nil
	}

	// 计算卖出数量（支持部分卖出）
	var sellQuantity decimal.Decimal
	if signal.Strength <= 0 || signal.Strength > 1 {
		sellQuantity = portfolio.Position
		logger.Info(fmt.Sprintf("信号强度无效，执行全仓卖出: strength=%.1f", signal.Strength))
	} else {
		sellQuantity = portfolio.Position.Mul(decimal.NewFromFloat(signal.Strength))
		if sellQuantity.GreaterThan(portfolio.Position) {
			sellQuantity = portfolio.Position
		}
		logger.Info("执行部分卖出",
			"strength", signal.Strength,
			"sell_quantity", sellQuantity.String(),
			"total_position", portfolio.Position.String())
	}

	// 设置卖出限价：比当前价格高0.1%（更优价格）
	sellSlippage := decimal.NewFromFloat(0.001) // 0.1%
	limitPrice := kline.Close.Mul(decimal.NewFromInt(1).Add(sellSlippage))

	// 取消现有的卖出挂单（避免重复挂单）
	pendingOrders := e.orderManager.GetPendingOrders()
	for _, order := range pendingOrders {
		if order.Type == PendingOrderTypeSellLimit {
			logger.Info(fmt.Sprintf("取消现有卖出挂单: id=%s", order.ID))
			e.orderManager.CancelOrder(ctx, order.ID)
		}
	}

	// 创建新的卖出挂单
	orderID := generateShortOrderID("sell", e.tradingPair.Base)
	expireTime := kline.OpenTime.Add(24 * time.Hour) // 24小时过期

	pendingOrder := &PendingOrder{
		ID:           orderID,
		Type:         PendingOrderTypeSellLimit,
		TradingPair:  e.tradingPair,
		Quantity:     sellQuantity,
		Price:        limitPrice,
		CreateTime:   kline.OpenTime,
		ExpireTime:   &expireTime,
		Reason:       signal.Reason,
		OriginSignal: signal.Type,
	}

	logger.Info(fmt.Sprintf("🔴 生成卖出限价单: id=%s, limit_price=%s, qty=%s, current_price=%s", 
		orderID, limitPrice.String(), sellQuantity.String(), kline.Close.String()))

	return e.orderManager.PlaceOrder(ctx, pendingOrder)
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
	if e.dataFeed != nil {
		if err := e.dataFeed.Stop(); err != nil {
			return err
		}
	}
	return e.executor.Close()
}
