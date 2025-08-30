package backtest

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"go-build-stream-gateway-go-server-main/src/binance"
	"go-build-stream-gateway-go-server-main/src/timeframes"

	"github.com/shopspring/decimal"
)

// Engine 回测引擎
type Engine struct {
	symbol         string
	timeframe      timeframes.Timeframe
	startTime      time.Time
	endTime        time.Time
	initialCapital decimal.Decimal
	commission     decimal.Decimal // 手续费率
	slippage       decimal.Decimal // 滑点

	// 数据
	klineData    []*binance.KlineData
	currentIndex int

	// 账户状态
	cash      decimal.Decimal
	position  decimal.Decimal // 持仓数量
	portfolio decimal.Decimal // 总资产

	// 交易记录
	trades []*Trade
	orders []*Order

	// 策略
	strategy Strategy

	// 统计
	stats *Statistics
}

// Trade 交易记录
type Trade struct {
	ID         int64           `json:"id"`
	Symbol     string          `json:"symbol"`
	Side       string          `json:"side"` // "buy" or "sell"
	Quantity   decimal.Decimal `json:"quantity"`
	Price      decimal.Decimal `json:"price"`
	Commission decimal.Decimal `json:"commission"`
	Timestamp  time.Time       `json:"timestamp"`
	PnL        decimal.Decimal `json:"pnl"` // 盈亏
}

// Order 订单
type Order struct {
	ID        int64           `json:"id"`
	Symbol    string          `json:"symbol"`
	Side      string          `json:"side"`
	Type      string          `json:"type"` // "market" or "limit"
	Quantity  decimal.Decimal `json:"quantity"`
	Price     decimal.Decimal `json:"price"`
	Status    string          `json:"status"` // "pending", "filled", "cancelled"
	Timestamp time.Time       `json:"timestamp"`
}

// Strategy 交易策略接口
type Strategy interface {
	// OnData 处理新的K线数据
	OnData(ctx context.Context, kline *binance.KlineData, portfolio *Portfolio) ([]*Signal, error)

	// GetName 获取策略名称
	GetName() string

	// GetParams 获取策略参数
	GetParams() map[string]interface{}
}

// Signal 交易信号
type Signal struct {
	Type      string          `json:"type"` // "buy", "sell", "close"
	Symbol    string          `json:"symbol"`
	Quantity  decimal.Decimal `json:"quantity"`
	Price     decimal.Decimal `json:"price"`      // 限价单价格，市价单可为空
	OrderType string          `json:"order_type"` // "market", "limit"
	Reason    string          `json:"reason"`     // 信号原因
	Timestamp time.Time       `json:"timestamp"`
}

// Portfolio 投资组合信息
type Portfolio struct {
	Cash         decimal.Decimal `json:"cash"`
	Position     decimal.Decimal `json:"position"`
	Portfolio    decimal.Decimal `json:"portfolio"`
	CurrentPrice decimal.Decimal `json:"current_price"`
	Timestamp    time.Time       `json:"timestamp"`
}

// Statistics 回测统计
type Statistics struct {
	TotalTrades      int             `json:"total_trades"`
	WinningTrades    int             `json:"winning_trades"`
	LosingTrades     int             `json:"losing_trades"`
	WinRate          decimal.Decimal `json:"win_rate"`
	TotalReturn      decimal.Decimal `json:"total_return"`
	AnnualizedReturn decimal.Decimal `json:"annualized_return"`
	MaxDrawdown      decimal.Decimal `json:"max_drawdown"`
	SharpeRatio      decimal.Decimal `json:"sharpe_ratio"`
	TotalCommission  decimal.Decimal `json:"total_commission"`
	TotalPnL         decimal.Decimal `json:"total_pnl"`

	// 详细收益曲线
	EquityCurve  []EquityPoint     `json:"equity_curve"`
	DailyReturns []decimal.Decimal `json:"daily_returns"`
}

// EquityPoint 权益曲线点
type EquityPoint struct {
	Timestamp time.Time       `json:"timestamp"`
	Portfolio decimal.Decimal `json:"portfolio"`
	Cash      decimal.Decimal `json:"cash"`
	Position  decimal.Decimal `json:"position"`
	Price     decimal.Decimal `json:"price"`
}

// NewEngine 创建回测引擎
func NewEngine(symbol string, tf timeframes.Timeframe, startTime, endTime time.Time, initialCapital decimal.Decimal) *Engine {
	return &Engine{
		symbol:         symbol,
		timeframe:      tf,
		startTime:      startTime,
		endTime:        endTime,
		initialCapital: initialCapital,
		commission:     decimal.NewFromFloat(0.001),  // 默认0.1%手续费
		slippage:       decimal.NewFromFloat(0.0001), // 默认0.01%滑点
		cash:           initialCapital,
		position:       decimal.Zero,
		portfolio:      initialCapital,
		currentIndex:   0,
		trades:         make([]*Trade, 0),
		orders:         make([]*Order, 0),
		stats:          &Statistics{},
	}
}

// LoadData 加载历史数据
func (e *Engine) LoadData(ctx context.Context, client *binance.Client) error {
	// 简化：直接获取最近100条K线数据用于回测
	fmt.Printf("📊 Loading recent 100 klines for %s (%s)...\n", e.symbol, e.timeframe)

	data, err := client.GetKlines(ctx, e.symbol, e.timeframe.GetBinanceInterval(), 100)
	if err != nil {
		return fmt.Errorf("failed to load kline data: %w", err)
	}

	if len(data) == 0 {
		return fmt.Errorf("no data loaded for symbol %s", e.symbol)
	}

	// 按时间排序
	sort.Slice(data, func(i, j int) bool {
		return data[i].OpenTime < data[j].OpenTime
	})

	e.klineData = data

	// 更新实际的开始和结束时间
	e.startTime = time.Unix(data[0].OpenTime/1000, 0)
	e.endTime = time.Unix(data[len(data)-1].OpenTime/1000, 0)

	fmt.Printf("✓ Loaded %d klines from %s to %s\n",
		len(data),
		e.startTime.Format("2006-01-02 15:04"),
		e.endTime.Format("2006-01-02 15:04"))

	return nil
}

// SetStrategy 设置交易策略
func (e *Engine) SetStrategy(strategy Strategy) {
	e.strategy = strategy
}

// SetCommission 设置手续费率
func (e *Engine) SetCommission(commission float64) {
	e.commission = decimal.NewFromFloat(commission)
}

// SetSlippage 设置滑点
func (e *Engine) SetSlippage(slippage float64) {
	e.slippage = decimal.NewFromFloat(slippage)
}

// Run 运行回测
func (e *Engine) Run(ctx context.Context) error {
	if e.strategy == nil {
		return fmt.Errorf("strategy not set")
	}

	if len(e.klineData) == 0 {
		return fmt.Errorf("no data loaded")
	}

	// 初始化统计
	e.stats.EquityCurve = make([]EquityPoint, 0)

	// 遍历历史数据
	for e.currentIndex = 0; e.currentIndex < len(e.klineData); e.currentIndex++ {
		kline := e.klineData[e.currentIndex]

		// 更新投资组合价值
		e.updatePortfolio(kline.Close)

		// 记录权益曲线
		point := EquityPoint{
			Timestamp: time.Unix(kline.OpenTime/1000, 0),
			Portfolio: e.portfolio,
			Cash:      e.cash,
			Position:  e.position,
			Price:     kline.Close,
		}
		e.stats.EquityCurve = append(e.stats.EquityCurve, point)

		// 创建投资组合快照
		portfolio := &Portfolio{
			Cash:         e.cash,
			Position:     e.position,
			Portfolio:    e.portfolio,
			CurrentPrice: kline.Close,
			Timestamp:    time.Unix(kline.OpenTime/1000, 0),
		}

		// 执行策略
		signals, err := e.strategy.OnData(ctx, kline, portfolio)
		if err != nil {
			return fmt.Errorf("strategy error: %w", err)
		}

		// 处理交易信号
		for _, signal := range signals {
			err := e.executeSignal(signal, kline)
			if err != nil {
				// 记录错误但继续执行
				fmt.Printf("Signal execution error: %v\n", err)
			}
		}
	}

	// 计算最终统计
	e.calculateStatistics()

	return nil
}

// executeSignal 执行交易信号
func (e *Engine) executeSignal(signal *Signal, kline *binance.KlineData) error {
	var executionPrice decimal.Decimal

	if signal.OrderType == "market" {
		// 市价单使用当前收盘价 + 滑点
		if signal.Type == "buy" {
			executionPrice = kline.Close.Mul(decimal.NewFromFloat(1).Add(e.slippage))
		} else {
			executionPrice = kline.Close.Mul(decimal.NewFromFloat(1).Sub(e.slippage))
		}
	} else {
		// 限价单使用指定价格
		executionPrice = signal.Price
	}

	switch signal.Type {
	case "buy":
		return e.executeBuy(signal.Quantity, executionPrice, kline)
	case "sell":
		return e.executeSell(signal.Quantity, executionPrice, kline)
	case "close":
		if e.position.GreaterThan(decimal.Zero) {
			return e.executeSell(e.position, executionPrice, kline)
		}
	}

	return nil
}

// executeBuy 执行买入
func (e *Engine) executeBuy(quantity, price decimal.Decimal, kline *binance.KlineData) error {
	totalCost := quantity.Mul(price)
	commission := totalCost.Mul(e.commission)

	if e.cash.LessThan(totalCost.Add(commission)) {
		return fmt.Errorf("insufficient cash")
	}

	e.cash = e.cash.Sub(totalCost).Sub(commission)
	e.position = e.position.Add(quantity)

	// 记录交易
	trade := &Trade{
		ID:         int64(len(e.trades) + 1),
		Symbol:     e.symbol,
		Side:       "buy",
		Quantity:   quantity,
		Price:      price,
		Commission: commission,
		Timestamp:  time.Unix(kline.OpenTime/1000, 0),
	}
	e.trades = append(e.trades, trade)

	return nil
}

// executeSell 执行卖出
func (e *Engine) executeSell(quantity, price decimal.Decimal, kline *binance.KlineData) error {
	if e.position.LessThan(quantity) {
		return fmt.Errorf("insufficient position")
	}

	totalValue := quantity.Mul(price)
	commission := totalValue.Mul(e.commission)

	e.cash = e.cash.Add(totalValue).Sub(commission)
	e.position = e.position.Sub(quantity)

	// 计算盈亏（简化计算，实际应该考虑FIFO等）
	var pnl decimal.Decimal
	if len(e.trades) > 0 {
		// 找到最近的买入交易计算盈亏
		for i := len(e.trades) - 1; i >= 0; i-- {
			if e.trades[i].Side == "buy" {
				buyPrice := e.trades[i].Price
				pnl = quantity.Mul(price.Sub(buyPrice)).Sub(commission)
				break
			}
		}
	}

	// 记录交易
	trade := &Trade{
		ID:         int64(len(e.trades) + 1),
		Symbol:     e.symbol,
		Side:       "sell",
		Quantity:   quantity,
		Price:      price,
		Commission: commission,
		Timestamp:  time.Unix(kline.OpenTime/1000, 0),
		PnL:        pnl,
	}
	e.trades = append(e.trades, trade)

	return nil
}

// updatePortfolio 更新投资组合价值
func (e *Engine) updatePortfolio(currentPrice decimal.Decimal) {
	positionValue := e.position.Mul(currentPrice)
	e.portfolio = e.cash.Add(positionValue)
}

// calculateStatistics 计算统计指标
func (e *Engine) calculateStatistics() {
	e.stats.TotalTrades = len(e.trades)

	var totalPnL decimal.Decimal
	var totalCommission decimal.Decimal
	winningTrades := 0
	losingTrades := 0

	for _, trade := range e.trades {
		totalCommission = totalCommission.Add(trade.Commission)
		if trade.Side == "sell" && !trade.PnL.IsZero() {
			totalPnL = totalPnL.Add(trade.PnL)
			if trade.PnL.GreaterThan(decimal.Zero) {
				winningTrades++
			} else {
				losingTrades++
			}
		}
	}

	e.stats.WinningTrades = winningTrades
	e.stats.LosingTrades = losingTrades
	e.stats.TotalCommission = totalCommission
	e.stats.TotalPnL = totalPnL

	// 计算胜率
	if e.stats.TotalTrades > 0 {
		winRate := decimal.NewFromInt(int64(winningTrades)).Div(decimal.NewFromInt(int64(winningTrades + losingTrades)))
		e.stats.WinRate = winRate
	}

	// 计算总收益率
	e.stats.TotalReturn = e.portfolio.Sub(e.initialCapital).Div(e.initialCapital)

	// 计算年化收益率
	days := e.endTime.Sub(e.startTime).Hours() / 24
	if days > 0 {
		annualFactor := 365 / days
		totalReturnFloat, _ := e.stats.TotalReturn.Float64()
		annualizedReturn := math.Pow(1+totalReturnFloat, annualFactor) - 1
		e.stats.AnnualizedReturn = decimal.NewFromFloat(annualizedReturn)
	}

	// 计算最大回撤
	e.calculateMaxDrawdown()

	// 计算夏普比率
	e.calculateSharpeRatio()
}

// calculateMaxDrawdown 计算最大回撤
func (e *Engine) calculateMaxDrawdown() {
	if len(e.stats.EquityCurve) == 0 {
		return
	}

	maxPortfolio := e.stats.EquityCurve[0].Portfolio
	maxDrawdown := decimal.Zero

	for _, point := range e.stats.EquityCurve {
		if point.Portfolio.GreaterThan(maxPortfolio) {
			maxPortfolio = point.Portfolio
		}

		drawdown := maxPortfolio.Sub(point.Portfolio).Div(maxPortfolio)
		if drawdown.GreaterThan(maxDrawdown) {
			maxDrawdown = drawdown
		}
	}

	e.stats.MaxDrawdown = maxDrawdown
}

// calculateSharpeRatio 计算夏普比率
func (e *Engine) calculateSharpeRatio() {
	if len(e.stats.EquityCurve) < 2 {
		return
	}

	// 计算日收益率
	returns := make([]decimal.Decimal, len(e.stats.EquityCurve)-1)

	for i := 1; i < len(e.stats.EquityCurve); i++ {
		prevValue := e.stats.EquityCurve[i-1].Portfolio
		currValue := e.stats.EquityCurve[i].Portfolio

		if prevValue.GreaterThan(decimal.Zero) {
			returns[i-1] = currValue.Sub(prevValue).Div(prevValue)
		}
	}

	e.stats.DailyReturns = returns

	// 计算平均收益率和标准差
	if len(returns) == 0 {
		return
	}

	sum := decimal.Zero
	for _, ret := range returns {
		sum = sum.Add(ret)
	}
	meanReturn := sum.Div(decimal.NewFromInt(int64(len(returns))))

	varianceSum := decimal.Zero
	for _, ret := range returns {
		diff := ret.Sub(meanReturn)
		varianceSum = varianceSum.Add(diff.Mul(diff))
	}

	if len(returns) > 1 {
		variance := varianceSum.Div(decimal.NewFromInt(int64(len(returns) - 1)))
		varianceFloat, _ := variance.Float64()
		std := decimal.NewFromFloat(math.Sqrt(varianceFloat))

		if std.GreaterThan(decimal.Zero) {
			// 假设无风险利率为0
			e.stats.SharpeRatio = meanReturn.Div(std).Mul(decimal.NewFromFloat(math.Sqrt(365)))
		}
	}
}

// GetStatistics 获取回测统计
func (e *Engine) GetStatistics() *Statistics {
	return e.stats
}

// GetTrades 获取交易记录
func (e *Engine) GetTrades() []*Trade {
	return e.trades
}

// GetEquityCurve 获取权益曲线
func (e *Engine) GetEquityCurve() []EquityPoint {
	return e.stats.EquityCurve
}
