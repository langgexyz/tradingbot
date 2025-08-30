-- 添加交易对表，记录完整的开仓平仓信息

-- 1. 创建交易对表（一笔完整交易：开仓+平仓）
CREATE TABLE IF NOT EXISTS trade_pairs (
    id BIGSERIAL PRIMARY KEY,
    backtest_run_id UUID REFERENCES backtest_runs(id),
    symbol VARCHAR(20) NOT NULL,
    
    -- 开仓信息
    buy_order_id BIGINT,
    buy_time TIMESTAMP NOT NULL,
    buy_price DECIMAL(40,18) NOT NULL,
    buy_quantity DECIMAL(40,18) NOT NULL,
    buy_amount DECIMAL(40,18) NOT NULL,
    buy_commission DECIMAL(40,18) DEFAULT 0,
    buy_reason VARCHAR(200),
    
    -- 平仓信息
    sell_order_id BIGINT,
    sell_time TIMESTAMP,
    sell_price DECIMAL(40,18),
    sell_quantity DECIMAL(40,18),
    sell_amount DECIMAL(40,18),
    sell_commission DECIMAL(40,18) DEFAULT 0,
    sell_reason VARCHAR(200),
    
    -- 盈亏分析
    pnl DECIMAL(40,18),
    pnl_percent DECIMAL(10,4),
    total_commission DECIMAL(40,18),
    net_profit DECIMAL(40,18),
    
    -- 持仓时间
    holding_duration INTERVAL,
    holding_days DECIMAL(10,2),
    
    -- 状态
    status VARCHAR(20) DEFAULT 'OPEN', -- 'OPEN', 'CLOSED'
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 2. 修改现有的 trades 表，添加 trade_pair_id 关联
ALTER TABLE trades ADD COLUMN IF NOT EXISTS trade_pair_id BIGINT REFERENCES trade_pairs(id);

-- 3. 创建索引
CREATE INDEX IF NOT EXISTS idx_trade_pairs_backtest_run_id ON trade_pairs(backtest_run_id);
CREATE INDEX IF NOT EXISTS idx_trade_pairs_symbol ON trade_pairs(symbol);
CREATE INDEX IF NOT EXISTS idx_trade_pairs_status ON trade_pairs(status);
CREATE INDEX IF NOT EXISTS idx_trade_pairs_buy_time ON trade_pairs(buy_time);
CREATE INDEX IF NOT EXISTS idx_trades_trade_pair_id ON trades(trade_pair_id);

-- 4. 创建更新触发器
CREATE TRIGGER update_trade_pairs_updated_at BEFORE UPDATE ON trade_pairs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- 5. 创建视图：完整交易记录
CREATE OR REPLACE VIEW complete_trades AS
SELECT 
    tp.id,
    tp.backtest_run_id,
    tp.symbol,
    tp.buy_time,
    tp.buy_price,
    tp.buy_quantity,
    tp.buy_amount,
    tp.buy_commission,
    tp.buy_reason,
    tp.sell_time,
    tp.sell_price,
    tp.sell_quantity,
    tp.sell_amount,
    tp.sell_commission,
    tp.sell_reason,
    tp.pnl,
    tp.pnl_percent,
    tp.total_commission,
    tp.net_profit,
    tp.holding_duration,
    tp.holding_days,
    tp.status,
    CASE 
        WHEN tp.status = 'CLOSED' THEN 
            CASE WHEN tp.net_profit > 0 THEN 'PROFIT' ELSE 'LOSS' END
        ELSE 'OPEN'
    END as trade_result
FROM trade_pairs tp
ORDER BY tp.buy_time DESC;

-- 6. 创建函数：记录开仓
CREATE OR REPLACE FUNCTION record_buy_order(
    p_backtest_run_id UUID,
    p_symbol VARCHAR(20),
    p_buy_time TIMESTAMP,
    p_buy_price DECIMAL(40,18),
    p_buy_quantity DECIMAL(40,18),
    p_buy_commission DECIMAL(40,18),
    p_buy_reason VARCHAR(200)
)
RETURNS BIGINT AS $$
DECLARE
    trade_pair_id BIGINT;
    buy_amount DECIMAL(40,18);
BEGIN
    -- 计算买入金额
    buy_amount := p_buy_price * p_buy_quantity;
    
    -- 插入交易对记录
    INSERT INTO trade_pairs (
        backtest_run_id, symbol, buy_time, buy_price, buy_quantity, 
        buy_amount, buy_commission, buy_reason, status
    ) VALUES (
        p_backtest_run_id, p_symbol, p_buy_time, p_buy_price, p_buy_quantity,
        buy_amount, p_buy_commission, p_buy_reason, 'OPEN'
    ) RETURNING id INTO trade_pair_id;
    
    RETURN trade_pair_id;
END;
$$ LANGUAGE plpgsql;

-- 7. 创建函数：记录平仓
CREATE OR REPLACE FUNCTION record_sell_order(
    p_trade_pair_id BIGINT,
    p_sell_time TIMESTAMP,
    p_sell_price DECIMAL(40,18),
    p_sell_commission DECIMAL(40,18),
    p_sell_reason VARCHAR(200)
)
RETURNS VOID AS $$
DECLARE
    tp_record trade_pairs%ROWTYPE;
    sell_amount DECIMAL(40,18);
    pnl DECIMAL(40,18);
    pnl_percent DECIMAL(10,4);
    total_commission DECIMAL(40,18);
    net_profit DECIMAL(40,18);
    duration INTERVAL;
    days DECIMAL(10,2);
BEGIN
    -- 获取交易对记录
    SELECT * INTO tp_record FROM trade_pairs WHERE id = p_trade_pair_id;
    
    IF NOT FOUND THEN
        RAISE EXCEPTION 'Trade pair not found: %', p_trade_pair_id;
    END IF;
    
    -- 计算相关数据
    sell_amount := p_sell_price * tp_record.buy_quantity;
    pnl := sell_amount - tp_record.buy_amount;
    pnl_percent := (pnl / tp_record.buy_amount) * 100;
    total_commission := tp_record.buy_commission + p_sell_commission;
    net_profit := pnl - total_commission;
    duration := p_sell_time - tp_record.buy_time;
    days := EXTRACT(EPOCH FROM duration) / 86400.0;
    
    -- 更新交易对记录
    UPDATE trade_pairs SET
        sell_time = p_sell_time,
        sell_price = p_sell_price,
        sell_quantity = tp_record.buy_quantity,
        sell_amount = sell_amount,
        sell_commission = p_sell_commission,
        sell_reason = p_sell_reason,
        pnl = pnl,
        pnl_percent = pnl_percent,
        total_commission = total_commission,
        net_profit = net_profit,
        holding_duration = duration,
        holding_days = days,
        status = 'CLOSED',
        updated_at = CURRENT_TIMESTAMP
    WHERE id = p_trade_pair_id;
END;
$$ LANGUAGE plpgsql;
