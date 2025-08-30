package database

import (
	"context"
	"database/sql"
	"testing"

	"go-build-stream-gateway-go-server-main/src/binance"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostgresDB_SaveKlines_Simple(t *testing.T) {
	// 使用 sqlmock 模拟数据库
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	postgresDB := &PostgresDB{db: db}

	// 测试数据
	klines := []*binance.KlineData{
		{
			OpenTime:            1640995200000, // 2022-01-01 00:00:00
			CloseTime:           1640998800000, // 2022-01-01 01:00:00
			Open:                decimal.NewFromFloat(50000),
			High:                decimal.NewFromFloat(51000),
			Low:                 decimal.NewFromFloat(49000),
			Close:               decimal.NewFromFloat(50500),
			Volume:              decimal.NewFromFloat(100),
			QuoteVolume:         decimal.NewFromFloat(5050000),
			TakerBuyVolume:      decimal.NewFromFloat(60),
			TakerBuyQuoteVolume: decimal.NewFromFloat(3030000),
		},
	}

	t.Run("successful save", func(t *testing.T) {
		// 设置期望的SQL调用
		mock.ExpectBegin()
		mock.ExpectPrepare("INSERT INTO klines").ExpectExec().
			WithArgs("BTCUSDT", "1h", int64(1640995200000), int64(1640998800000),
				klines[0].Open, klines[0].High, klines[0].Low, klines[0].Close,
				klines[0].Volume, klines[0].QuoteVolume, klines[0].TakerBuyVolume, klines[0].TakerBuyQuoteVolume).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		err := postgresDB.SaveKlines(context.Background(), "BTCUSDT", "1h", klines)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("empty klines", func(t *testing.T) {
		err := postgresDB.SaveKlines(context.Background(), "BTCUSDT", "1h", []*binance.KlineData{})
		assert.NoError(t, err)
	})

	t.Run("database error", func(t *testing.T) {
		mock.ExpectBegin().WillReturnError(sql.ErrConnDone)

		err := postgresDB.SaveKlines(context.Background(), "BTCUSDT", "1h", klines)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to begin transaction")
	})
}

func TestPostgresDB_GetKlines_Simple(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	postgresDB := &PostgresDB{db: db}

	t.Run("successful get", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{
			"open_time", "close_time", "open_price", "high_price", "low_price", "close_price",
			"volume", "quote_volume", "taker_buy_volume", "taker_buy_quote_volume",
		}).AddRow(
			int64(1640995200000), int64(1640998800000),
			"50000.00000000", "51000.00000000", "49000.00000000", "50500.00000000",
			"100.00000000", "5050000.00000000", "60.00000000", "3030000.00000000",
		)

		// GetKlines with limit only (startTime=0, endTime=0, limit=10)
		mock.ExpectQuery("SELECT (.+) FROM klines").
			WithArgs("BTCUSDT", "1h", 10).
			WillReturnRows(rows)

		klines, err := postgresDB.GetKlines(context.Background(), "BTCUSDT", "1h", 0, 0, 10)

		assert.NoError(t, err)
		assert.Len(t, klines, 1)
		assert.Equal(t, int64(1640995200000), klines[0].OpenTime)
		// 使用字符串创建decimal以匹配数据库精度
		expectedOpen, _ := decimal.NewFromString("50000.00000000")
		assert.True(t, klines[0].Open.Equal(expectedOpen))
	})

	t.Run("no data found", func(t *testing.T) {
		mock.ExpectQuery("SELECT (.+) FROM klines").
			WithArgs("BTCUSDT", "1h", 10).
			WillReturnRows(sqlmock.NewRows([]string{"open_time"}))

		klines, err := postgresDB.GetKlines(context.Background(), "BTCUSDT", "1h", 0, 0, 10)

		assert.NoError(t, err)
		assert.Len(t, klines, 0)
	})
}

func TestPostgresDB_GetLatestKlineTime_Simple(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	postgresDB := &PostgresDB{db: db}

	t.Run("has data", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"max"}).AddRow(int64(1640998800000))

		mock.ExpectQuery("SELECT MAX\\(open_time\\) FROM klines").
			WithArgs("BTCUSDT", "1h").
			WillReturnRows(rows)

		latestTime, err := postgresDB.GetLatestKlineTime(context.Background(), "BTCUSDT", "1h")

		assert.NoError(t, err)
		assert.Equal(t, int64(1640998800000), latestTime)
	})

	t.Run("no data", func(t *testing.T) {
		mock.ExpectQuery("SELECT MAX\\(open_time\\) FROM klines").
			WithArgs("BTCUSDT", "1h").
			WillReturnError(sql.ErrNoRows)

		latestTime, err := postgresDB.GetLatestKlineTime(context.Background(), "BTCUSDT", "1h")

		assert.Error(t, err) // 应该返回错误
		assert.Equal(t, int64(0), latestTime)
	})
}

// PostgresDB没有Ping方法，跳过此测试

func TestPostgresDB_Close_Simple(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	postgresDB := &PostgresDB{db: db}

	mock.ExpectClose()

	err = postgresDB.Close()
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// 基准测试
func BenchmarkPostgresDB_SaveKlines_Simple(b *testing.B) {
	db, mock, err := sqlmock.New()
	require.NoError(b, err)
	defer db.Close()

	postgresDB := &PostgresDB{db: db}

	klines := []*binance.KlineData{
		{
			OpenTime:            1640995200000,
			CloseTime:           1640998800000,
			Open:                decimal.NewFromFloat(50000),
			High:                decimal.NewFromFloat(51000),
			Low:                 decimal.NewFromFloat(49000),
			Close:               decimal.NewFromFloat(50500),
			Volume:              decimal.NewFromFloat(100),
			QuoteVolume:         decimal.NewFromFloat(5050000),
			TakerBuyVolume:      decimal.NewFromFloat(60),
			TakerBuyQuoteVolume: decimal.NewFromFloat(3030000),
		},
	}

	// 设置期望调用
	for i := 0; i < b.N; i++ {
		mock.ExpectBegin()
		mock.ExpectPrepare("INSERT INTO klines").ExpectExec().
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		postgresDB.SaveKlines(context.Background(), "BTCUSDT", "1h", klines)
	}
}
