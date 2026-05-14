-- Go Radar PostgreSQL 建表脚本
-- 命名规范：
-- t_<业务域>_<最终业务表名>
-- 例如：t_radar_signal_event

CREATE TABLE IF NOT EXISTS t_radar_token (
  id BIGSERIAL PRIMARY KEY,
  chain VARCHAR(64) NOT NULL,
  address VARCHAR(255) NOT NULL,
  symbol VARCHAR(64) NOT NULL DEFAULT '',
  name VARCHAR(255) NOT NULL DEFAULT '',
  narrative_theme VARCHAR(128) NOT NULL DEFAULT '',
  narrative_tags_json TEXT NOT NULL DEFAULT '[]',
  social_links_json TEXT NOT NULL DEFAULT '{}',
  description TEXT NOT NULL DEFAULT '',
  first_seen_at VARCHAR(40) NOT NULL DEFAULT '',
  last_seen_at VARCHAR(40) NOT NULL DEFAULT ''
);
COMMENT ON TABLE t_radar_token IS '雷达标的主数据表';
COMMENT ON COLUMN t_radar_token.id IS '主键ID';
COMMENT ON COLUMN t_radar_token.chain IS '链或市场标识';
COMMENT ON COLUMN t_radar_token.address IS '标准化后的地址或合约标识';
COMMENT ON COLUMN t_radar_token.symbol IS '展示用符号';
COMMENT ON COLUMN t_radar_token.name IS '展示用名称';
COMMENT ON COLUMN t_radar_token.narrative_theme IS '叙事主题';
COMMENT ON COLUMN t_radar_token.narrative_tags_json IS '叙事标签JSON';
COMMENT ON COLUMN t_radar_token.social_links_json IS '社交链接JSON';
COMMENT ON COLUMN t_radar_token.description IS '标的描述';
COMMENT ON COLUMN t_radar_token.first_seen_at IS '首次发现时间RFC3339Nano';
COMMENT ON COLUMN t_radar_token.last_seen_at IS '最近发现时间RFC3339Nano';
CREATE UNIQUE INDEX IF NOT EXISTS uk_radar_token_chain_address ON t_radar_token(chain, address);

CREATE TABLE IF NOT EXISTS t_radar_market_snapshot (
  id BIGSERIAL PRIMARY KEY,
  token_id BIGINT NOT NULL REFERENCES t_radar_token(id),
  source VARCHAR(32) NOT NULL,
  price DOUBLE PRECISION NULL,
  mc DOUBLE PRECISION NULL,
  liq DOUBLE PRECISION NULL,
  volume DOUBLE PRECISION NULL,
  holders BIGINT NULL,
  smart_money BIGINT NULL,
  funding_pct DOUBLE PRECISION NULL,
  oi_usd DOUBLE PRECISION NULL,
  oi_d6h DOUBLE PRECISION NULL,
  buys_1h BIGINT NULL,
  sells_1h BIGINT NULL,
  age_h DOUBLE PRECISION NULL,
  raw_json TEXT NOT NULL DEFAULT '{}',
  created_at VARCHAR(40) NOT NULL DEFAULT ''
);
COMMENT ON TABLE t_radar_market_snapshot IS '雷达市场快照表';
COMMENT ON COLUMN t_radar_market_snapshot.token_id IS '标的主键ID';
COMMENT ON COLUMN t_radar_market_snapshot.source IS '扫描器来源';
COMMENT ON COLUMN t_radar_market_snapshot.price IS '价格';
COMMENT ON COLUMN t_radar_market_snapshot.mc IS '市值';
COMMENT ON COLUMN t_radar_market_snapshot.liq IS '流动性';
COMMENT ON COLUMN t_radar_market_snapshot.volume IS '成交量';
COMMENT ON COLUMN t_radar_market_snapshot.holders IS '持有人数';
COMMENT ON COLUMN t_radar_market_snapshot.smart_money IS '聪明钱数量';
COMMENT ON COLUMN t_radar_market_snapshot.funding_pct IS '资金费率百分比';
COMMENT ON COLUMN t_radar_market_snapshot.oi_usd IS '未平仓合约美元价值';
COMMENT ON COLUMN t_radar_market_snapshot.oi_d6h IS '六小时OI变化';
COMMENT ON COLUMN t_radar_market_snapshot.buys_1h IS '一小时买入次数';
COMMENT ON COLUMN t_radar_market_snapshot.sells_1h IS '一小时卖出次数';
COMMENT ON COLUMN t_radar_market_snapshot.age_h IS '标的年龄小时数';
COMMENT ON COLUMN t_radar_market_snapshot.raw_json IS '原始快照JSON';
COMMENT ON COLUMN t_radar_market_snapshot.created_at IS '快照创建时间RFC3339Nano';
CREATE INDEX IF NOT EXISTS idx_radar_market_snapshot_token_id ON t_radar_market_snapshot(token_id);
CREATE INDEX IF NOT EXISTS idx_radar_market_snapshot_source ON t_radar_market_snapshot(source);
CREATE INDEX IF NOT EXISTS idx_radar_market_snapshot_created_at ON t_radar_market_snapshot(created_at);

CREATE TABLE IF NOT EXISTS t_radar_signal_event (
  id BIGSERIAL PRIMARY KEY,
  token_id BIGINT NULL REFERENCES t_radar_token(id),
  source VARCHAR(32) NOT NULL,
  chain VARCHAR(64) NOT NULL,
  address VARCHAR(255) NOT NULL,
  symbol VARCHAR(64) NOT NULL DEFAULT '',
  signal_type VARCHAR(64) NOT NULL,
  priority VARCHAR(16) NOT NULL DEFAULT 'low',
  score DOUBLE PRECISION NOT NULL DEFAULT 0,
  reason TEXT NOT NULL DEFAULT '',
  tags_json TEXT NOT NULL DEFAULT '[]',
  raw_json TEXT NOT NULL DEFAULT '{}',
  dedupe_key VARCHAR(255) NOT NULL,
  created_at VARCHAR(40) NOT NULL DEFAULT '',
  pushed_at VARCHAR(40) NULL
);
COMMENT ON TABLE t_radar_signal_event IS '雷达信号事件表';
COMMENT ON COLUMN t_radar_signal_event.token_id IS '标的主键ID';
COMMENT ON COLUMN t_radar_signal_event.source IS '扫描器来源';
COMMENT ON COLUMN t_radar_signal_event.chain IS '链或市场标识';
COMMENT ON COLUMN t_radar_signal_event.address IS '标准化后的地址或合约标识';
COMMENT ON COLUMN t_radar_signal_event.symbol IS '展示用符号';
COMMENT ON COLUMN t_radar_signal_event.signal_type IS '信号类型编码';
COMMENT ON COLUMN t_radar_signal_event.priority IS '优先级';
COMMENT ON COLUMN t_radar_signal_event.score IS '信号分数';
COMMENT ON COLUMN t_radar_signal_event.reason IS '信号说明';
COMMENT ON COLUMN t_radar_signal_event.tags_json IS '信号标签JSON';
COMMENT ON COLUMN t_radar_signal_event.raw_json IS '原始信号JSON';
COMMENT ON COLUMN t_radar_signal_event.dedupe_key IS '去重键';
COMMENT ON COLUMN t_radar_signal_event.created_at IS '信号创建时间RFC3339Nano';
COMMENT ON COLUMN t_radar_signal_event.pushed_at IS '推送时间RFC3339Nano';
CREATE UNIQUE INDEX IF NOT EXISTS uk_radar_signal_event_dedupe_key ON t_radar_signal_event(dedupe_key);
CREATE INDEX IF NOT EXISTS idx_radar_signal_event_chain_address ON t_radar_signal_event(chain, address);
CREATE INDEX IF NOT EXISTS idx_radar_signal_event_source ON t_radar_signal_event(source);
CREATE INDEX IF NOT EXISTS idx_radar_signal_event_signal_type ON t_radar_signal_event(signal_type);
CREATE INDEX IF NOT EXISTS idx_radar_signal_event_priority ON t_radar_signal_event(priority);
CREATE INDEX IF NOT EXISTS idx_radar_signal_event_created_at ON t_radar_signal_event(created_at);
CREATE INDEX IF NOT EXISTS idx_radar_signal_event_pushed_at ON t_radar_signal_event(pushed_at);

CREATE TABLE IF NOT EXISTS t_radar_watchlist (
  id BIGSERIAL PRIMARY KEY,
  chain VARCHAR(64) NOT NULL,
  address VARCHAR(255) NOT NULL,
  symbol VARCHAR(64) NOT NULL DEFAULT '',
  name VARCHAR(255) NOT NULL DEFAULT '',
  status VARCHAR(32) NOT NULL DEFAULT 'watch',
  note TEXT NOT NULL DEFAULT '',
  updated_at VARCHAR(40) NOT NULL DEFAULT ''
);
COMMENT ON TABLE t_radar_watchlist IS '雷达观察名单表';
COMMENT ON COLUMN t_radar_watchlist.chain IS '链或市场标识';
COMMENT ON COLUMN t_radar_watchlist.address IS '标准化后的地址或合约标识';
COMMENT ON COLUMN t_radar_watchlist.symbol IS '展示用符号';
COMMENT ON COLUMN t_radar_watchlist.name IS '展示用名称';
COMMENT ON COLUMN t_radar_watchlist.status IS '观察状态';
COMMENT ON COLUMN t_radar_watchlist.note IS '备注';
COMMENT ON COLUMN t_radar_watchlist.updated_at IS '更新时间RFC3339Nano';
CREATE UNIQUE INDEX IF NOT EXISTS uk_radar_watchlist_chain_address ON t_radar_watchlist(chain, address);
CREATE INDEX IF NOT EXISTS idx_radar_watchlist_updated_at ON t_radar_watchlist(updated_at);

CREATE TABLE IF NOT EXISTS t_radar_scanner_run (
  id BIGSERIAL PRIMARY KEY,
  scanner VARCHAR(32) NOT NULL,
  started_at VARCHAR(40) NOT NULL DEFAULT '',
  finished_at VARCHAR(40) NOT NULL DEFAULT '',
  status VARCHAR(16) NOT NULL DEFAULT 'ok',
  error TEXT NOT NULL DEFAULT '',
  signal_count BIGINT NOT NULL DEFAULT 0,
  snapshot_count BIGINT NOT NULL DEFAULT 0,
  metadata_json TEXT NOT NULL DEFAULT '{}'
);
COMMENT ON TABLE t_radar_scanner_run IS '雷达调度运行日志表';
COMMENT ON COLUMN t_radar_scanner_run.scanner IS '扫描器名称';
COMMENT ON COLUMN t_radar_scanner_run.started_at IS '开始时间RFC3339Nano';
COMMENT ON COLUMN t_radar_scanner_run.finished_at IS '结束时间RFC3339Nano';
COMMENT ON COLUMN t_radar_scanner_run.status IS '运行状态';
COMMENT ON COLUMN t_radar_scanner_run.error IS '错误信息';
COMMENT ON COLUMN t_radar_scanner_run.signal_count IS '新增信号数';
COMMENT ON COLUMN t_radar_scanner_run.snapshot_count IS '新增快照数';
COMMENT ON COLUMN t_radar_scanner_run.metadata_json IS '运行元数据JSON';
CREATE INDEX IF NOT EXISTS idx_radar_scanner_run_scanner ON t_radar_scanner_run(scanner);
CREATE INDEX IF NOT EXISTS idx_radar_scanner_run_started_at ON t_radar_scanner_run(started_at);
CREATE INDEX IF NOT EXISTS idx_radar_scanner_run_status ON t_radar_scanner_run(status);

CREATE TABLE IF NOT EXISTS t_sys_app_setting (
  key VARCHAR(128) PRIMARY KEY,
  value_json TEXT NOT NULL DEFAULT '{}',
  updated_at VARCHAR(40) NOT NULL DEFAULT ''
);
COMMENT ON TABLE t_sys_app_setting IS '系统运行配置表';
COMMENT ON COLUMN t_sys_app_setting.key IS '配置键';
COMMENT ON COLUMN t_sys_app_setting.value_json IS '配置值JSON';
COMMENT ON COLUMN t_sys_app_setting.updated_at IS '更新时间RFC3339Nano';

CREATE TABLE IF NOT EXISTS t_insider_wallet (
  id BIGSERIAL PRIMARY KEY,
  address VARCHAR(64) NOT NULL,
  label VARCHAR(255) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
COMMENT ON TABLE t_insider_wallet IS '链上钱包监控钱包表';
COMMENT ON COLUMN t_insider_wallet.address IS '钱包地址';
COMMENT ON COLUMN t_insider_wallet.label IS '钱包标签';
CREATE UNIQUE INDEX IF NOT EXISTS uk_insider_wallet_address ON t_insider_wallet(address);

CREATE TABLE IF NOT EXISTS t_insider_token_account (
  id BIGSERIAL PRIMARY KEY,
  wallet_id BIGINT NOT NULL REFERENCES t_insider_wallet(id),
  mint_address VARCHAR(64) NOT NULL,
  token_name VARCHAR(255) NOT NULL DEFAULT '',
  balance DOUBLE PRECISION NOT NULL DEFAULT 0,
  decimals INTEGER NOT NULL DEFAULT 0,
  usd_value DOUBLE PRECISION NOT NULL DEFAULT 0,
  last_updated TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
COMMENT ON TABLE t_insider_token_account IS '链上钱包监控持仓账户表';
COMMENT ON COLUMN t_insider_token_account.wallet_id IS '钱包主键ID';
COMMENT ON COLUMN t_insider_token_account.mint_address IS '代币Mint地址';
COMMENT ON COLUMN t_insider_token_account.token_name IS '代币名称';
COMMENT ON COLUMN t_insider_token_account.balance IS '代币余额';
COMMENT ON COLUMN t_insider_token_account.decimals IS '代币精度';
COMMENT ON COLUMN t_insider_token_account.usd_value IS '美元价值';
COMMENT ON COLUMN t_insider_token_account.last_updated IS '最近更新时间';
CREATE UNIQUE INDEX IF NOT EXISTS uk_insider_token_account_wallet_mint ON t_insider_token_account(wallet_id, mint_address);

CREATE TABLE IF NOT EXISTS t_insider_transaction (
  id BIGSERIAL PRIMARY KEY,
  wallet_id BIGINT NOT NULL REFERENCES t_insider_wallet(id),
  signature VARCHAR(128) NOT NULL,
  mint_address VARCHAR(64) NOT NULL,
  token_name VARCHAR(255) NOT NULL DEFAULT '',
  tx_type VARCHAR(20) NOT NULL,
  amount DOUBLE PRECISION NOT NULL DEFAULT 0,
  price_at_time DOUBLE PRECISION NOT NULL DEFAULT 0,
  sol_amount DOUBLE PRECISION NOT NULL DEFAULT 0,
  block_time TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
COMMENT ON TABLE t_insider_transaction IS '链上钱包监控交易明细表';
COMMENT ON COLUMN t_insider_transaction.wallet_id IS '钱包主键ID';
COMMENT ON COLUMN t_insider_transaction.signature IS '交易签名';
COMMENT ON COLUMN t_insider_transaction.mint_address IS '代币Mint地址';
COMMENT ON COLUMN t_insider_transaction.token_name IS '代币名称';
COMMENT ON COLUMN t_insider_transaction.tx_type IS '交易类型';
COMMENT ON COLUMN t_insider_transaction.amount IS '代币数量';
COMMENT ON COLUMN t_insider_transaction.price_at_time IS '交易时价格';
COMMENT ON COLUMN t_insider_transaction.sol_amount IS 'SOL数量';
COMMENT ON COLUMN t_insider_transaction.block_time IS '区块时间';
CREATE UNIQUE INDEX IF NOT EXISTS uk_insider_transaction_sig_mint_type ON t_insider_transaction(signature, mint_address, tx_type);
CREATE INDEX IF NOT EXISTS idx_insider_transaction_wallet_id ON t_insider_transaction(wallet_id);
CREATE INDEX IF NOT EXISTS idx_insider_transaction_block_time ON t_insider_transaction(block_time);

CREATE TABLE IF NOT EXISTS t_insider_price_record (
  id BIGSERIAL PRIMARY KEY,
  mint_address VARCHAR(64) NOT NULL,
  price_usd DOUBLE PRECISION NOT NULL DEFAULT 0,
  source VARCHAR(50) NOT NULL DEFAULT 'jupiter',
  recorded_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
COMMENT ON TABLE t_insider_price_record IS '链上钱包监控价格记录表';
COMMENT ON COLUMN t_insider_price_record.mint_address IS '代币Mint地址';
COMMENT ON COLUMN t_insider_price_record.price_usd IS '美元价格';
COMMENT ON COLUMN t_insider_price_record.source IS '价格来源';
COMMENT ON COLUMN t_insider_price_record.recorded_at IS '记录时间';
CREATE INDEX IF NOT EXISTS idx_insider_price_record_mint ON t_insider_price_record(mint_address);
CREATE INDEX IF NOT EXISTS idx_insider_price_record_recorded_at ON t_insider_price_record(recorded_at);

CREATE TABLE IF NOT EXISTS t_insider_alert_rule (
  id BIGSERIAL PRIMARY KEY,
  wallet_id BIGINT NULL REFERENCES t_insider_wallet(id),
  rule_type VARCHAR(50) NOT NULL,
  threshold DOUBLE PRECISION NOT NULL DEFAULT 0,
  channel_ids_json TEXT NOT NULL DEFAULT '[]',
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
COMMENT ON TABLE t_insider_alert_rule IS '链上钱包监控告警规则表';
COMMENT ON COLUMN t_insider_alert_rule.wallet_id IS '钱包主键ID';
COMMENT ON COLUMN t_insider_alert_rule.rule_type IS '规则类型';
COMMENT ON COLUMN t_insider_alert_rule.threshold IS '阈值';
COMMENT ON COLUMN t_insider_alert_rule.channel_ids_json IS '通知渠道ID列表JSON';
COMMENT ON COLUMN t_insider_alert_rule.enabled IS '是否启用';
CREATE INDEX IF NOT EXISTS idx_insider_alert_rule_wallet_id ON t_insider_alert_rule(wallet_id);

CREATE TABLE IF NOT EXISTS t_insider_alert_history (
  id BIGSERIAL PRIMARY KEY,
  wallet_id BIGINT NULL REFERENCES t_insider_wallet(id),
  alert_rule_id BIGINT NULL REFERENCES t_insider_alert_rule(id),
  alert_type VARCHAR(50) NOT NULL,
  message TEXT NOT NULL DEFAULT '',
  level VARCHAR(20) NOT NULL DEFAULT 'info',
  data_json TEXT NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
COMMENT ON TABLE t_insider_alert_history IS '链上钱包监控告警历史表';
COMMENT ON COLUMN t_insider_alert_history.wallet_id IS '钱包主键ID';
COMMENT ON COLUMN t_insider_alert_history.alert_rule_id IS '告警规则主键ID';
COMMENT ON COLUMN t_insider_alert_history.alert_type IS '告警类型';
COMMENT ON COLUMN t_insider_alert_history.message IS '告警消息';
COMMENT ON COLUMN t_insider_alert_history.level IS '告警级别';
COMMENT ON COLUMN t_insider_alert_history.data_json IS '告警载荷JSON';
COMMENT ON COLUMN t_insider_alert_history.created_at IS '创建时间';
CREATE INDEX IF NOT EXISTS idx_insider_alert_history_wallet_id ON t_insider_alert_history(wallet_id);
CREATE INDEX IF NOT EXISTS idx_insider_alert_history_created_at ON t_insider_alert_history(created_at);

CREATE TABLE IF NOT EXISTS t_insider_wallet_snapshot (
  id BIGSERIAL PRIMARY KEY,
  wallet_id BIGINT NOT NULL REFERENCES t_insider_wallet(id),
  total_balance_usd DOUBLE PRECISION NOT NULL DEFAULT 0,
  token_mints_json TEXT NOT NULL DEFAULT '[]',
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
COMMENT ON TABLE t_insider_wallet_snapshot IS '链上钱包监控资产快照表';
COMMENT ON COLUMN t_insider_wallet_snapshot.wallet_id IS '钱包主键ID';
COMMENT ON COLUMN t_insider_wallet_snapshot.total_balance_usd IS '总美元余额';
COMMENT ON COLUMN t_insider_wallet_snapshot.token_mints_json IS '代币Mint列表JSON';
COMMENT ON COLUMN t_insider_wallet_snapshot.created_at IS '创建时间';
CREATE INDEX IF NOT EXISTS idx_insider_wallet_snapshot_wallet_id ON t_insider_wallet_snapshot(wallet_id);
CREATE INDEX IF NOT EXISTS idx_insider_wallet_snapshot_created_at ON t_insider_wallet_snapshot(created_at);

CREATE TABLE IF NOT EXISTS t_insider_notification_channel (
  id BIGSERIAL PRIMARY KEY,
  name VARCHAR(100) NOT NULL,
  channel_type VARCHAR(20) NOT NULL,
  recipient VARCHAR(255) NOT NULL DEFAULT '',
  webhook_url VARCHAR(500) NOT NULL DEFAULT '',
  bot_token VARCHAR(255) NOT NULL DEFAULT '',
  chat_id VARCHAR(100) NOT NULL DEFAULT '',
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
COMMENT ON TABLE t_insider_notification_channel IS '链上钱包监控通知渠道表';
COMMENT ON COLUMN t_insider_notification_channel.name IS '渠道名称';
COMMENT ON COLUMN t_insider_notification_channel.channel_type IS '渠道类型';
COMMENT ON COLUMN t_insider_notification_channel.recipient IS '接收对象';
COMMENT ON COLUMN t_insider_notification_channel.webhook_url IS '回调地址';
COMMENT ON COLUMN t_insider_notification_channel.bot_token IS '机器人令牌';
COMMENT ON COLUMN t_insider_notification_channel.chat_id IS '会话ID';
COMMENT ON COLUMN t_insider_notification_channel.enabled IS '是否启用';
