package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"go-build-stream-gateway-go-server-main/src/binance"

	"github.com/lib/pq"
	_ "github.com/lib/pq"
	"github.com/shopspring/decimal"
)

// PostgresDB PostgreSQL数据库连接
type PostgresDB struct {
	db *sql.DB
}

// KlineRecord K线数据记录
type KlineRecord struct {
	ID                  int64           `json:"id"`
	Symbol              string          `json:"symbol"`
	Timeframe           string          `json:"timeframe"`
	OpenTime            int64           `json:"open_time"`
	CloseTime           int64           `json:"close_time"`
	OpenPrice           decimal.Decimal `json:"open_price"`
	HighPrice           decimal.Decimal `json:"high_price"`
	LowPrice            decimal.Decimal `json:"low_price"`
	ClosePrice          decimal.Decimal `json:"close_price"`
	Volume              decimal.Decimal `json:"volume"`
	QuoteVolume         decimal.Decimal `json:"quote_volume"`
	TakerBuyVolume      decimal.Decimal `json:"taker_buy_volume"`
	TakerBuyQuoteVolume decimal.Decimal `json:"taker_buy_quote_volume"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
}

// BacktestRun 回测运行记录
type BacktestRun struct {
	ID              string                 `json:"id"`
	Name            string                 `json:"name"`
	Symbol          string                 `json:"symbol"`
	Timeframe       string                 `json:"timeframe"`
	StrategyName    string                 `json:"strategy_name"`
	StrategyParams  map[string]interface{} `json:"strategy_params"`
	StartTime       time.Time              `json:"start_time"`
	EndTime         time.Time              `json:"end_time"`
	InitialCapital  decimal.Decimal        `json:"initial_capital"`
	FinalCapital    decimal.Decimal        `json:"final_capital"`
	TotalReturn     decimal.Decimal        `json:"total_return"`
	MaxDrawdown     decimal.Decimal        `json:"max_drawdown"`
	SharpeRatio     decimal.Decimal        `json:"sharpe_ratio"`
	WinRate         decimal.Decimal        `json:"win_rate"`
	TotalTrades     int                    `json:"total_trades"`
	WinningTrades   int                    `json:"winning_trades"`
	LosingTrades    int                    `json:"losing_trades"`
	TotalCommission decimal.Decimal        `json:"total_commission"`
	Status          string                 `json:"status"`
	CreatedAt       time.Time              `json:"created_at"`
	CompletedAt     *time.Time             `json:"completed_at"`
}

// TradeRecord 交易记录
type TradeRecord struct {
	ID            int64           `json:"id"`
	BacktestRunID string          `json:"backtest_run_id"`
	Symbol        string          `json:"symbol"`
	Side          string          `json:"side"`
	Quantity      decimal.Decimal `json:"quantity"`
	Price         decimal.Decimal `json:"price"`
	Commission    decimal.Decimal `json:"commission"`
	PnL           decimal.Decimal `json:"pnl"`
	Reason        string          `json:"reason"`
	Timestamp     time.Time       `json:"timestamp"`
	KlineOpenTime int64           `json:"kline_open_time"`
	CreatedAt     time.Time       `json:"created_at"`
}

// SyncStatus 数据同步状态
type SyncStatus struct {
	ID           int       `json:"id"`
	Symbol       string    `json:"symbol"`
	Timeframe    string    `json:"timeframe"`
	LastSyncTime int64     `json:"last_sync_time"`
	LastOpenTime int64     `json:"last_open_time"`
	TotalRecords int       `json:"total_records"`
	Status       string    `json:"status"`
	ErrorMessage string    `json:"error_message"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// NewPostgresDB 创建PostgreSQL数据库连接
func NewPostgresDB(host, port, user, password, dbname string, sslmode string) (*PostgresDB, error) {
	if sslmode == "" {
		sslmode = "disable"
	}

	psqlInfo := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode)

	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 测试连接
	err = db.Ping()
	if err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// 设置连接池参数
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	return &PostgresDB{db: db}, nil
}

// Close 关闭数据库连接
func (p *PostgresDB) Close() error {
	return p.db.Close()
}

// SaveKlines 批量保存K线数据
func (p *PostgresDB) SaveKlines(ctx context.Context, symbol, timeframe string, klines []*binance.KlineData) error {
	if len(klines) == 0 {
		return nil
	}

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO klines (
			symbol, timeframe, open_time, close_time,
			open_price, high_price, low_price, close_price,
			volume, quote_volume, taker_buy_volume, taker_buy_quote_volume
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (symbol, timeframe, open_time) 
		DO UPDATE SET
			close_time = EXCLUDED.close_time,
			open_price = EXCLUDED.open_price,
			high_price = EXCLUDED.high_price,
			low_price = EXCLUDED.low_price,
			close_price = EXCLUDED.close_price,
			volume = EXCLUDED.volume,
			quote_volume = EXCLUDED.quote_volume,
			taker_buy_volume = EXCLUDED.taker_buy_volume,
			taker_buy_quote_volume = EXCLUDED.taker_buy_quote_volume,
			updated_at = CURRENT_TIMESTAMP
		WHERE (
			klines.close_time != EXCLUDED.close_time OR
			klines.open_price != EXCLUDED.open_price OR
			klines.high_price != EXCLUDED.high_price OR
			klines.low_price != EXCLUDED.low_price OR
			klines.close_price != EXCLUDED.close_price OR
			klines.volume != EXCLUDED.volume OR
			klines.quote_volume != EXCLUDED.quote_volume OR
			klines.taker_buy_volume IS DISTINCT FROM EXCLUDED.taker_buy_volume OR
			klines.taker_buy_quote_volume IS DISTINCT FROM EXCLUDED.taker_buy_quote_volume
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, kline := range klines {
		_, err = stmt.ExecContext(ctx,
			symbol, timeframe, kline.OpenTime, kline.CloseTime,
			kline.Open, kline.High, kline.Low, kline.Close,
			kline.Volume, kline.QuoteVolume, kline.TakerBuyVolume, kline.TakerBuyQuoteVolume,
		)
		if err != nil {
			return fmt.Errorf("failed to insert kline: %w", err)
		}
	}

	return tx.Commit()
}

// SaveKlinesBatch 批量保存K线数据（高性能版本）
func (p *PostgresDB) SaveKlinesBatch(ctx context.Context, symbol, timeframe string, klines []*binance.KlineData) error {
	if len(klines) == 0 {
		return nil
	}

	// 分批处理，避免SQL语句过长
	const batchSize = 100
	for i := 0; i < len(klines); i += batchSize {
		end := i + batchSize
		if end > len(klines) {
			end = len(klines)
		}

		if err := p.saveBatch(ctx, symbol, timeframe, klines[i:end]); err != nil {
			return err
		}
	}

	return nil
}

// saveBatch 保存一批K线数据
func (p *PostgresDB) saveBatch(ctx context.Context, symbol, timeframe string, klines []*binance.KlineData) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 构建批量插入的VALUES子句
	valueStrings := make([]string, 0, len(klines))
	valueArgs := make([]interface{}, 0, len(klines)*12)

	for i, kline := range klines {
		valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
			i*12+1, i*12+2, i*12+3, i*12+4, i*12+5, i*12+6, i*12+7, i*12+8, i*12+9, i*12+10, i*12+11, i*12+12))

		valueArgs = append(valueArgs,
			symbol, timeframe, kline.OpenTime, kline.CloseTime,
			kline.Open, kline.High, kline.Low, kline.Close,
			kline.Volume, kline.QuoteVolume, kline.TakerBuyVolume, kline.TakerBuyQuoteVolume,
		)
	}

	query := `
		INSERT INTO klines (
			symbol, timeframe, open_time, close_time,
			open_price, high_price, low_price, close_price,
			volume, quote_volume, taker_buy_volume, taker_buy_quote_volume
		) VALUES ` + strings.Join(valueStrings, ",") + `
		ON CONFLICT (symbol, timeframe, open_time) 
		DO UPDATE SET
			close_time = EXCLUDED.close_time,
			open_price = EXCLUDED.open_price,
			high_price = EXCLUDED.high_price,
			low_price = EXCLUDED.low_price,
			close_price = EXCLUDED.close_price,
			volume = EXCLUDED.volume,
			quote_volume = EXCLUDED.quote_volume,
			taker_buy_volume = EXCLUDED.taker_buy_volume,
			taker_buy_quote_volume = EXCLUDED.taker_buy_quote_volume,
			updated_at = CURRENT_TIMESTAMP
		WHERE (
			klines.close_time != EXCLUDED.close_time OR
			klines.open_price != EXCLUDED.open_price OR
			klines.high_price != EXCLUDED.high_price OR
			klines.low_price != EXCLUDED.low_price OR
			klines.close_price != EXCLUDED.close_price OR
			klines.volume != EXCLUDED.volume OR
			klines.quote_volume != EXCLUDED.quote_volume OR
			klines.taker_buy_volume IS DISTINCT FROM EXCLUDED.taker_buy_volume OR
			klines.taker_buy_quote_volume IS DISTINCT FROM EXCLUDED.taker_buy_quote_volume
		)
	`

	_, err = tx.ExecContext(ctx, query, valueArgs...)
	if err != nil {
		return fmt.Errorf("failed to batch insert klines: %w", err)
	}

	return tx.Commit()
}

// GetKlines 获取K线数据
func (p *PostgresDB) GetKlines(ctx context.Context, symbol, timeframe string, startTime, endTime int64, limit int) ([]*binance.KlineData, error) {
	query := `
		SELECT open_time, close_time, open_price, high_price, low_price, close_price,
		       volume, quote_volume, taker_buy_volume, taker_buy_quote_volume
		FROM klines
		WHERE symbol = $1 AND timeframe = $2
	`
	args := []interface{}{symbol, timeframe}
	argIndex := 3

	if startTime > 0 {
		query += fmt.Sprintf(" AND open_time >= $%d", argIndex)
		args = append(args, startTime)
		argIndex++
	}

	if endTime > 0 {
		query += fmt.Sprintf(" AND open_time <= $%d", argIndex)
		args = append(args, endTime)
		argIndex++
	}

	query += " ORDER BY open_time ASC"

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, limit)
	}

	rows, err := p.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query klines: %w", err)
	}
	defer rows.Close()

	var klines []*binance.KlineData
	for rows.Next() {
		kline := &binance.KlineData{Symbol: symbol}
		err := rows.Scan(
			&kline.OpenTime, &kline.CloseTime,
			&kline.Open, &kline.High, &kline.Low, &kline.Close,
			&kline.Volume, &kline.QuoteVolume,
			&kline.TakerBuyVolume, &kline.TakerBuyQuoteVolume,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan kline: %w", err)
		}
		klines = append(klines, kline)
	}

	return klines, rows.Err()
}

// GetLatestKlineTime 获取最新K线时间
func (p *PostgresDB) GetLatestKlineTime(ctx context.Context, symbol, timeframe string) (int64, error) {
	var openTime sql.NullInt64
	err := p.db.QueryRowContext(ctx,
		"SELECT MAX(open_time) FROM klines WHERE symbol = $1 AND timeframe = $2",
		symbol, timeframe,
	).Scan(&openTime)

	if err != nil {
		return 0, fmt.Errorf("failed to get latest kline time: %w", err)
	}

	if !openTime.Valid {
		return 0, nil
	}

	return openTime.Int64, nil
}

// SaveBacktestRun 保存回测运行记录
func (p *PostgresDB) SaveBacktestRun(ctx context.Context, run *BacktestRun) error {
	query := `
		INSERT INTO backtest_runs (
			id, name, symbol, timeframe, strategy_name, strategy_params,
			start_time, end_time, initial_capital, final_capital,
			total_return, max_drawdown, sharpe_ratio, win_rate,
			total_trades, winning_trades, losing_trades, total_commission,
			status, completed_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16, $17, $18, $19, $20
		)
	`

	paramsJSON, err := pq.Array(run.StrategyParams).Value()
	if err != nil {
		return fmt.Errorf("failed to marshal strategy params: %w", err)
	}

	_, err = p.db.ExecContext(ctx, query,
		run.ID, run.Name, run.Symbol, run.Timeframe, run.StrategyName, paramsJSON,
		run.StartTime, run.EndTime, run.InitialCapital, run.FinalCapital,
		run.TotalReturn, run.MaxDrawdown, run.SharpeRatio, run.WinRate,
		run.TotalTrades, run.WinningTrades, run.LosingTrades, run.TotalCommission,
		run.Status, run.CompletedAt,
	)

	return err
}

// SaveTrades 批量保存交易记录
func (p *PostgresDB) SaveTrades(ctx context.Context, trades []*TradeRecord) error {
	if len(trades) == 0 {
		return nil
	}

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO trades (
			backtest_run_id, symbol, side, quantity, price,
			commission, pnl, reason, timestamp, kline_open_time
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, trade := range trades {
		_, err = stmt.ExecContext(ctx,
			trade.BacktestRunID, trade.Symbol, trade.Side, trade.Quantity, trade.Price,
			trade.Commission, trade.PnL, trade.Reason, trade.Timestamp, trade.KlineOpenTime,
		)
		if err != nil {
			return fmt.Errorf("failed to insert trade: %w", err)
		}
	}

	return tx.Commit()
}

// UpdateSyncStatus 更新同步状态
func (p *PostgresDB) UpdateSyncStatus(ctx context.Context, symbol, timeframe string, lastOpenTime int64, totalRecords int, status, errorMsg string) error {
	query := `
		INSERT INTO sync_status (symbol, timeframe, last_sync_time, last_open_time, total_records, status, error_message)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (symbol, timeframe)
		DO UPDATE SET
			last_sync_time = $3,
			last_open_time = $4,
			total_records = $5,
			status = $6,
			error_message = $7,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := p.db.ExecContext(ctx, query,
		symbol, timeframe, time.Now().Unix(), lastOpenTime, totalRecords, status, errorMsg,
	)

	return err
}

// GetSyncStatus 获取同步状态
func (p *PostgresDB) GetSyncStatus(ctx context.Context, symbol, timeframe string) (*SyncStatus, error) {
	var status SyncStatus
	err := p.db.QueryRowContext(ctx,
		"SELECT id, symbol, timeframe, last_sync_time, last_open_time, total_records, status, error_message, created_at, updated_at FROM sync_status WHERE symbol = $1 AND timeframe = $2",
		symbol, timeframe,
	).Scan(
		&status.ID, &status.Symbol, &status.Timeframe, &status.LastSyncTime,
		&status.LastOpenTime, &status.TotalRecords, &status.Status,
		&status.ErrorMessage, &status.CreatedAt, &status.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get sync status: %w", err)
	}

	return &status, nil
}

// SymbolInfo 交易对信息结构体
type SymbolInfo struct {
	ID          int             `json:"id"`
	Symbol      string          `json:"symbol"`
	BaseAsset   string          `json:"base_asset"`
	QuoteAsset  string          `json:"quote_asset"`
	Status      string          `json:"status"`
	MinQty      decimal.Decimal `json:"min_qty"`
	MaxQty      decimal.Decimal `json:"max_qty"`
	StepSize    decimal.Decimal `json:"step_size"`
	MinPrice    decimal.Decimal `json:"min_price"`
	MaxPrice    decimal.Decimal `json:"max_price"`
	TickSize    decimal.Decimal `json:"tick_size"`
	MinNotional decimal.Decimal `json:"min_notional"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// GetSymbolInfo 获取交易对信息
func (db *PostgresDB) GetSymbolInfo(symbol string) (*SymbolInfo, error) {
	query := `
		SELECT id, symbol, base_asset, quote_asset, status, 
		       min_qty, max_qty, step_size, min_price, max_price, 
		       tick_size, min_notional, created_at, updated_at
		FROM symbols 
		WHERE symbol = $1 AND status = 'TRADING'
	`

	var info SymbolInfo
	err := db.db.QueryRow(query, symbol).Scan(
		&info.ID, &info.Symbol, &info.BaseAsset, &info.QuoteAsset, &info.Status,
		&info.MinQty, &info.MaxQty, &info.StepSize, &info.MinPrice, &info.MaxPrice,
		&info.TickSize, &info.MinNotional, &info.CreatedAt, &info.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("trading pair %s is not supported or not active", symbol)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get symbol info: %w", err)
	}

	return &info, nil
}

// IsSymbolSupported 检查交易对是否支持
func (db *PostgresDB) IsSymbolSupported(symbol string) (bool, error) {
	query := `SELECT COUNT(*) FROM symbols WHERE symbol = $1 AND status = 'TRADING'`

	var count int
	err := db.db.QueryRow(query, symbol).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check symbol support: %w", err)
	}

	return count > 0, nil
}

// GetSupportedSymbols 获取所有支持的交易对
func (db *PostgresDB) GetSupportedSymbols() ([]string, error) {
	query := `SELECT symbol FROM symbols WHERE status = 'TRADING' ORDER BY symbol`

	rows, err := db.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get supported symbols: %w", err)
	}
	defer rows.Close()

	var symbols []string
	for rows.Next() {
		var symbol string
		if err := rows.Scan(&symbol); err != nil {
			return nil, fmt.Errorf("failed to scan symbol: %w", err)
		}
		symbols = append(symbols, symbol)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate symbols: %w", err)
	}

	return symbols, nil
}
