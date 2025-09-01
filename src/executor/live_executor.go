package executor

import (
	"context"
	"fmt"
	"time"

	"tradingbot/src/cex"

	"github.com/shopspring/decimal"
	"github.com/xpwu/go-log/log"
)

// LiveExecutor 实盘交易执行器
type LiveExecutor struct {
	cexClient   cex.CEXClient
	tradingPair cex.TradingPair
}

// NewLiveExecutor 创建实盘交易执行器
func NewLiveExecutor(cexClient cex.CEXClient, pair cex.TradingPair) *LiveExecutor {
	return &LiveExecutor{
		cexClient:   cexClient,
		tradingPair: pair,
	}
}

// validateTradingEnabled 验证交易是否启用
func (e *LiveExecutor) validateTradingEnabled(ctx context.Context) error {
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

// Buy 执行买入订单（真实交易）
func (e *LiveExecutor) Buy(ctx context.Context, order *BuyOrder) (*OrderResult, error) {
	ctx, logger := log.WithCtx(ctx)
	logger.PushPrefix("LiveExecutor")

	logger.Info(fmt.Sprintf("执行实盘买入订单: quantity=%s, price=%s, reason=%s",
		order.Quantity.String(),
		order.Price.String(),
		order.Reason))

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

	logger.Info(fmt.Sprintf("实盘买入订单成功: OrderID=%s, ExecutedQty=%s, ExecutedPrice=%s",
		result.OrderID,
		result.Quantity.String(),
		result.Price.String()))

	return result, nil
}

// Sell 执行卖出订单（真实交易）
func (e *LiveExecutor) Sell(ctx context.Context, order *SellOrder) (*OrderResult, error) {
	ctx, logger := log.WithCtx(ctx)
	logger.PushPrefix("LiveExecutor")

	logger.Info(fmt.Sprintf("执行实盘卖出订单: quantity=%s, price=%s, reason=%s",
		order.Quantity.String(),
		order.Price.String(),
		order.Reason))

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

	logger.Info(fmt.Sprintf("实盘卖出订单成功: OrderID=%s, ExecutedQty=%s, ExecutedPrice=%s",
		result.OrderID,
		result.Quantity.String(),
		result.Price.String()))

	return result, nil
}

// GetPortfolio 获取当前投资组合状态
func (e *LiveExecutor) GetPortfolio(ctx context.Context) (*Portfolio, error) {
	ctx, logger := log.WithCtx(ctx)
	logger.PushPrefix("LiveExecutor")

	// 获取账户余额信息
	balances, err := e.cexClient.GetAccount(ctx)
	if err != nil {
		logger.Error(fmt.Sprintf("获取账户信息失败: %v", err))
		return nil, fmt.Errorf("failed to get account info: %w", err)
	}

	// 查找基础资产和计价资产的余额
	var baseBalance, quoteBalance decimal.Decimal

	for _, balance := range balances {
		if balance.Asset == e.tradingPair.Base {
			baseBalance = balance.Free.Add(balance.Locked)
		}
		if balance.Asset == e.tradingPair.Quote {
			quoteBalance = balance.Free.Add(balance.Locked)
		}
	}

	// 获取当前市场价格（使用最新K线）
	klines, err := e.cexClient.GetKlines(ctx, e.tradingPair, "1m", 1)
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

	logger.Info(fmt.Sprintf("投资组合状态: %s余额=%s, %s余额=%s, 当前价格=%s, 总价值=%s",
		e.tradingPair.Base, baseBalance.String(),
		e.tradingPair.Quote, quoteBalance.String(),
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

// GetName 获取执行器名称
func (e *LiveExecutor) GetName() string {
	return "LiveExecutor"
}

// Close 关闭执行器
func (e *LiveExecutor) Close() error {
	// 实盘执行器无需特殊清理
	return nil
}

// calculateCommission 计算交易手续费
func calculateCommission(price, quantity decimal.Decimal, feeRate float64) decimal.Decimal {
	// 手续费 = 交易金额 × 费率
	tradeAmount := price.Mul(quantity)
	fee := tradeAmount.Mul(decimal.NewFromFloat(feeRate))
	return fee
}
