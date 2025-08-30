package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"github.com/xpwu/go-log/log"

	"tradingbot/src/cex"
	"tradingbot/src/executor"
	"tradingbot/src/strategy"
)

// SignalHandler 信号处理器接口
type SignalHandler interface {
	// HandleSignal 处理交易信号
	HandleSignal(ctx context.Context, signal *strategy.Signal, kline *cex.KlineData, portfolio *executor.Portfolio) error
}

// SignalHandlerRegistry 信号处理器注册表
type SignalHandlerRegistry struct {
	handlers map[string]SignalHandler
}

// NewSignalHandlerRegistry 创建信号处理器注册表
func NewSignalHandlerRegistry() *SignalHandlerRegistry {
	return &SignalHandlerRegistry{
		handlers: make(map[string]SignalHandler),
	}
}

// RegisterHandler 注册信号处理器
func (r *SignalHandlerRegistry) RegisterHandler(signalType string, handler SignalHandler) {
	r.handlers[signalType] = handler
}

// HandleSignal 处理信号
func (r *SignalHandlerRegistry) HandleSignal(ctx context.Context, signal *strategy.Signal, kline *cex.KlineData, portfolio *executor.Portfolio) error {
	handler, exists := r.handlers[signal.Type]
	if !exists {
		return fmt.Errorf("未知信号类型: %s", signal.Type)
	}

	return handler.HandleSignal(ctx, signal, kline, portfolio)
}

// BuySignalHandler 买入信号处理器
type BuySignalHandler struct {
	executor            executor.Executor
	tradingPair         cex.TradingPair
	positionSizePercent decimal.Decimal
	minTradeAmount      decimal.Decimal
}

// NewBuySignalHandler 创建买入信号处理器
func NewBuySignalHandler(executor executor.Executor, pair cex.TradingPair, positionSizePercent, minTradeAmount decimal.Decimal) *BuySignalHandler {
	return &BuySignalHandler{
		executor:            executor,
		tradingPair:         pair,
		positionSizePercent: positionSizePercent,
		minTradeAmount:      minTradeAmount,
	}
}

// HandleSignal 处理买入信号
func (h *BuySignalHandler) HandleSignal(ctx context.Context, signal *strategy.Signal, kline *cex.KlineData, portfolio *executor.Portfolio) error {
	ctx, logger := log.WithCtx(ctx)

	logger.Info("处理买入信号",
		"reason", signal.Reason,
		"strength", signal.Strength)

	// 计算买入数量
	availableCash := portfolio.Cash
	tradeAmount := availableCash.Mul(h.positionSizePercent)

	if tradeAmount.LessThan(h.minTradeAmount) {
		logger.Info("交易金额过小，跳过买入", "amount", tradeAmount.String(), "min", h.minTradeAmount.String())
		return nil
	}

	quantity := tradeAmount.Div(kline.Close)

	buyOrder := &executor.BuyOrder{
		TradingPair: h.tradingPair,
		Type:        executor.OrderTypeMarket,
		Quantity:    quantity,
		Price:       kline.Close,
		Timestamp:   time.Unix(signal.Timestamp/1000, 0),
		Reason:      signal.Reason,
	}

	// 执行买入订单
	result, err := h.executor.Buy(ctx, buyOrder)
	if err != nil {
		return fmt.Errorf("failed to execute buy order: %w", err)
	}

	if result.Success {
		logger.Info("买入订单执行成功",
			"order_id", result.OrderID,
			"quantity", result.Quantity.String(),
			"price", result.Price.String())
	} else {
		logger.Error("买入订单执行失败", "error", result.Error)
	}

	return nil
}

// SellSignalHandler 卖出信号处理器
type SellSignalHandler struct {
	executor    executor.Executor
	tradingPair cex.TradingPair
}

// NewSellSignalHandler 创建卖出信号处理器
func NewSellSignalHandler(executor executor.Executor, pair cex.TradingPair) *SellSignalHandler {
	return &SellSignalHandler{
		executor:    executor,
		tradingPair: pair,
	}
}

// HandleSignal 处理卖出信号
func (h *SellSignalHandler) HandleSignal(ctx context.Context, signal *strategy.Signal, kline *cex.KlineData, portfolio *executor.Portfolio) error {
	ctx, logger := log.WithCtx(ctx)

	logger.Info("处理卖出信号",
		"reason", signal.Reason,
		"strength", signal.Strength)

	// 检查持仓
	if portfolio.Position.IsZero() {
		logger.Info("无持仓，跳过卖出")
		return nil
	}

	// 🔥 新功能：根据信号强度计算卖出数量
	var sellQuantity decimal.Decimal

	// 如果信号强度为0或超过1，默认全仓卖出
	if signal.Strength <= 0 || signal.Strength > 1 {
		sellQuantity = portfolio.Position
		logger.Info("信号强度无效，执行全仓卖出", "strength", signal.Strength)
	} else {
		// 按信号强度计算部分卖出数量
		sellQuantity = portfolio.Position.Mul(decimal.NewFromFloat(signal.Strength))

		// 确保不超过持仓数量
		if sellQuantity.GreaterThan(portfolio.Position) {
			sellQuantity = portfolio.Position
		}

		// 记录分批交易信息
		sellPercent := decimal.NewFromFloat(signal.Strength).Mul(decimal.NewFromInt(100))
		logger.Info("执行分批卖出",
			"sell_quantity", sellQuantity.String(),
			"total_position", portfolio.Position.String(),
			"sell_percent", sellPercent.String()+"%")
	}

	sellOrder := &executor.SellOrder{
		TradingPair: h.tradingPair,
		Type:        executor.OrderTypeMarket,
		Quantity:    sellQuantity, // 🎯 使用计算后的数量，而不是全部持仓
		Price:       kline.Close,
		Timestamp:   time.Unix(signal.Timestamp/1000, 0),
		Reason:      signal.Reason,
	}

	// 执行卖出订单
	result, err := h.executor.Sell(ctx, sellOrder)
	if err != nil {
		return fmt.Errorf("failed to execute sell order: %w", err)
	}

	if result.Success {
		logger.Info("卖出订单执行成功",
			"order_id", result.OrderID,
			"quantity", result.Quantity.String(),
			"price", result.Price.String())
	} else {
		logger.Error("卖出订单执行失败", "error", result.Error)
	}

	return nil
}
