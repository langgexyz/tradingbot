package binance

import (
	"context"
	"fmt"
	"time"

	"github.com/adshao/go-binance/v2"
	"github.com/shopspring/decimal"
)

// Client 币安客户端封装
type Client struct {
	client    *binance.Client
	apiKey    string
	secretKey string
}

// KlineData K线数据 - 符合币安官方API返回格式
type KlineData struct {
	Symbol              string          `json:"symbol"`
	OpenTime            int64           `json:"open_time"`              // 开盘时间
	Open                decimal.Decimal `json:"open"`                   // 开盘价
	High                decimal.Decimal `json:"high"`                   // 最高价
	Low                 decimal.Decimal `json:"low"`                    // 最低价
	Close               decimal.Decimal `json:"close"`                  // 收盘价
	Volume              decimal.Decimal `json:"volume"`                 // 成交量
	CloseTime           int64           `json:"close_time"`             // 收盘时间
	QuoteVolume         decimal.Decimal `json:"quote_volume"`           // 成交额
	TakerBuyVolume      decimal.Decimal `json:"taker_buy_volume"`       // 主动买入成交量
	TakerBuyQuoteVolume decimal.Decimal `json:"taker_buy_quote_volume"` // 主动买入成交额
}

// OrderResult 订单结果
type OrderResult struct {
	Symbol        string          `json:"symbol"`
	OrderID       int64           `json:"order_id"`
	ClientOrderID string          `json:"client_order_id"`
	Price         decimal.Decimal `json:"price"`
	Quantity      decimal.Decimal `json:"quantity"`
	Side          string          `json:"side"`
	Status        string          `json:"status"`
	Type          string          `json:"type"`
	TimeInForce   string          `json:"time_in_force"`
	TransactTime  int64           `json:"transact_time"`
}

// AccountBalance 账户余额
type AccountBalance struct {
	Asset  string          `json:"asset"`
	Free   decimal.Decimal `json:"free"`
	Locked decimal.Decimal `json:"locked"`
}

// NewClient 创建新的币安客户端
func NewClient(apiKey, secretKey, baseURL string) *Client {
	client := binance.NewClient(apiKey, secretKey)
	client.BaseURL = baseURL

	return &Client{
		client:    client,
		apiKey:    apiKey,
		secretKey: secretKey,
	}
}

// GetKlines 获取K线数据 - 基础版本，向后兼容
func (c *Client) GetKlines(ctx context.Context, symbol string, interval string, limit int) ([]*KlineData, error) {
	return c.GetKlinesWithTimeRange(ctx, symbol, interval, 0, 0, limit)
}

// GetKlinesWithTimeRange 获取指定时间范围的K线数据 - 完整版本
func (c *Client) GetKlinesWithTimeRange(ctx context.Context, symbol string, interval string, startTime, endTime int64, limit int) ([]*KlineData, error) {
	service := c.client.NewKlinesService().
		Symbol(symbol).
		Interval(interval)

	if limit > 0 {
		service = service.Limit(limit)
	}

	if startTime > 0 {
		service = service.StartTime(startTime)
	}

	if endTime > 0 {
		service = service.EndTime(endTime)
	}

	klines, err := service.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get klines: %w", err)
	}

	result := make([]*KlineData, len(klines))
	for i, kline := range klines {
		open, _ := decimal.NewFromString(kline.Open)
		high, _ := decimal.NewFromString(kline.High)
		low, _ := decimal.NewFromString(kline.Low)
		close, _ := decimal.NewFromString(kline.Close)
		volume, _ := decimal.NewFromString(kline.Volume)
		quoteVolume, _ := decimal.NewFromString(kline.QuoteAssetVolume)
		takerBuyVolume, _ := decimal.NewFromString(kline.TakerBuyBaseAssetVolume)
		takerBuyQuoteVolume, _ := decimal.NewFromString(kline.TakerBuyQuoteAssetVolume)

		result[i] = &KlineData{
			Symbol:              symbol,
			OpenTime:            kline.OpenTime,
			Open:                open,
			High:                high,
			Low:                 low,
			Close:               close,
			Volume:              volume,
			CloseTime:           kline.CloseTime,
			QuoteVolume:         quoteVolume,
			TakerBuyVolume:      takerBuyVolume,
			TakerBuyQuoteVolume: takerBuyQuoteVolume,
		}
	}

	return result, nil
}

// GetCurrentPrice 获取当前价格
func (c *Client) GetCurrentPrice(ctx context.Context, symbol string) (decimal.Decimal, error) {
	prices, err := c.client.NewListPricesService().Symbol(symbol).Do(ctx)
	if err != nil {
		return decimal.Zero, fmt.Errorf("failed to get current price: %w", err)
	}

	if len(prices) == 0 {
		return decimal.Zero, fmt.Errorf("no price data for symbol %s", symbol)
	}

	price, err := decimal.NewFromString(prices[0].Price)
	if err != nil {
		return decimal.Zero, fmt.Errorf("failed to parse price: %w", err)
	}

	return price, nil
}

// PlaceMarketBuyOrder 下市价买单
func (c *Client) PlaceMarketBuyOrder(ctx context.Context, symbol string, quantity decimal.Decimal) (*OrderResult, error) {
	order, err := c.client.NewCreateOrderService().
		Symbol(symbol).
		Side(binance.SideTypeBuy).
		Type(binance.OrderTypeMarket).
		Quantity(quantity.String()).
		Do(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to place buy order: %w", err)
	}

	return NewOrderResultFromCreateResponse(order), nil
}

// PlaceMarketSellOrder 下市价卖单
func (c *Client) PlaceMarketSellOrder(ctx context.Context, symbol string, quantity decimal.Decimal) (*OrderResult, error) {
	order, err := c.client.NewCreateOrderService().
		Symbol(symbol).
		Side(binance.SideTypeSell).
		Type(binance.OrderTypeMarket).
		Quantity(quantity.String()).
		Do(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to place sell order: %w", err)
	}

	return NewOrderResultFromCreateResponse(order), nil
}

// PlaceLimitBuyOrder 下限价买单
func (c *Client) PlaceLimitBuyOrder(ctx context.Context, symbol string, quantity, price decimal.Decimal) (*OrderResult, error) {
	order, err := c.client.NewCreateOrderService().
		Symbol(symbol).
		Side(binance.SideTypeBuy).
		Type(binance.OrderTypeLimit).
		TimeInForce(binance.TimeInForceTypeGTC).
		Quantity(quantity.String()).
		Price(price.String()).
		Do(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to place limit buy order: %w", err)
	}

	return NewOrderResultFromCreateResponse(order), nil
}

// PlaceLimitSellOrder 下限价卖单
func (c *Client) PlaceLimitSellOrder(ctx context.Context, symbol string, quantity, price decimal.Decimal) (*OrderResult, error) {
	order, err := c.client.NewCreateOrderService().
		Symbol(symbol).
		Side(binance.SideTypeSell).
		Type(binance.OrderTypeLimit).
		TimeInForce(binance.TimeInForceTypeGTC).
		Quantity(quantity.String()).
		Price(price.String()).
		Do(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to place limit sell order: %w", err)
	}

	return NewOrderResultFromCreateResponse(order), nil
}

// GetAccountBalances 获取账户余额
func (c *Client) GetAccountBalances(ctx context.Context) ([]*AccountBalance, error) {
	account, err := c.client.NewGetAccountService().Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get account balances: %w", err)
	}

	balances := make([]*AccountBalance, len(account.Balances))
	for i, balance := range account.Balances {
		free, _ := decimal.NewFromString(balance.Free)
		locked, _ := decimal.NewFromString(balance.Locked)

		balances[i] = &AccountBalance{
			Asset:  balance.Asset,
			Free:   free,
			Locked: locked,
		}
	}

	return balances, nil
}

// GetAssetBalance 获取特定资产余额
func (c *Client) GetAssetBalance(ctx context.Context, asset string) (*AccountBalance, error) {
	balances, err := c.GetAccountBalances(ctx)
	if err != nil {
		return nil, err
	}

	for _, balance := range balances {
		if balance.Asset == asset {
			return balance, nil
		}
	}

	return &AccountBalance{
		Asset:  asset,
		Free:   decimal.Zero,
		Locked: decimal.Zero,
	}, nil
}

// CancelOrder 取消订单
func (c *Client) CancelOrder(ctx context.Context, symbol string, orderID int64) error {
	_, err := c.client.NewCancelOrderService().
		Symbol(symbol).
		OrderID(orderID).
		Do(ctx)

	if err != nil {
		return fmt.Errorf("failed to cancel order: %w", err)
	}

	return nil
}

// GetOrder 获取订单信息
func (c *Client) GetOrder(ctx context.Context, symbol string, orderID int64) (*OrderResult, error) {
	order, err := c.client.NewGetOrderService().
		Symbol(symbol).
		OrderID(orderID).
		Do(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	price, _ := decimal.NewFromString(order.Price)
	quantity, _ := decimal.NewFromString(order.OrigQuantity)

	return &OrderResult{
		Symbol:        order.Symbol,
		OrderID:       order.OrderID,
		ClientOrderID: order.ClientOrderID,
		Price:         price,
		Quantity:      quantity,
		Side:          string(order.Side),
		Status:        string(order.Status),
		Type:          string(order.Type),
		TimeInForce:   string(order.TimeInForce),
		TransactTime:  order.Time,
	}, nil
}

// NewOrderResultFromCreateResponse 从CreateOrderResponse创建OrderResult
func NewOrderResultFromCreateResponse(order *binance.CreateOrderResponse) *OrderResult {
	price, _ := decimal.NewFromString(order.Price)
	quantity, _ := decimal.NewFromString(order.OrigQuantity)

	return &OrderResult{
		Symbol:        order.Symbol,
		OrderID:       order.OrderID,
		ClientOrderID: order.ClientOrderID,
		Price:         price,
		Quantity:      quantity,
		Side:          string(order.Side),
		Status:        string(order.Status),
		Type:          string(order.Type),
		TimeInForce:   string(order.TimeInForce),
		TransactTime:  order.TransactTime,
	}
}

// Ping 测试连接
func (c *Client) Ping(ctx context.Context) error {
	err := c.client.NewPingService().Do(ctx)
	if err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}
	return nil
}

// GetServerTime 获取服务器时间
func (c *Client) GetServerTime(ctx context.Context) (time.Time, error) {
	serverTime, err := c.client.NewServerTimeService().Do(ctx)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get server time: %w", err)
	}
	return time.Unix(0, serverTime*int64(time.Millisecond)), nil
}
