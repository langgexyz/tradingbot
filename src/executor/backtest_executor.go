package executor

import (
	"context"
	"fmt"
	"time"

	"tradingbot/src/cex"

	"github.com/shopspring/decimal"
	"github.com/xpwu/go-log/log"
)

// BacktestExecutor 回测执行器
type BacktestExecutor struct {
	tradingPair    cex.TradingPair
	initialCapital decimal.Decimal
	commission     decimal.Decimal // 手续费率

	// 当前状态
	cash      decimal.Decimal
	position  decimal.Decimal
	portfolio decimal.Decimal

	// 交易记录
	orders []OrderResult

	// 统计
	totalTrades     int
	winningTrades   int
	losingTrades    int
	totalCommission decimal.Decimal
}

// NewBacktestExecutor 创建回测执行器
func NewBacktestExecutor(pair cex.TradingPair, initialCapital decimal.Decimal) *BacktestExecutor {
	return &BacktestExecutor{
		tradingPair:    pair,
		initialCapital: initialCapital,
		commission:     decimal.NewFromFloat(0.001), // 默认0.1%手续费
		cash:           initialCapital,
		position:       decimal.Zero,
		portfolio:      initialCapital,
		orders:         make([]OrderResult, 0),
	}
}

// SetCommission 设置手续费率
func (e *BacktestExecutor) SetCommission(commission float64) {
	e.commission = decimal.NewFromFloat(commission)
}

// Buy 执行买入订单（模拟）
func (e *BacktestExecutor) Buy(ctx context.Context, order *BuyOrder) (*OrderResult, error) {
	ctx, logger := log.WithCtx(ctx)
	logger.PushPrefix("BacktestExecutor")

	logger.Info(fmt.Sprintf("执行回测买入订单: quantity=%s, price=%s, reason=%s",
		order.Quantity.String(),
		order.Price.String(),
		order.Reason))

	// CEX交易使用精确价格，无滑点
	executionPrice := order.Price

	// 计算手续费
	notional := order.Quantity.Mul(executionPrice)
	commission := notional.Mul(e.commission)
	totalCost := notional.Add(commission)

	result := &OrderResult{
		OrderID:     fmt.Sprintf("backtest_%d", time.Now().UnixNano()),
		TradingPair: order.TradingPair,
		Side:        OrderSideBuy,
		Quantity:    order.Quantity,
		Price:       executionPrice,
		Commission:  commission,
		Timestamp:   order.Timestamp,
		Success:     true,
	}

	// 检查现金是否充足
	if e.cash.LessThan(totalCost) {
		result.Success = false
		result.Error = "insufficient cash"
		logger.Error("现金不足", "required", totalCost.String(), "available", e.cash.String())
		return result, fmt.Errorf("insufficient cash: required %s, available %s",
			totalCost.String(), e.cash.String())
	}

	// 更新持仓和现金
	e.cash = e.cash.Sub(totalCost)
	e.position = e.position.Add(order.Quantity)

	logger.Info(fmt.Sprintf("买入成功: quantity=%s, price=%s, commission=%s, remaining_cash=%s, position=%s",
		order.Quantity.String(),
		executionPrice.String(),
		commission.String(),
		e.cash.String(),
		e.position.String()))

	// 记录订单
	e.orders = append(e.orders, *result)
	e.totalTrades++

	return result, nil
}

// Sell 执行卖出订单（模拟）
func (e *BacktestExecutor) Sell(ctx context.Context, order *SellOrder) (*OrderResult, error) {
	ctx, logger := log.WithCtx(ctx)
	logger.PushPrefix("BacktestExecutor")

	logger.Info(fmt.Sprintf("执行回测卖出订单: quantity=%s, price=%s, reason=%s",
		order.Quantity.String(),
		order.Price.String(),
		order.Reason))

	// 检查持仓是否充足
	if e.position.LessThan(order.Quantity) {
		result := &OrderResult{
			OrderID:     fmt.Sprintf("backtest_%d", time.Now().UnixNano()),
			TradingPair: order.TradingPair,
			Side:        OrderSideSell,
			Quantity:    order.Quantity,
			Price:       order.Price,
			Timestamp:   order.Timestamp,
			Success:     false,
			Error:       "insufficient position",
		}
		logger.Error("持仓不足", "required", order.Quantity.String(), "available", e.position.String())
		return result, fmt.Errorf("insufficient position: required %s, available %s",
			order.Quantity.String(), e.position.String())
	}

	// CEX交易使用精确价格，无滑点
	executionPrice := order.Price

	// 计算手续费
	notional := order.Quantity.Mul(executionPrice)
	commission := notional.Mul(e.commission)

	result := &OrderResult{
		OrderID:     fmt.Sprintf("backtest_%d", time.Now().UnixNano()),
		TradingPair: order.TradingPair,
		Side:        OrderSideSell,
		Quantity:    order.Quantity,
		Price:       executionPrice,
		Commission:  commission,
		Timestamp:   order.Timestamp,
		Success:     true,
	}

	// 更新持仓和现金
	e.position = e.position.Sub(order.Quantity)
	e.cash = e.cash.Add(notional.Sub(commission))

	// 计算盈亏（简化计算）
	if len(e.orders) > 0 {
		// 找到最近的买入订单计算盈亏
		for i := len(e.orders) - 1; i >= 0; i-- {
			if e.orders[i].Side == OrderSideBuy {
				buyPrice := e.orders[i].Price
				pnl := order.Quantity.Mul(executionPrice.Sub(buyPrice))

				// 更新盈亏统计
				if pnl.GreaterThan(decimal.Zero) {
					e.winningTrades++
				} else {
					e.losingTrades++
				}

				// 计算盈利率
				profitPercent := executionPrice.Sub(buyPrice).Div(buyPrice).Mul(decimal.NewFromInt(100))

				// 计算买入和卖出金额
				buyAmount := order.Quantity.Mul(buyPrice)
				sellAmount := order.Quantity.Mul(executionPrice)

				// 计算持仓时间
				duration := order.Timestamp.Sub(e.orders[i].Timestamp)
				days := int(duration.Hours() / 24)
				hours := int(duration.Hours()) % 24
				totalDays := duration.Hours() / 24

				logger.Info(fmt.Sprintf("卖出成功: quantity=%s, sell_price=%s, buy_price=%s, pnl=%s, commission=%s, cash=%s, position=%s",
					order.Quantity.String(),
					executionPrice.String(),
					buyPrice.String(),
					pnl.String(),
					commission.String(),
					e.cash.String(),
					e.position.String()))

				// 输出详细交易记录
				fmt.Printf("\n🔸 交易完成: 买入价 %s → 卖出价 %s\n",
					buyPrice.StringFixed(8), executionPrice.StringFixed(8))
				fmt.Printf("📅 买入时间: %s\n", e.orders[i].Timestamp.Format("2006-01-02 15:04"))
				fmt.Printf("💰 买入价格: %s USDT\n", buyPrice.StringFixed(8))
				fmt.Printf("💵 买入金额: $%s (%s PEPE × %s)\n",
					buyAmount.StringFixed(2), order.Quantity.StringFixed(0), buyPrice.StringFixed(8))
				fmt.Printf("📅 卖出时间: %s\n", order.Timestamp.Format("2006-01-02 15:04"))
				fmt.Printf("💰 卖出价格: %s USDT\n", executionPrice.StringFixed(8))
				fmt.Printf("💵 卖出金额: $%s (%s PEPE × %s)\n",
					sellAmount.StringFixed(2), order.Quantity.StringFixed(0), executionPrice.StringFixed(8))
				fmt.Printf("📈 盈利率: %s%%\n", profitPercent.StringFixed(2))
				fmt.Printf("💎 净盈利: $%s\n", pnl.StringFixed(2))
				fmt.Printf("⏱️  持仓时间: %d天%d小时 (%.2f天)\n", days, hours, totalDays)
				fmt.Printf("----------------------------------------\n")

				break
			}
		}
	}

	// 更新投资组合价值
	e.portfolio = e.cash.Add(e.position.Mul(executionPrice))

	// 记录订单
	e.orders = append(e.orders, *result)
	e.totalTrades++
	e.totalCommission = e.totalCommission.Add(commission)

	return result, nil
}

// GetPortfolio 获取当前投资组合状态
func (e *BacktestExecutor) GetPortfolio(ctx context.Context) (*Portfolio, error) {
	return &Portfolio{
		Cash:      e.cash,
		Position:  e.position,
		Portfolio: e.portfolio,
		Timestamp: time.Now(),
	}, nil
}

// GetName 获取执行器名称
func (e *BacktestExecutor) GetName() string {
	return "BacktestExecutor"
}

// Close 关闭执行器
func (e *BacktestExecutor) Close() error {
	// 回测执行器无需清理资源
	return nil
}

// GetOrders 获取所有订单记录
func (e *BacktestExecutor) GetOrders() []OrderResult {
	return e.orders
}

// GetStatistics 获取交易统计
func (e *BacktestExecutor) GetStatistics() map[string]interface{} {
	totalReturn := decimal.Zero
	if !e.initialCapital.IsZero() {
		totalReturn = e.portfolio.Sub(e.initialCapital).Div(e.initialCapital)
	}

	return map[string]interface{}{
		"initial_capital":  e.initialCapital,
		"final_portfolio":  e.portfolio,
		"total_return":     totalReturn,
		"total_trades":     e.totalTrades,
		"winning_trades":   e.winningTrades,
		"losing_trades":    e.losingTrades,
		"total_commission": e.totalCommission,
		"cash":             e.cash,
		"position":         e.position,
	}
}
