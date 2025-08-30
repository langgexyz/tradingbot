package executor

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"github.com/xpwu/go-log/log"
)

// BacktestExecutor 回测执行器
type BacktestExecutor struct {
	symbol         string
	initialCapital decimal.Decimal
	commission     decimal.Decimal // 手续费率
	slippage       decimal.Decimal // 滑点

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
func NewBacktestExecutor(symbol string, initialCapital decimal.Decimal) *BacktestExecutor {
	return &BacktestExecutor{
		symbol:         symbol,
		initialCapital: initialCapital,
		commission:     decimal.NewFromFloat(0.001),  // 默认0.1%手续费
		slippage:       decimal.NewFromFloat(0.0001), // 默认0.01%滑点
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

// SetSlippage 设置滑点
func (e *BacktestExecutor) SetSlippage(slippage float64) {
	e.slippage = decimal.NewFromFloat(slippage)
}

// ExecuteOrder 执行订单（模拟）
func (e *BacktestExecutor) ExecuteOrder(ctx context.Context, order *Order) (*OrderResult, error) {
	ctx, logger := log.WithCtx(ctx)
	logger.PushPrefix("BacktestExecutor")

	logger.Info("执行回测订单",
		"side", order.Side,
		"quantity", order.Quantity.String(),
		"price", order.Price.String(),
		"reason", order.Reason)

	// 应用滑点
	executionPrice := order.Price
	if order.Side == OrderSideBuy {
		executionPrice = executionPrice.Mul(decimal.NewFromInt(1).Add(e.slippage))
	} else {
		executionPrice = executionPrice.Mul(decimal.NewFromInt(1).Sub(e.slippage))
	}

	// 计算手续费
	notional := order.Quantity.Mul(executionPrice)
	commission := notional.Mul(e.commission)

	result := &OrderResult{
		OrderID:    fmt.Sprintf("backtest_%d", time.Now().UnixNano()),
		Symbol:     order.Symbol,
		Side:       order.Side,
		Quantity:   order.Quantity,
		Price:      executionPrice,
		Commission: commission,
		Timestamp:  order.Timestamp,
		Success:    true,
	}

	// 更新持仓和现金
	if order.Side == OrderSideBuy {
		// 买入
		totalCost := notional.Add(commission)
		if e.cash.LessThan(totalCost) {
			result.Success = false
			result.Error = "insufficient cash"
			logger.Error("现金不足", "required", totalCost.String(), "available", e.cash.String())
			return result, fmt.Errorf("insufficient cash: required %s, available %s",
				totalCost.String(), e.cash.String())
		}

		e.cash = e.cash.Sub(totalCost)
		e.position = e.position.Add(order.Quantity)

		logger.Info("买入成功",
			"quantity", order.Quantity.String(),
			"price", executionPrice.String(),
			"commission", commission.String(),
			"remaining_cash", e.cash.String(),
			"position", e.position.String())

	} else {
		// 卖出
		if e.position.LessThan(order.Quantity) {
			result.Success = false
			result.Error = "insufficient position"
			logger.Error("持仓不足", "required", order.Quantity.String(), "available", e.position.String())
			return result, fmt.Errorf("insufficient position: required %s, available %s",
				order.Quantity.String(), e.position.String())
		}

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

					logger.Info("卖出成功",
						"quantity", order.Quantity.String(),
						"sell_price", executionPrice.String(),
						"buy_price", buyPrice.String(),
						"pnl", pnl.String(),
						"commission", commission.String(),
						"cash", e.cash.String(),
						"position", e.position.String())
					break
				}
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
