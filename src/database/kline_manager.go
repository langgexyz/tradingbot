package database

import (
	"context"
	"time"

	"go-build-stream-gateway-go-server-main/src/binance"

	"github.com/xpwu/go-log/log"
)

// KlineManager K线数据管理器
type KlineManager struct {
	db     *PostgresDB
	client *binance.Client
}

// NewKlineManager 创建K线数据管理器
func NewKlineManager(db *PostgresDB, client *binance.Client) *KlineManager {
	return &KlineManager{
		db:     db,
		client: client,
	}
}

// GetKlines 智能获取K线数据（优先数据库，缺失时从网络补充）
func (km *KlineManager) GetKlines(ctx context.Context, symbol, timeframe string, limit int) ([]*binance.KlineData, error) {
	ctx, logger := log.WithCtx(ctx)
	logger.PushPrefix("KlineManager")

	// 1. 先从数据库获取现有数据
	logger.Debug("尝试从数据库获取K线数据", "symbol", symbol, "timeframe", timeframe, "limit", limit)

	dbKlines, err := km.db.GetKlines(ctx, symbol, timeframe, 0, 0, limit)
	if err != nil {
		logger.Error("从数据库获取K线数据失败", "error", err)
		// 数据库失败，直接从网络获取
		return km.getFromNetwork(ctx, symbol, timeframe, limit)
	}

	// 2. 检查数据是否足够
	if len(dbKlines) >= limit {
		logger.Info("数据库数据充足", "count", len(dbKlines), "required", limit)
		// 返回最新的limit条数据
		if len(dbKlines) > limit {
			return dbKlines[len(dbKlines)-limit:], nil
		}
		return dbKlines, nil
	}

	// 3. 数据不足，需要从网络补充
	logger.Info("数据库数据不足，从网络补充", "db_count", len(dbKlines), "required", limit)

	// 获取最新的数据时间
	var lastTime int64
	if len(dbKlines) > 0 {
		lastTime = dbKlines[len(dbKlines)-1].OpenTime
	}

	// 从网络获取更多数据
	networkKlines, err := km.getFromNetwork(ctx, symbol, timeframe, limit*2) // 获取更多数据确保覆盖
	if err != nil {
		logger.Error("从网络获取K线数据失败", "error", err)
		// 网络也失败，返回数据库中的数据
		return dbKlines, nil
	}

	// 4. 合并数据并保存新数据到数据库
	allKlines := km.mergeKlines(dbKlines, networkKlines, lastTime)

	// 保存新数据到数据库
	if len(networkKlines) > 0 {
		newKlines := km.filterNewKlines(networkKlines, lastTime)
		if len(newKlines) > 0 {
			err = km.db.SaveKlines(ctx, symbol, timeframe, newKlines)
			if err != nil {
				logger.Error("保存K线数据到数据库失败", "error", err)
			} else {
				logger.Info("保存新K线数据到数据库", "count", len(newKlines))
			}
		}
	}

	// 5. 返回请求的数量
	if len(allKlines) > limit {
		return allKlines[len(allKlines)-limit:], nil
	}

	return allKlines, nil
}

// GetKlinesInRange 获取指定时间范围的K线数据
func (km *KlineManager) GetKlinesInRange(ctx context.Context, symbol, timeframe string, startTime, endTime int64) ([]*binance.KlineData, error) {
	ctx, logger := log.WithCtx(ctx)
	logger.PushPrefix("KlineManager")

	logger.Debug("获取时间范围K线数据",
		"symbol", symbol,
		"timeframe", timeframe,
		"start", time.Unix(startTime/1000, 0).Format("2006-01-02 15:04"),
		"end", time.Unix(endTime/1000, 0).Format("2006-01-02 15:04"))

	// 1. 从数据库获取范围内的数据
	dbKlines, err := km.db.GetKlines(ctx, symbol, timeframe, startTime, endTime, 0)
	if err != nil {
		logger.Error("从数据库获取范围K线数据失败", "error", err)
	}

	// 2. 检查数据完整性
	missingRanges := km.findMissingRanges(dbKlines, startTime, endTime, timeframe)

	if len(missingRanges) == 0 {
		logger.Info("数据库数据完整", "count", len(dbKlines))
		return dbKlines, nil
	}

	// 3. 补充缺失的数据
	logger.Info("发现缺失数据段", "missing_ranges", len(missingRanges))

	var allNewKlines []*binance.KlineData
	for _, missingRange := range missingRanges {
		logger.Debug("补充缺失数据段",
			"start", time.Unix(missingRange.Start/1000, 0).Format("2006-01-02 15:04"),
			"end", time.Unix(missingRange.End/1000, 0).Format("2006-01-02 15:04"))

		// 从网络获取缺失的数据
		newKlines, err := km.client.GetKlines(ctx, symbol, timeframe, 1000)
		if err != nil {
			logger.Error("获取缺失数据失败", "error", err)
			continue
		}

		if len(newKlines) > 0 {
			// 使用批量保存到数据库（高性能版本）
			err = km.db.SaveKlinesBatch(ctx, symbol, timeframe, newKlines)
			if err != nil {
				logger.Error("批量保存缺失数据失败", "error", err)
			} else {
				logger.Info("批量保存缺失数据", "count", len(newKlines))
			}

			allNewKlines = append(allNewKlines, newKlines...)
		}
	}

	// 4. 重新从数据库获取完整数据
	finalKlines, err := km.db.GetKlines(ctx, symbol, timeframe, startTime, endTime, 0)
	if err != nil {
		logger.Error("重新获取完整数据失败", "error", err)
		// 合并原有数据和新数据
		return km.mergeKlines(dbKlines, allNewKlines, 0), nil
	}

	logger.Info("获取完整K线数据", "total_count", len(finalKlines))
	return finalKlines, nil
}

// getFromNetwork 从网络获取K线数据
func (km *KlineManager) getFromNetwork(ctx context.Context, symbol, timeframe string, limit int) ([]*binance.KlineData, error) {
	ctx, logger := log.WithCtx(ctx)
	logger.Debug("从网络获取K线数据", "symbol", symbol, "timeframe", timeframe, "limit", limit)

	return km.client.GetKlines(ctx, symbol, timeframe, limit)
}

// mergeKlines 合并K线数据，去重并按时间排序
func (km *KlineManager) mergeKlines(dbKlines, networkKlines []*binance.KlineData, lastTime int64) []*binance.KlineData {
	// 创建时间索引映射
	klineMap := make(map[int64]*binance.KlineData)

	// 添加数据库数据
	for _, kline := range dbKlines {
		klineMap[kline.OpenTime] = kline
	}

	// 添加网络数据（会覆盖相同时间的数据，确保最新）
	for _, kline := range networkKlines {
		if lastTime == 0 || kline.OpenTime > lastTime {
			klineMap[kline.OpenTime] = kline
		}
	}

	// 转换为切片并排序
	var result []*binance.KlineData
	for _, kline := range klineMap {
		result = append(result, kline)
	}

	// 按时间排序
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].OpenTime > result[j].OpenTime {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result
}

// filterNewKlines 过滤出新的K线数据
func (km *KlineManager) filterNewKlines(klines []*binance.KlineData, lastTime int64) []*binance.KlineData {
	var newKlines []*binance.KlineData
	for _, kline := range klines {
		if kline.OpenTime > lastTime {
			newKlines = append(newKlines, kline)
		}
	}
	return newKlines
}

// TimeRange 时间范围
type TimeRange struct {
	Start int64
	End   int64
}

// findMissingRanges 查找缺失的时间范围
func (km *KlineManager) findMissingRanges(klines []*binance.KlineData, startTime, endTime int64, timeframe string) []TimeRange {
	if len(klines) == 0 {
		return []TimeRange{{Start: startTime, End: endTime}}
	}

	// 计算时间间隔（毫秒）
	interval := km.getTimeframeInterval(timeframe)
	if interval == 0 {
		return nil
	}

	var missingRanges []TimeRange

	// 检查开始时间之前是否有缺失
	if klines[0].OpenTime > startTime {
		missingRanges = append(missingRanges, TimeRange{
			Start: startTime,
			End:   klines[0].OpenTime - interval,
		})
	}

	// 检查中间的缺失
	for i := 0; i < len(klines)-1; i++ {
		expectedNext := klines[i].OpenTime + interval
		actualNext := klines[i+1].OpenTime

		if actualNext > expectedNext {
			missingRanges = append(missingRanges, TimeRange{
				Start: expectedNext,
				End:   actualNext - interval,
			})
		}
	}

	// 检查结束时间之后是否有缺失
	lastKline := klines[len(klines)-1]
	if lastKline.OpenTime < endTime {
		missingRanges = append(missingRanges, TimeRange{
			Start: lastKline.OpenTime + interval,
			End:   endTime,
		})
	}

	return missingRanges
}

// getTimeframeInterval 获取时间周期对应的毫秒间隔
func (km *KlineManager) getTimeframeInterval(timeframe string) int64 {
	intervals := map[string]int64{
		"1s":  1000,
		"1m":  60 * 1000,
		"3m":  3 * 60 * 1000,
		"5m":  5 * 60 * 1000,
		"15m": 15 * 60 * 1000,
		"30m": 30 * 60 * 1000,
		"1h":  60 * 60 * 1000,
		"2h":  2 * 60 * 60 * 1000,
		"4h":  4 * 60 * 60 * 1000,
		"6h":  6 * 60 * 60 * 1000,
		"8h":  8 * 60 * 60 * 1000,
		"12h": 12 * 60 * 60 * 1000,
		"1d":  24 * 60 * 60 * 1000,
		"3d":  3 * 24 * 60 * 60 * 1000,
		"1w":  7 * 24 * 60 * 60 * 1000,
		"1M":  30 * 24 * 60 * 60 * 1000, // 近似值
	}

	return intervals[timeframe]
}
