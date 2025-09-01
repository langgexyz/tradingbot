package engine

import (
	"context"
	"time"

	"tradingbot/src/cex"

	"github.com/xpwu/go-log/log"
)

// DataFeed 统一的数据喂入接口
type DataFeed interface {
	// Start 开始数据流
	Start(ctx context.Context) error

	// GetNext 获取下一个K线数据
	// 返回nil表示数据流结束
	GetNext(ctx context.Context) (*cex.KlineData, error)

	// Stop 停止数据流
	Stop() error

	// GetCurrentTime 获取当前模拟时间
	GetCurrentTime() time.Time
}

// BacktestDataFeed 回测数据喂入器
type BacktestDataFeed struct {
	klines      []*cex.KlineData
	currentIdx  int
	currentTime time.Time
	finished    bool
}

// NewBacktestDataFeed 创建回测数据喂入器
func NewBacktestDataFeed(klines []*cex.KlineData) *BacktestDataFeed {
	var startTime time.Time
	if len(klines) > 0 {
		startTime = klines[0].OpenTime
	} else {
		startTime = time.Now()
	}

	return &BacktestDataFeed{
		klines:      klines,
		currentIdx:  0,
		currentTime: startTime,
		finished:    false,
	}
}

func (f *BacktestDataFeed) Start(ctx context.Context) error {
	f.currentIdx = 0
	f.finished = false
	if len(f.klines) > 0 {
		f.currentTime = f.klines[0].OpenTime
	}
	return nil
}

func (f *BacktestDataFeed) GetNext(ctx context.Context) (*cex.KlineData, error) {
	if f.finished || f.currentIdx >= len(f.klines) {
		return nil, nil // 数据流结束
	}

	kline := f.klines[f.currentIdx]
	f.currentIdx++
	f.currentTime = kline.OpenTime

	if f.currentIdx >= len(f.klines) {
		f.finished = true
	}

	return kline, nil
}

func (f *BacktestDataFeed) Stop() error {
	f.finished = true
	return nil
}

func (f *BacktestDataFeed) GetCurrentTime() time.Time {
	return f.currentTime
}

// LiveDataFeed 实盘数据喂入器
type LiveDataFeed struct {
	cexClient   cex.CEXClient
	tradingPair cex.TradingPair
	interval    string
	ticker      *time.Ticker
	stopChan    chan struct{}
	currentTime time.Time
}

// NewLiveDataFeed 创建实盘数据喂入器
func NewLiveDataFeed(cexClient cex.CEXClient, tradingPair cex.TradingPair, interval string, tickerInterval time.Duration) *LiveDataFeed {
	return &LiveDataFeed{
		cexClient:   cexClient,
		tradingPair: tradingPair,
		interval:    interval,
		ticker:      time.NewTicker(tickerInterval),
		stopChan:    make(chan struct{}),
		currentTime: time.Now(),
	}
}

func (f *LiveDataFeed) Start(ctx context.Context) error {
	f.currentTime = time.Now()
	return nil
}

func (f *LiveDataFeed) GetNext(ctx context.Context) (*cex.KlineData, error) {
	ctx, logger := log.WithCtx(ctx)

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-f.stopChan:
		return nil, nil // 数据流结束
	case <-f.ticker.C:
		f.currentTime = time.Now()
		logger.Info("📡 LiveDataFeed开始获取数据",
			"trading_pair", f.tradingPair.String(),
			"interval", f.interval,
			"current_time", f.currentTime.Format("15:04:05"))

		// 获取最新K线数据
		klines, err := f.cexClient.GetKlines(ctx, f.tradingPair, f.interval, 1)
		if err != nil {
			logger.Error("❌ 获取K线数据失败", "error", err)
			return nil, err
		}

		if len(klines) == 0 {
			logger.Info("⚠️ 没有获取到K线数据")
			return nil, nil
		}

		logger.Info("✅ 成功获取K线数据",
			"klines_count", len(klines),
			"kline_open_time", klines[0].OpenTime.Format("15:04:05"),
			"close_price", klines[0].Close.String())
		return klines[0], nil
	}
}

func (f *LiveDataFeed) Stop() error {
	// 安全地关闭channel，防止重复关闭
	select {
	case <-f.stopChan:
		// channel已经关闭，不做任何操作
	default:
		close(f.stopChan)
	}

	if f.ticker != nil {
		f.ticker.Stop()
	}
	return nil
}

func (f *LiveDataFeed) GetCurrentTime() time.Time {
	return f.currentTime
}
