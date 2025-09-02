package executor

import (
	"context"
	"fmt"
	"time"

	"tradingbot/src/cex"

	"github.com/shopspring/decimal"
	"github.com/xpwu/go-log/log"
)

// TradingExecutor 交易执行器：包含所有交易业务逻辑
type TradingExecutor struct {
	tradingPair    cex.TradingPair
	initialCapital decimal.Decimal
	orderStrategy  OrderStrategy

	// 本地状态管理（回测和实盘都需要）
	cash      decimal.Decimal
	position  decimal.Decimal
	portfolio decimal.Decimal

	// 交易记录和统计（回测和实盘都需要）
	orders        []OrderResult
	totalTrades   int
	winningTrades int
	losingTrades  int
}

// NewTradingExecutor 创建交易执行器
func NewTradingExecutor(pair cex.TradingPair, initialCapital decimal.Decimal) *TradingExecutor {
	return &TradingExecutor{
		tradingPair:    pair,
		initialCapital: initialCapital,
		cash:           initialCapital,
		position:       decimal.Zero,
		portfolio:      initialCapital,
		orders:         make([]OrderResult, 0),
	}
}

// SetOrderStrategy 设置订单策略（回测或实盘）
func (e *TradingExecutor) SetOrderStrategy(strategy OrderStrategy) {
	e.orderStrategy = strategy
}

// Buy 执行买入订单（统一业务逻辑）
func (e *TradingExecutor) Buy(ctx context.Context, order *BuyOrder) (*OrderResult, error) {
	ctx, logger := log.WithCtx(ctx)
	logger.PushPrefix("TradingExecutor")

	// 删除详细的执行步骤日志，买入结果将在最后统一记录

	// 1. 业务逻辑检查（回测和实盘都需要）
	executionPrice := order.Price
	notional := order.Quantity.Mul(executionPrice)

	// 资金充足性检查
	if e.cash.LessThan(notional) {
		logger.Error("资金不足", "required", notional.String(), "available", e.cash.String())
		return &OrderResult{
			OrderID:     fmt.Sprintf("failed_%d", time.Now().UnixNano()),
			TradingPair: order.TradingPair,
			Side:        OrderSideBuy,
			Quantity:    order.Quantity,
			Price:       executionPrice,
			Timestamp:   order.Timestamp,
			Success:     false,
			Error:       "insufficient cash",
		}, fmt.Errorf("insufficient cash: required %s, available %s", notional.String(), e.cash.String())
	}

	// 2. 委托给具体的订单策略（差异化处理）
	result, err := e.orderStrategy.ExecuteBuy(ctx, order)
	if err != nil {
		return result, err
	}

	// 3. 更新本地状态（回测和实盘都需要）
	e.cash = e.cash.Sub(notional)
	e.position = e.position.Add(order.Quantity)

	// 4. 记录订单和统计（回测和实盘都需要）
	e.orders = append(e.orders, *result)

	logger.Info(fmt.Sprintf("💰 买入完成: %s @ %s, 余额: %s", 
		order.Quantity.String(), executionPrice.String(), e.cash.String()))

	return result, nil
}

// Sell 执行卖出订单（统一业务逻辑）
func (e *TradingExecutor) Sell(ctx context.Context, order *SellOrder) (*OrderResult, error) {
	ctx, logger := log.WithCtx(ctx)
	logger.PushPrefix("TradingExecutor")

	// 删除详细的执行步骤日志，卖出结果将在最后统一记录

	// 1. 业务逻辑检查（回测和实盘都需要）
	if e.position.LessThan(order.Quantity) {
		logger.Error("持仓不足", "required", order.Quantity.String(), "available", e.position.String())
		return &OrderResult{
			OrderID:     fmt.Sprintf("failed_%d", time.Now().UnixNano()),
			TradingPair: order.TradingPair,
			Side:        OrderSideSell,
			Quantity:    order.Quantity,
			Price:       order.Price,
			Timestamp:   order.Timestamp,
			Success:     false,
			Error:       "insufficient position",
		}, fmt.Errorf("insufficient position: required %s, available %s", order.Quantity.String(), e.position.String())
	}

	// 2. 委托给具体的订单策略（差异化处理）
	result, err := e.orderStrategy.ExecuteSell(ctx, order)
	if err != nil {
		return result, err
	}

	// 3. 更新本地状态（回测和实盘都需要）
	executionPrice := result.Price
	notional := order.Quantity.Mul(executionPrice)

	e.cash = e.cash.Add(notional)
	e.position = e.position.Sub(order.Quantity)

	// 4. 计算盈亏和统计（回测和实盘都需要）
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

				// 完成一个交易对，增加总交易数
				e.totalTrades++

				logger.Info("")  // 空行分隔
				logger.Info(fmt.Sprintf("📈 交易完成: %s → %s, 盈亏: %s", 
					buyPrice.String(), executionPrice.String(), pnl.String()))
				break
			}
		}
	}

	// 5. 更新投资组合价值
	e.portfolio = e.cash.Add(e.position.Mul(executionPrice))

	// 6. 记录订单
	e.orders = append(e.orders, *result)

	logger.Info(fmt.Sprintf("💎 卖出完成: %s @ %s, 余额: %s", 
		order.Quantity.String(), executionPrice.String(), e.cash.String()))

	return result, nil
}

// GetPortfolio 获取当前投资组合状态
func (e *TradingExecutor) GetPortfolio(ctx context.Context) (*Portfolio, error) {
	// 对于实盘交易，可以选择返回本地状态或从CEX获取实时状态
	// 这里先返回本地维护的状态，保持一致性
	return &Portfolio{
		Cash:      e.cash,
		Position:  e.position,
		Portfolio: e.portfolio,
		Timestamp: time.Now(),
	}, nil
}

// GetOrders 获取所有订单记录
func (e *TradingExecutor) GetOrders() []OrderResult {
	return e.orders
}

// GetStatistics 获取交易统计
func (e *TradingExecutor) GetStatistics() map[string]interface{} {
	totalReturn := decimal.Zero
	if !e.initialCapital.IsZero() {
		totalReturn = e.portfolio.Sub(e.initialCapital).Div(e.initialCapital)
	}

	return map[string]interface{}{
		"initial_capital": e.initialCapital,
		"final_portfolio": e.portfolio,
		"total_return":    totalReturn,
		"total_trades":    e.totalTrades,
		"winning_trades":  e.winningTrades,
		"losing_trades":   e.losingTrades,
		"cash":            e.cash,
		"position":        e.position,
	}
}

// GetName 获取执行器名称
func (e *TradingExecutor) GetName() string {
	return "TradingExecutor"
}

// Close 关闭执行器
func (e *TradingExecutor) Close() error {
	return nil
}
