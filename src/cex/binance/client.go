package binance

import (
	"context"
	"fmt"
	"strings"
	"time"

	"tradingbot/src/cex"
	"tradingbot/src/database"

	"github.com/adshao/go-binance/v2"
	"github.com/shopspring/decimal"
)

// Client Binance客户端实现
type Client struct {
	client    *binance.Client
	apiKey    string
	secretKey string
	database  *database.PostgresDB // 内部管理的数据库连接
}

// NewClient 创建Binance客户端
func NewClient(apiKey, secretKey string) *Client {
	binanceClient := binance.NewClient(apiKey, secretKey)

	// 初始化数据库连接
	config := &ConfigValue
	dbConfig := database.GetDatabaseConfigForCEX(config.DBName)

	var db *database.PostgresDB
	if dbConfig.Host != "" {
		fmt.Printf("🗄️ Connecting to binance database...")
		var err error
		db, err = database.NewPostgresDB(
			dbConfig.Host,
			dbConfig.Port,
			dbConfig.User,
			dbConfig.Password,
			dbConfig.DBName,
			dbConfig.SSLMode,
		)
		if err != nil {
			fmt.Printf(" failed: %v\n", err)
			fmt.Println("⚠️ Database unavailable, using network only")
			db = nil
		} else {
			fmt.Println(" connected!")
		}
	}

	return &Client{
		client:    binanceClient,
		apiKey:    apiKey,
		secretKey: secretKey,
		database:  db,
	}
}

// GetName 获取交易所名称
func (c *Client) GetName() string {
	return "binance"
}

// GetDatabase 获取数据库连接
func (c *Client) GetDatabase() interface{} {
	return c.database
}

// GetTradingFee 获取交易手续费率
func (c *Client) GetTradingFee() float64 {
	config := &ConfigValue
	return config.Fee
}

// tradingPairToSymbol 将标准化交易对转换为Binance格式
func (c *Client) tradingPairToSymbol(pair cex.TradingPair) string {
	// Binance格式: BTCUSDT, PEPEUSDT (无分隔符)
	return strings.ToUpper(pair.Base) + strings.ToUpper(pair.Quote)
}

// convertKlineData 转换Binance K线数据为标准格式
func (c *Client) convertKlineData(kline *binance.Kline, pair cex.TradingPair) *cex.KlineData {
	open, _ := decimal.NewFromString(kline.Open)
	high, _ := decimal.NewFromString(kline.High)
	low, _ := decimal.NewFromString(kline.Low)
	close, _ := decimal.NewFromString(kline.Close)
	volume, _ := decimal.NewFromString(kline.Volume)
	quoteVolume, _ := decimal.NewFromString(kline.QuoteAssetVolume)
	takerBuyVolume, _ := decimal.NewFromString(kline.TakerBuyBaseAssetVolume)
	takerBuyQuoteVolume, _ := decimal.NewFromString(kline.TakerBuyQuoteAssetVolume)

	return &cex.KlineData{
		TradingPair:         pair,
		OpenTime:            time.Unix(kline.OpenTime/1000, 0),
		Open:                open,
		High:                high,
		Low:                 low,
		Close:               close,
		Volume:              volume,
		CloseTime:           time.Unix(kline.CloseTime/1000, 0),
		QuoteVolume:         quoteVolume,
		TakerBuyVolume:      takerBuyVolume,
		TakerBuyQuoteVolume: takerBuyQuoteVolume,
	}
}

// GetKlines 获取K线数据
func (c *Client) GetKlines(ctx context.Context, pair cex.TradingPair, interval string, limit int) ([]*cex.KlineData, error) {
	symbol := c.tradingPairToSymbol(pair)

	klines, err := c.client.NewKlinesService().
		Symbol(symbol).
		Interval(interval).
		Limit(limit).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get klines from Binance: %w", err)
	}

	result := make([]*cex.KlineData, len(klines))
	for i, kline := range klines {
		result[i] = c.convertKlineData(kline, pair)
	}

	return result, nil
}

// GetKlinesWithTimeRange 获取指定时间范围的K线数据
func (c *Client) GetKlinesWithTimeRange(ctx context.Context, pair cex.TradingPair, interval string, startTime, endTime time.Time, limit int) ([]*cex.KlineData, error) {
	symbol := c.tradingPairToSymbol(pair)

	// 批量获取数据以克服1000条限制
	var allKlines []*cex.KlineData
	currentStart := startTime

	for currentStart.Before(endTime) {
		klines, err := c.client.NewKlinesService().
			Symbol(symbol).
			Interval(interval).
			StartTime(currentStart.UnixMilli()).
			EndTime(endTime.UnixMilli()).
			Limit(limit).
			Do(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get klines from Binance: %w", err)
		}

		if len(klines) == 0 {
			break
		}

		// 转换数据
		for _, kline := range klines {
			allKlines = append(allKlines, c.convertKlineData(kline, pair))
		}

		// 更新下一批的开始时间
		lastKline := klines[len(klines)-1]
		currentStart = time.Unix(lastKline.CloseTime/1000, 0).Add(time.Millisecond)

		// 如果返回的数据少于限制，说明已经获取完毕
		if len(klines) < limit {
			break
		}
	}

	return allKlines, nil
}

// Buy 买入
func (c *Client) Buy(ctx context.Context, order cex.BuyOrderRequest) (*cex.OrderResult, error) {
	symbol := c.tradingPairToSymbol(order.TradingPair)

	service := c.client.NewCreateOrderService().
		Symbol(symbol).
		Side(binance.SideTypeBuy).
		Type(binance.OrderType(order.Type)).
		Quantity(order.Quantity.String())

	if order.Type == cex.OrderTypeLimit {
		service = service.Price(order.Price.String()).TimeInForce(binance.TimeInForceTypeGTC)
	}

	result, err := service.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to place buy order on Binance: %w", err)
	}

	price, _ := decimal.NewFromString(result.Price)
	quantity, _ := decimal.NewFromString(result.ExecutedQuantity)

	return &cex.OrderResult{
		TradingPair:   order.TradingPair,
		OrderID:       fmt.Sprintf("%d", result.OrderID),
		ClientOrderID: result.ClientOrderID,
		Price:         price,
		Quantity:      quantity,
		Side:          cex.OrderSideBuy,
		Status:        string(result.Status),
		Type:          cex.OrderType(result.Type),
		TransactTime:  time.Unix(result.TransactTime/1000, 0),
	}, nil
}

// Sell 卖出
func (c *Client) Sell(ctx context.Context, order cex.SellOrderRequest) (*cex.OrderResult, error) {
	symbol := c.tradingPairToSymbol(order.TradingPair)

	service := c.client.NewCreateOrderService().
		Symbol(symbol).
		Side(binance.SideTypeSell).
		Type(binance.OrderType(order.Type)).
		Quantity(order.Quantity.String())

	if order.Type == cex.OrderTypeLimit {
		service = service.Price(order.Price.String()).TimeInForce(binance.TimeInForceTypeGTC)
	}

	result, err := service.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to place sell order on Binance: %w", err)
	}

	price, _ := decimal.NewFromString(result.Price)
	quantity, _ := decimal.NewFromString(result.ExecutedQuantity)

	return &cex.OrderResult{
		TradingPair:   order.TradingPair,
		OrderID:       fmt.Sprintf("%d", result.OrderID),
		ClientOrderID: result.ClientOrderID,
		Price:         price,
		Quantity:      quantity,
		Side:          cex.OrderSideSell,
		Status:        string(result.Status),
		Type:          cex.OrderType(result.Type),
		TransactTime:  time.Unix(result.TransactTime/1000, 0),
	}, nil
}

// GetAccount 获取账户信息
func (c *Client) GetAccount(ctx context.Context) ([]*cex.AccountBalance, error) {
	account, err := c.client.NewGetAccountService().Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get account from Binance: %w", err)
	}

	balances := make([]*cex.AccountBalance, len(account.Balances))
	for i, balance := range account.Balances {
		free, _ := decimal.NewFromString(balance.Free)
		locked, _ := decimal.NewFromString(balance.Locked)

		balances[i] = &cex.AccountBalance{
			Asset:  balance.Asset,
			Free:   free,
			Locked: locked,
		}
	}

	return balances, nil
}

// Ping 测试连接
func (c *Client) Ping(ctx context.Context) error {
	err := c.client.NewPingService().Do(ctx)
	if err != nil {
		return fmt.Errorf("Binance ping failed: %w", err)
	}
	return nil
}
