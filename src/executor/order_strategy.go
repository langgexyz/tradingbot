package executor

import (
	"context"
	"fmt"
	"time"

	"tradingbot/src/cex"

	"github.com/shopspring/decimal"
	"github.com/xpwu/go-log/log"
)

// BacktestOrderStrategy 回测订单策略：只在本地数据库记录
type BacktestOrderStrategy struct {
	tradingPair cex.TradingPair
}

// NewBacktestOrderStrategy 创建回测订单策略
func NewBacktestOrderStrategy(pair cex.TradingPair) *BacktestOrderStrategy {
	return &BacktestOrderStrategy{
		tradingPair: pair,
	}
}

// ExecuteBuy 执行买入订单（模拟）
func (e *BacktestOrderStrategy) ExecuteBuy(ctx context.Context, order *BuyOrder) (*OrderResult, error) {
	// 回测模式：只需要生成订单记录，无真实API调用
	result := &OrderResult{
		OrderID:     fmt.Sprintf("backtest_%d", time.Now().UnixNano()),
		TradingPair: order.TradingPair,
		Side:        OrderSideBuy,
		Quantity:    order.Quantity,
		Price:       order.Price, // 回测使用精确价格，无滑点
		Timestamp:   order.Timestamp,
		Success:     true,
	}

	// TODO: 保存到本地数据库

	// 打印结构化日志用于数据分析
	ctx, logger := log.WithCtx(ctx)
	logger.Info("TRADE_RECORD",
		"mode", "BACKTEST",
		"action", "BUY",
		"order_id", result.OrderID,
		"symbol", result.TradingPair.String(),
		"quantity", result.Quantity.String(),
		"price", result.Price.String(),
		"notional", result.Quantity.Mul(result.Price).String(),
		"timestamp", result.Timestamp.Format("2006-01-02T15:04:05Z"),
		"reason", order.Reason)

	return result, nil
}

// ExecuteSell 执行卖出订单（模拟）
func (e *BacktestOrderStrategy) ExecuteSell(ctx context.Context, order *SellOrder) (*OrderResult, error) {
	// 回测模式：只需要生成订单记录，无真实API调用
	result := &OrderResult{
		OrderID:     fmt.Sprintf("backtest_%d", time.Now().UnixNano()),
		TradingPair: order.TradingPair,
		Side:        OrderSideSell,
		Quantity:    order.Quantity,
		Price:       order.Price, // 回测使用精确价格，无滑点
		Timestamp:   order.Timestamp,
		Success:     true,
	}

	// TODO: 保存到本地数据库

	// 打印结构化日志用于数据分析
	ctx, logger := log.WithCtx(ctx)
	logger.Info("TRADE_RECORD",
		"mode", "BACKTEST",
		"action", "SELL",
		"order_id", result.OrderID,
		"symbol", result.TradingPair.String(),
		"quantity", result.Quantity.String(),
		"price", result.Price.String(),
		"notional", result.Quantity.Mul(result.Price).String(),
		"timestamp", result.Timestamp.Format("2006-01-02T15:04:05Z"),
		"reason", order.Reason)

	return result, nil
}

// GetRealPortfolio 获取真实投资组合（回测模式返回nil）
func (e *BacktestOrderStrategy) GetRealPortfolio(ctx context.Context, pair cex.TradingPair) (*Portfolio, error) {
	// 回测模式不需要从外部获取，返回nil让TradingExecutor使用本地状态
	return nil, nil
}

// LiveOrderStrategy 实盘订单策略：本地数据库记录 + CEX API调用
type LiveOrderStrategy struct {
	cexClient   cex.CEXClient
	tradingPair cex.TradingPair
}

// NewLiveOrderStrategy 创建实盘订单策略
func NewLiveOrderStrategy(cexClient cex.CEXClient, pair cex.TradingPair) *LiveOrderStrategy {
	return &LiveOrderStrategy{
		cexClient:   cexClient,
		tradingPair: pair,
	}
}

// validateTradingEnabled 验证交易是否启用
func (e *LiveOrderStrategy) validateTradingEnabled(ctx context.Context) error {
	ctx, logger := log.WithCtx(ctx)

	// 测试连接
	if err := e.cexClient.Ping(ctx); err != nil {
		return fmt.Errorf("CEX连接失败: %w", err)
	}

	// 这里可以添加更多安全检查
	// 例如：检查配置文件中的 EnableTrading 标志
	// 例如：检查API权限
	// 例如：检查余额是否足够

	logger.Info("✅ 实盘交易安全检查通过")
	return nil
}

// ExecuteBuy 执行买入订单（真实交易）
func (e *LiveOrderStrategy) ExecuteBuy(ctx context.Context, order *BuyOrder) (*OrderResult, error) {
	ctx, logger := log.WithCtx(ctx)
	logger.PushPrefix("LiveOrderExecutor")

	// 执行安全检查
	if err := e.validateTradingEnabled(ctx); err != nil {
		return nil, fmt.Errorf("实盘交易安全检查失败: %w", err)
	}

	// 创建币安买入订单请求
	buyRequest := cex.BuyOrderRequest{
		TradingPair: e.tradingPair,
		Type:        cex.OrderType(order.Type),
		Quantity:    order.Quantity,
		Price:       order.Price,
	}

	// 执行真实的币安API调用
	cexResult, err := e.cexClient.Buy(ctx, buyRequest)
	if err != nil {
		logger.Error(fmt.Sprintf("币安买入订单失败: %v", err))
		return &OrderResult{
			OrderID:     fmt.Sprintf("live_failed_%d", time.Now().UnixNano()),
			TradingPair: order.TradingPair,
			Side:        OrderSideBuy,
			Quantity:    order.Quantity,
			Price:       order.Price,
			Timestamp:   order.Timestamp,
			Success:     false,
			Error:       err.Error(),
		}, err
	}

	// 转换为内部订单结果格式
	result := &OrderResult{
		OrderID:     cexResult.OrderID,
		TradingPair: order.TradingPair,
		Side:        OrderSideBuy,
		Quantity:    cexResult.Quantity,
		Price:       cexResult.Price,
		Timestamp:   cexResult.TransactTime,
		Success:     true,
	}

	// TODO: 保存到本地数据库

	// 打印结构化日志用于数据分析
	logger.Info("TRADE_RECORD",
		"mode", "LIVE",
		"action", "BUY",
		"order_id", result.OrderID,
		"symbol", result.TradingPair.String(),
		"quantity", result.Quantity.String(),
		"price", result.Price.String(),
		"notional", result.Quantity.Mul(result.Price).String(),
		"timestamp", result.Timestamp.Format("2006-01-02T15:04:05Z"),
		"reason", order.Reason,
		"cex_order_id", cexResult.OrderID)

	logger.Info(fmt.Sprintf("实盘买入订单成功: OrderID=%s, ExecutedQty=%s, ExecutedPrice=%s",
		result.OrderID,
		result.Quantity.String(),
		result.Price.String()))

	return result, nil
}

// ExecuteSell 执行卖出订单（真实交易）
func (e *LiveOrderStrategy) ExecuteSell(ctx context.Context, order *SellOrder) (*OrderResult, error) {
	ctx, logger := log.WithCtx(ctx)
	logger.PushPrefix("LiveOrderExecutor")

	// 执行安全检查
	if err := e.validateTradingEnabled(ctx); err != nil {
		return nil, fmt.Errorf("实盘交易安全检查失败: %w", err)
	}

	// 创建币安卖出订单请求
	sellRequest := cex.SellOrderRequest{
		TradingPair: e.tradingPair,
		Type:        cex.OrderType(order.Type),
		Quantity:    order.Quantity,
		Price:       order.Price,
	}

	// 执行真实的币安API调用
	cexResult, err := e.cexClient.Sell(ctx, sellRequest)
	if err != nil {
		logger.Error(fmt.Sprintf("币安卖出订单失败: %v", err))
		return &OrderResult{
			OrderID:     fmt.Sprintf("live_failed_%d", time.Now().UnixNano()),
			TradingPair: order.TradingPair,
			Side:        OrderSideSell,
			Quantity:    order.Quantity,
			Price:       order.Price,
			Timestamp:   order.Timestamp,
			Success:     false,
			Error:       err.Error(),
		}, err
	}

	// 转换为内部订单结果格式
	result := &OrderResult{
		OrderID:     cexResult.OrderID,
		TradingPair: order.TradingPair,
		Side:        OrderSideSell,
		Quantity:    cexResult.Quantity,
		Price:       cexResult.Price,
		Timestamp:   cexResult.TransactTime,
		Success:     true,
	}

	// TODO: 保存到本地数据库

	// 打印结构化日志用于数据分析
	logger.Info("TRADE_RECORD",
		"mode", "LIVE",
		"action", "SELL",
		"order_id", result.OrderID,
		"symbol", result.TradingPair.String(),
		"quantity", result.Quantity.String(),
		"price", result.Price.String(),
		"notional", result.Quantity.Mul(result.Price).String(),
		"timestamp", result.Timestamp.Format("2006-01-02T15:04:05Z"),
		"reason", order.Reason,
		"cex_order_id", cexResult.OrderID)

	logger.Info(fmt.Sprintf("实盘卖出订单成功: OrderID=%s, ExecutedQty=%s, ExecutedPrice=%s",
		result.OrderID,
		result.Quantity.String(),
		result.Price.String()))

	return result, nil
}

// GetRealPortfolio 获取真实投资组合状态（从CEX）
func (e *LiveOrderStrategy) GetRealPortfolio(ctx context.Context, pair cex.TradingPair) (*Portfolio, error) {
	ctx, logger := log.WithCtx(ctx)
	logger.PushPrefix("LiveOrderStrategy")

	// 获取账户余额信息
	balances, err := e.cexClient.GetAccount(ctx)
	if err != nil {
		logger.Error(fmt.Sprintf("获取账户信息失败: %v", err))
		return nil, fmt.Errorf("failed to get account info: %w", err)
	}

	// 查找基础资产和计价资产的余额
	var baseBalance, quoteBalance decimal.Decimal

	for _, balance := range balances {
		if balance.Asset == pair.Base {
			baseBalance = balance.Free.Add(balance.Locked)
		}
		if balance.Asset == pair.Quote {
			quoteBalance = balance.Free.Add(balance.Locked)
		}
	}

	logger.Info(fmt.Sprintf("真实账户余额: %s=%s, %s=%s",
		pair.Base, baseBalance.String(),
		pair.Quote, quoteBalance.String()))

	return &Portfolio{
		Cash:      quoteBalance, // 计价资产作为现金
		Position:  baseBalance,  // 基础资产作为持仓
		Portfolio: decimal.Zero, // 不计算总价值，保持简单
		Timestamp: time.Now(),
	}, nil
}
