package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"github.com/xpwu/go-log/log"

	"tradingbot/src/cex"
	"tradingbot/src/executor"
)

// PendingOrderType 挂单类型
type PendingOrderType string

const (
	PendingOrderTypeBuyLimit  PendingOrderType = "BUY_LIMIT"
	PendingOrderTypeSellLimit PendingOrderType = "SELL_LIMIT"
)

// PendingOrder 挂单
type PendingOrder struct {
	ID           string           `json:"id"`
	Type         PendingOrderType `json:"type"`
	TradingPair  cex.TradingPair  `json:"trading_pair"`
	Quantity     decimal.Decimal  `json:"quantity"`
	Price        decimal.Decimal  `json:"price"`         // 挂单价格
	CreateTime   time.Time        `json:"create_time"`   // 挂单时间
	ExpireTime   *time.Time       `json:"expire_time"`   // 过期时间（可选）
	Reason       string           `json:"reason"`        // 挂单原因
	OriginSignal string           `json:"origin_signal"` // 原始信号类型
}

// OrderManager 挂单管理器接口
type OrderManager interface {
	// PlaceOrder 下挂单
	PlaceOrder(ctx context.Context, order *PendingOrder) error

	// CancelOrder 取消挂单
	CancelOrder(ctx context.Context, orderID string) error

	// CancelAllOrders 取消所有挂单
	CancelAllOrders(ctx context.Context) error

	// CheckAndExecuteOrders 检查并执行满足条件的挂单
	CheckAndExecuteOrders(ctx context.Context, kline *cex.KlineData) ([]*executor.OrderResult, error)

	// GetPendingOrders 获取所有待执行挂单
	GetPendingOrders() []*PendingOrder

	// GetOrderCount 获取挂单数量
	GetOrderCount() int
}

// BacktestOrderManager 回测挂单管理器
type BacktestOrderManager struct {
	executor      executor.Executor
	pendingOrders map[string]*PendingOrder
	mu            sync.RWMutex
	currentTime   time.Time
}

// NewBacktestOrderManager 创建回测挂单管理器
func NewBacktestOrderManager(executor executor.Executor) *BacktestOrderManager {
	return &BacktestOrderManager{
		executor:      executor,
		pendingOrders: make(map[string]*PendingOrder),
		currentTime:   time.Now(),
	}
}

func (m *BacktestOrderManager) PlaceOrder(ctx context.Context, order *PendingOrder) error {
	ctx, logger := log.WithCtx(ctx)

	m.mu.Lock()
	defer m.mu.Unlock()

	logger.Info(fmt.Sprintf("📋 挂单: %s %s @ %s", 
		order.Type, order.Quantity.String(), order.Price.String()))

	m.pendingOrders[order.ID] = order
	return nil
}

func (m *BacktestOrderManager) CancelOrder(ctx context.Context, orderID string) error {
	ctx, logger := log.WithCtx(ctx)

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.pendingOrders[orderID]; exists {
		delete(m.pendingOrders, orderID)
		logger.Info(fmt.Sprintf("取消挂单: id=%s", orderID))
		return nil
	}

	return fmt.Errorf("挂单不存在: %s", orderID)
}

func (m *BacktestOrderManager) CancelAllOrders(ctx context.Context) error {
	ctx, logger := log.WithCtx(ctx)

	m.mu.Lock()
	defer m.mu.Unlock()

	count := len(m.pendingOrders)
	m.pendingOrders = make(map[string]*PendingOrder)

	logger.Info(fmt.Sprintf("取消所有挂单: count=%d", count))
	return nil
}

func (m *BacktestOrderManager) CheckAndExecuteOrders(ctx context.Context, kline *cex.KlineData) ([]*executor.OrderResult, error) {
	ctx, logger := log.WithCtx(ctx)

	m.mu.Lock()
	defer m.mu.Unlock()

	m.currentTime = kline.OpenTime
	var executedResults []*executor.OrderResult
	var toRemove []string

	for orderID, pendingOrder := range m.pendingOrders {
		// 检查是否过期
		if pendingOrder.ExpireTime != nil && m.currentTime.After(*pendingOrder.ExpireTime) {
			logger.Info(fmt.Sprintf("挂单过期，自动取消: id=%s, expire_time=%s", orderID, pendingOrder.ExpireTime))
			toRemove = append(toRemove, orderID)
			continue
		}

		// 检查是否满足执行条件
		shouldExecute := false
		var executionPrice decimal.Decimal

		switch pendingOrder.Type {
		case PendingOrderTypeBuyLimit:
			// 买入限价单：当前价格 <= 挂单价格时执行
			if kline.Low.LessThanOrEqual(pendingOrder.Price) {
				shouldExecute = true
				// 使用挂单价格或更优价格执行
				if kline.Open.LessThanOrEqual(pendingOrder.Price) {
					executionPrice = kline.Open
				} else {
					executionPrice = pendingOrder.Price
				}
			}

		case PendingOrderTypeSellLimit:
			// 卖出限价单：当前价格 >= 挂单价格时执行
			if kline.High.GreaterThanOrEqual(pendingOrder.Price) {
				shouldExecute = true
				// 使用挂单价格或更优价格执行
				if kline.Open.GreaterThanOrEqual(pendingOrder.Price) {
					executionPrice = kline.Open
				} else {
					executionPrice = pendingOrder.Price
				}
			}
		}

		if shouldExecute {
			// 删除详细的执行条件日志，执行结果在executor中记录

			// 执行订单
			var result *executor.OrderResult
			var err error

			switch pendingOrder.Type {
			case PendingOrderTypeBuyLimit:
				buyOrder := &executor.BuyOrder{
					ID:          pendingOrder.ID,
					TradingPair: pendingOrder.TradingPair,
					Type:        executor.OrderTypeLimit,
					Quantity:    pendingOrder.Quantity,
					Price:       executionPrice,
					Timestamp:   kline.OpenTime,
					Reason:      fmt.Sprintf("执行买入挂单: %s", pendingOrder.Reason),
				}
				result, err = m.executor.Buy(ctx, buyOrder)

			case PendingOrderTypeSellLimit:
				sellOrder := &executor.SellOrder{
					ID:          pendingOrder.ID,
					TradingPair: pendingOrder.TradingPair,
					Type:        executor.OrderTypeLimit,
					Quantity:    pendingOrder.Quantity,
					Price:       executionPrice,
					Timestamp:   kline.OpenTime,
					Reason:      fmt.Sprintf("执行卖出挂单: %s", pendingOrder.Reason),
				}
				result, err = m.executor.Sell(ctx, sellOrder)
			}

			if err != nil {
				logger.Error("挂单执行失败", "id", orderID, "error", err)
				// 执行失败，保留挂单
				continue
			}

			if result != nil && result.Success {
				// 挂单执行详情已在executor中记录，此处无需重复
				executedResults = append(executedResults, result)
				toRemove = append(toRemove, orderID)
			}
		}
	}

	// 移除已执行或过期的挂单
	for _, orderID := range toRemove {
		delete(m.pendingOrders, orderID)
	}

	return executedResults, nil
}

func (m *BacktestOrderManager) GetPendingOrders() []*PendingOrder {
	m.mu.RLock()
	defer m.mu.RUnlock()

	orders := make([]*PendingOrder, 0, len(m.pendingOrders))
	for _, order := range m.pendingOrders {
		orders = append(orders, order)
	}
	return orders
}

func (m *BacktestOrderManager) GetOrderCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.pendingOrders)
}

// LiveOrderManager 实盘挂单管理器
type LiveOrderManager struct {
	cexClient     cex.CEXClient
	pendingOrders map[string]*PendingOrder
	mu            sync.RWMutex
}

// NewLiveOrderManager 创建实盘挂单管理器
func NewLiveOrderManager(cexClient cex.CEXClient) *LiveOrderManager {
	return &LiveOrderManager{
		cexClient:     cexClient,
		pendingOrders: make(map[string]*PendingOrder),
	}
}

func (m *LiveOrderManager) PlaceOrder(ctx context.Context, order *PendingOrder) error {
	ctx, logger := log.WithCtx(ctx)

	// TODO: 实现真实的挂单API调用
	logger.Info("下实盘挂单（暂未实现）",
		"id", order.ID,
		"type", order.Type,
		"price", order.Price.String(),
		"quantity", order.Quantity.String())

	m.mu.Lock()
	defer m.mu.Unlock()
	m.pendingOrders[order.ID] = order

	return fmt.Errorf("live order placement not implemented yet")
}

func (m *LiveOrderManager) CancelOrder(ctx context.Context, orderID string) error {
	ctx, logger := log.WithCtx(ctx)

	// TODO: 实现真实的取消挂单API调用
	logger.Info(fmt.Sprintf("取消实盘挂单（暂未实现）: id=%s", orderID))

	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.pendingOrders, orderID)

	return fmt.Errorf("live order cancellation not implemented yet")
}

func (m *LiveOrderManager) CancelAllOrders(ctx context.Context) error {
	ctx, logger := log.WithCtx(ctx)

	m.mu.Lock()
	defer m.mu.Unlock()

	count := len(m.pendingOrders)
	m.pendingOrders = make(map[string]*PendingOrder)

	logger.Info(fmt.Sprintf("取消所有实盘挂单（暂未实现）: count=%d", count))
	return fmt.Errorf("live order cancellation not implemented yet")
}

func (m *LiveOrderManager) CheckAndExecuteOrders(ctx context.Context, kline *cex.KlineData) ([]*executor.OrderResult, error) {
	// TODO: 实现真实的挂单状态检查
	return []*executor.OrderResult{}, fmt.Errorf("live order execution check not implemented yet")
}

func (m *LiveOrderManager) GetPendingOrders() []*PendingOrder {
	m.mu.RLock()
	defer m.mu.RUnlock()

	orders := make([]*PendingOrder, 0, len(m.pendingOrders))
	for _, order := range m.pendingOrders {
		orders = append(orders, order)
	}
	return orders
}

func (m *LiveOrderManager) GetOrderCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.pendingOrders)
}
