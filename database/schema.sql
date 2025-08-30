-- 交易机器人数据库设计
-- PostgreSQL Schema

-- 创建数据库 (需要手动执行)
-- CREATE DATABASE tradingbot;
-- \c tradingbot;

-- 启用必要的扩展
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- 1. 交易对信息表
CREATE TABLE IF NOT EXISTS symbols (
    id SERIAL PRIMARY KEY,
    symbol VARCHAR(20) NOT NULL UNIQUE,
    base_asset VARCHAR(10) NOT NULL,
    quote_asset VARCHAR(10) NOT NULL,
    status VARCHAR(20) DEFAULT 'TRADING',
    min_qty DECIMAL(20,8),
    max_qty DECIMAL(20,8),
    step_size DECIMAL(20,8),
    min_price DECIMAL(20,8),
    max_price DECIMAL(20,8),
    tick_size DECIMAL(20,8),
    min_notional DECIMAL(20,8),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 2. K线数据表 (主表)
CREATE TABLE IF NOT EXISTS klines (
    id BIGSERIAL PRIMARY KEY,
    symbol VARCHAR(20) NOT NULL,
    timeframe VARCHAR(10) NOT NULL,
    open_time BIGINT NOT NULL,
    close_time BIGINT NOT NULL,
    open_price DECIMAL(20,8) NOT NULL,
    high_price DECIMAL(20,8) NOT NULL,
    low_price DECIMAL(20,8) NOT NULL,
    close_price DECIMAL(20,8) NOT NULL,
    volume DECIMAL(20,8) NOT NULL,
    quote_volume DECIMAL(20,8) NOT NULL,
    taker_buy_volume DECIMAL(20,8),
    taker_buy_quote_volume DECIMAL(20,8),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- 唯一约束：同一交易对、时间周期、开盘时间只能有一条记录
    UNIQUE(symbol, timeframe, open_time)
);

-- 3. 回测记录表
CREATE TABLE IF NOT EXISTS backtest_runs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100),
    symbol VARCHAR(20) NOT NULL,
    timeframe VARCHAR(10) NOT NULL,
    strategy_name VARCHAR(50) NOT NULL,
    strategy_params JSONB,
    start_time TIMESTAMP NOT NULL,
    end_time TIMESTAMP NOT NULL,
    initial_capital DECIMAL(20,2) NOT NULL,
    final_capital DECIMAL(20,2),
    total_return DECIMAL(10,4),
    max_drawdown DECIMAL(10,4),
    sharpe_ratio DECIMAL(10,4),
    win_rate DECIMAL(5,4),
    total_trades INTEGER,
    winning_trades INTEGER,
    losing_trades INTEGER,
    total_commission DECIMAL(20,8),
    status VARCHAR(20) DEFAULT 'RUNNING',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP
);

-- 4. 交易记录表
CREATE TABLE IF NOT EXISTS trades (
    id BIGSERIAL PRIMARY KEY,
    backtest_run_id UUID REFERENCES backtest_runs(id),
    symbol VARCHAR(20) NOT NULL,
    side VARCHAR(10) NOT NULL, -- 'BUY' or 'SELL'
    quantity DECIMAL(20,8) NOT NULL,
    price DECIMAL(20,8) NOT NULL,
    commission DECIMAL(20,8) DEFAULT 0,
    pnl DECIMAL(20,8),
    reason VARCHAR(100),
    timestamp TIMESTAMP NOT NULL,
    kline_open_time BIGINT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 5. 策略参数表 (用于存储不同策略的参数配置)
CREATE TABLE IF NOT EXISTS strategy_configs (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    strategy_type VARCHAR(50) NOT NULL,
    parameters JSONB NOT NULL,
    description TEXT,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 6. 数据同步状态表 (记录数据同步进度)
CREATE TABLE IF NOT EXISTS sync_status (
    id SERIAL PRIMARY KEY,
    symbol VARCHAR(20) NOT NULL,
    timeframe VARCHAR(10) NOT NULL,
    last_sync_time BIGINT,
    last_open_time BIGINT,
    total_records INTEGER DEFAULT 0,
    status VARCHAR(20) DEFAULT 'PENDING',
    error_message TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE(symbol, timeframe)
);

-- 创建索引优化查询性能
-- K线数据查询索引
CREATE INDEX IF NOT EXISTS idx_klines_symbol_timeframe ON klines(symbol, timeframe);
CREATE INDEX IF NOT EXISTS idx_klines_open_time ON klines(open_time);
CREATE INDEX IF NOT EXISTS idx_klines_symbol_timeframe_time ON klines(symbol, timeframe, open_time);
CREATE INDEX IF NOT EXISTS idx_klines_close_time ON klines(close_time);

-- 回测相关索引
CREATE INDEX IF NOT EXISTS idx_backtest_runs_symbol ON backtest_runs(symbol);
CREATE INDEX IF NOT EXISTS idx_backtest_runs_created_at ON backtest_runs(created_at);
CREATE INDEX IF NOT EXISTS idx_trades_backtest_run_id ON trades(backtest_run_id);
CREATE INDEX IF NOT EXISTS idx_trades_symbol_timestamp ON trades(symbol, timestamp);

-- 同步状态索引
CREATE INDEX IF NOT EXISTS idx_sync_status_symbol_timeframe ON sync_status(symbol, timeframe);

-- 创建更新时间触发器函数
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- 为需要的表创建更新时间触发器
CREATE TRIGGER update_symbols_updated_at BEFORE UPDATE ON symbols
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_klines_updated_at BEFORE UPDATE ON klines
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_strategy_configs_updated_at BEFORE UPDATE ON strategy_configs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_sync_status_updated_at BEFORE UPDATE ON sync_status
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- 插入一些常用的交易对
INSERT INTO symbols (symbol, base_asset, quote_asset) VALUES
('BTCUSDT', 'BTC', 'USDT'),
('ETHUSDT', 'ETH', 'USDT'),
('WIFUSDT', 'WIF', 'USDT'),
('SOLUSDT', 'SOL', 'USDT'),
('ADAUSDT', 'ADA', 'USDT')
ON CONFLICT (symbol) DO NOTHING;

-- 插入默认策略配置
INSERT INTO strategy_configs (name, strategy_type, parameters, description) VALUES
('默认布林道策略', 'bollinger_bands', '{
    "period": 20,
    "multiplier": 2.0,
    "position_size_percent": 0.95,
    "min_trade_amount": 10,
    "stop_loss_percent": 0.05,
    "take_profit_percent": 0.1,
    "cooldown_bars": 3
}', '默认的布林道交易策略配置'),
('保守布林道策略', 'bollinger_bands', '{
    "period": 20,
    "multiplier": 2.5,
    "position_size_percent": 0.5,
    "min_trade_amount": 10,
    "stop_loss_percent": 0.03,
    "take_profit_percent": 0.06,
    "cooldown_bars": 5
}', '更保守的布林道策略，降低风险')
ON CONFLICT DO NOTHING;

-- 创建视图：最新K线数据
CREATE OR REPLACE VIEW latest_klines AS
SELECT DISTINCT ON (symbol, timeframe) 
    symbol,
    timeframe,
    open_time,
    close_time,
    open_price,
    high_price,
    low_price,
    close_price,
    volume,
    quote_volume
FROM klines
ORDER BY symbol, timeframe, open_time DESC;

-- 创建视图：回测统计汇总
CREATE OR REPLACE VIEW backtest_summary AS
SELECT 
    br.id,
    br.name,
    br.symbol,
    br.timeframe,
    br.strategy_name,
    br.initial_capital,
    br.final_capital,
    br.total_return,
    br.max_drawdown,
    br.sharpe_ratio,
    br.win_rate,
    br.total_trades,
    COUNT(t.id) as actual_trades,
    SUM(CASE WHEN t.pnl > 0 THEN 1 ELSE 0 END) as profitable_trades,
    SUM(t.pnl) as total_pnl,
    br.created_at,
    br.completed_at
FROM backtest_runs br
LEFT JOIN trades t ON br.id = t.backtest_run_id
GROUP BY br.id, br.name, br.symbol, br.timeframe, br.strategy_name, 
         br.initial_capital, br.final_capital, br.total_return, 
         br.max_drawdown, br.sharpe_ratio, br.win_rate, br.total_trades,
         br.created_at, br.completed_at;

-- 创建函数：获取指定时间范围的K线数据
CREATE OR REPLACE FUNCTION get_klines(
    p_symbol VARCHAR(20),
    p_timeframe VARCHAR(10),
    p_start_time BIGINT DEFAULT NULL,
    p_end_time BIGINT DEFAULT NULL,
    p_limit INTEGER DEFAULT 1000
)
RETURNS TABLE (
    open_time BIGINT,
    close_time BIGINT,
    open_price DECIMAL(20,8),
    high_price DECIMAL(20,8),
    low_price DECIMAL(20,8),
    close_price DECIMAL(20,8),
    volume DECIMAL(20,8),
    quote_volume DECIMAL(20,8),
    taker_buy_volume DECIMAL(20,8),
    taker_buy_quote_volume DECIMAL(20,8)
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        k.open_time,
        k.close_time,
        k.open_price,
        k.high_price,
        k.low_price,
        k.close_price,
        k.volume,
        k.quote_volume,
        k.taker_buy_volume,
        k.taker_buy_quote_volume
    FROM klines k
    WHERE k.symbol = p_symbol 
      AND k.timeframe = p_timeframe
      AND (p_start_time IS NULL OR k.open_time >= p_start_time)
      AND (p_end_time IS NULL OR k.open_time <= p_end_time)
    ORDER BY k.open_time ASC
    LIMIT p_limit;
END;
$$ LANGUAGE plpgsql;

-- 插入常见交易对信息
INSERT INTO symbols (symbol, base_asset, quote_asset, status, min_qty, max_qty, step_size, min_price, max_price, tick_size, min_notional) VALUES
    ('WIFUSDT', 'WIF', 'USDT', 'TRADING', 0.00000001, 90000000000.00000000, 0.00000001, 0.00000001, 1000.00000000, 0.00000001, 5.00000000),
    ('WIFUSDC', 'WIF', 'USDC', 'TRADING', 0.00000001, 90000000000.00000000, 0.00000001, 0.00000001, 1000.00000000, 0.00000001, 5.00000000),
    ('BTCUSDT', 'BTC', 'USDT', 'TRADING', 0.00000100, 9000.00000000, 0.00000100, 0.01000000, 1000000.00000000, 0.01000000, 5.00000000),
    ('BTCUSDC', 'BTC', 'USDC', 'TRADING', 0.00000100, 9000.00000000, 0.00000100, 0.01000000, 1000000.00000000, 0.01000000, 5.00000000),
    ('ETHUSDT', 'ETH', 'USDT', 'TRADING', 0.00000100, 100000.00000000, 0.00000100, 0.01000000, 100000.00000000, 0.01000000, 5.00000000),
    ('ETHUSDC', 'ETH', 'USDC', 'TRADING', 0.00000100, 100000.00000000, 0.00000100, 0.01000000, 100000.00000000, 0.01000000, 5.00000000),
    ('SOLUSDT', 'SOL', 'USDT', 'TRADING', 0.00000100, 100000.00000000, 0.00000100, 0.00100000, 20000.00000000, 0.00100000, 5.00000000),
    ('SOLUSDC', 'SOL', 'USDC', 'TRADING', 0.00000100, 100000.00000000, 0.00000100, 0.00100000, 20000.00000000, 0.00100000, 5.00000000)
ON CONFLICT (symbol) DO UPDATE SET
    base_asset = EXCLUDED.base_asset,
    quote_asset = EXCLUDED.quote_asset,
    status = EXCLUDED.status,
    min_qty = EXCLUDED.min_qty,
    max_qty = EXCLUDED.max_qty,
    step_size = EXCLUDED.step_size,
    min_price = EXCLUDED.min_price,
    max_price = EXCLUDED.max_price,
    tick_size = EXCLUDED.tick_size,
    min_notional = EXCLUDED.min_notional,
    updated_at = CURRENT_TIMESTAMP;

-- 索引优化
-- K线数据查询索引
CREATE INDEX IF NOT EXISTS idx_klines_symbol_timeframe_time 
ON klines (symbol, timeframe, open_time);

-- K线数据时间范围查询索引
CREATE INDEX IF NOT EXISTS idx_klines_symbol_timeframe 
ON klines (symbol, timeframe);

-- 回测记录查询索引
CREATE INDEX IF NOT EXISTS idx_backtest_runs_symbol_strategy 
ON backtest_runs (symbol, strategy_name, created_at);

-- 交易记录查询索引
CREATE INDEX IF NOT EXISTS idx_trades_backtest_time 
ON trades (backtest_run_id, executed_at);
