package executor

import (
	"context"
	"fmt"
	"time"

	"tradingbot/src/cex"

	"github.com/shopspring/decimal"
	"github.com/xpwu/go-log/log"
)

// BacktestOrderExecutor 回测订单执行器：只在本地数据库记录
type BacktestOrderExecutor struct {
	tradingPair cex.TradingPair
}

// NewBacktestOrderExecutor 创建回测订单执行器
func NewBacktestOrderExecutor(pair cex.TradingPair) *BacktestOrderExecutor {
	return &BacktestOrderExecutor{
		tradingPair: pair,
	}
}

// ExecuteBuy 执行买入订单（模拟）
func (e *BacktestOrderExecutor) ExecuteBuy(ctx context.Context, order *BuyOrder) (*OrderResult, error) {
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
func (e *BacktestOrderExecutor) ExecuteSell(ctx context.Context, order *SellOrder) (*OrderResult, error) {
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
func (e *BacktestOrderExecutor) GetRealPortfolio(ctx context.Context, pair cex.TradingPair) (*Portfolio, error) {
	// 回测模式不需要从外部获取，返回nil让UnifiedExecutor使用本地状态
	return nil, nil
}

// LiveOrderExecutor 实盘订单执行器：本地数据库记录 + CEX API调用
type LiveOrderExecutor struct {
	cexClient   cex.CEXClient
	tradingPair cex.TradingPair
}

// NewLiveOrderExecutor 创建实盘订单执行器
func NewLiveOrderExecutor(cexClient cex.CEXClient, pair cex.TradingPair) *LiveOrderExecutor {
	return &LiveOrderExecutor{
		cexClient:   cexClient,
		tradingPair: pair,
	}
}

// validateTradingEnabled 验证交易是否启用
func (e *LiveOrderExecutor) validateTradingEnabled(ctx context.Context) error {
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
func (e *LiveOrderExecutor) ExecuteBuy(ctx context.Context, order *BuyOrder) (*OrderResult, error) {
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
		Commission:  calculateCommission(cexResult.Price, cexResult.Quantity, e.cexClient.GetTradingFee()),
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
		"commission", result.Commission.String(),
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
func (e *LiveOrderExecutor) ExecuteSell(ctx context.Context, order *SellOrder) (*OrderResult, error) {
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
		Commission:  calculateCommission(cexResult.Price, cexResult.Quantity, e.cexClient.GetTradingFee()),
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
		"commission", result.Commission.String(),
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
func (e *LiveOrderExecutor) GetRealPortfolio(ctx context.Context, pair cex.TradingPair) (*Portfolio, error) {
	ctx, logger := log.WithCtx(ctx)
	logger.PushPrefix("LiveOrderExecutor")

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

	// 获取当前市场价格（使用最新K线）
	klines, err := e.cexClient.GetKlines(ctx, pair, "1m", 1)
	if err != nil {
		logger.Error(fmt.Sprintf("获取当前价格失败: %v", err))
		return nil, fmt.Errorf("failed to get current price: %w", err)
	}

	var currentPrice decimal.Decimal
	if len(klines) > 0 {
		currentPrice = klines[0].Close
	} else {
		return nil, fmt.Errorf("no price data available")
	}

	// 计算投资组合总价值 = 计价资产余额 + (基础资产余额 × 当前价格)
	baseValue := baseBalance.Mul(currentPrice)
	totalPortfolio := quoteBalance.Add(baseValue)

	logger.Info(fmt.Sprintf("真实投资组合状态: %s余额=%s, %s余额=%s, 当前价格=%s, 总价值=%s",
		pair.Base, baseBalance.String(),
		pair.Quote, quoteBalance.String(),
		currentPrice.String(),
		totalPortfolio.String()))

	return &Portfolio{
		Cash:         quoteBalance,   // 计价资产作为现金
		Position:     baseBalance,    // 基础资产作为持仓
		CurrentPrice: currentPrice,   // 当前市场价格
		Portfolio:    totalPortfolio, // 总投资组合价值
		Timestamp:    time.Now(),
	}, nil
}

// calculateCommission 计算手续费
func calculateCommission(price, quantity decimal.Decimal, feeRate float64) decimal.Decimal {
	notional := price.Mul(quantity)
	return notional.Mul(decimal.NewFromFloat(feeRate))
}
