package model

type TokenProfile struct {
	ID                int64  `gorm:"column:id;primaryKey;comment:主键ID" json:"id"`
	Chain             string `gorm:"column:chain;size:64;not null;index:idx_radar_token_chain_address,priority:1;comment:链或市场标识" json:"chain"`
	Address           string `gorm:"column:address;size:255;not null;index:idx_radar_token_chain_address,priority:2;comment:标准化后的地址或合约标识" json:"address"`
	Symbol            string `gorm:"column:symbol;size:64;not null;default:'';comment:展示用符号" json:"symbol"`
	Name              string `gorm:"column:name;size:255;not null;default:'';comment:展示用名称" json:"name"`
	NarrativeTheme    string `gorm:"column:narrative_theme;size:128;not null;default:'';comment:叙事主题" json:"narrative_theme"`
	NarrativeTagsJSON string `gorm:"column:narrative_tags_json;type:text;not null;default:'[]';comment:叙事标签JSON" json:"narrative_tags_json"`
	SocialLinksJSON   string `gorm:"column:social_links_json;type:text;not null;default:'{}';comment:社交链接JSON" json:"social_links_json"`
	Description       string `gorm:"column:description;type:text;not null;default:'';comment:标的描述" json:"description"`
	FirstSeenAt       string `gorm:"column:first_seen_at;size:40;not null;default:'';comment:首次发现时间RFC3339Nano" json:"first_seen_at"`
	LastSeenAt        string `gorm:"column:last_seen_at;size:40;not null;default:'';comment:最近发现时间RFC3339Nano" json:"last_seen_at"`
}

func (TokenProfile) TableName() string {
	return "t_radar_token"
}

type TokenSnapshot struct {
	ID         int64    `gorm:"column:id;primaryKey;comment:主键ID" json:"id"`
	TokenID    int64    `gorm:"column:token_id;not null;index;comment:标的主键ID" json:"token_id"`
	Source     string   `gorm:"column:source;size:32;not null;index;comment:扫描器来源" json:"source"`
	Price      *float64 `gorm:"column:price;comment:价格" json:"price"`
	MC         *float64 `gorm:"column:mc;comment:市值" json:"mc"`
	Liq        *float64 `gorm:"column:liq;comment:流动性" json:"liq"`
	Volume     *float64 `gorm:"column:volume;comment:成交量" json:"volume"`
	Holders    *int64   `gorm:"column:holders;comment:持有人数" json:"holders"`
	SmartMoney *int64   `gorm:"column:smart_money;comment:聪明钱数量" json:"smart_money"`
	FundingPct *float64 `gorm:"column:funding_pct;comment:资金费率百分比" json:"funding_pct"`
	OIUSD      *float64 `gorm:"column:oi_usd;comment:未平仓合约美元价值" json:"oi_usd"`
	OID6H      *float64 `gorm:"column:oi_d6h;comment:六小时OI变化" json:"oi_d6h"`
	Buys1H     *int64   `gorm:"column:buys_1h;comment:一小时买入次数" json:"buys_1h"`
	Sells1H    *int64   `gorm:"column:sells_1h;comment:一小时卖出次数" json:"sells_1h"`
	AgeH       *float64 `gorm:"column:age_h;comment:标的年龄小时数" json:"age_h"`
	RawJSON    string   `gorm:"column:raw_json;type:text;not null;default:'{}';comment:原始快照JSON" json:"raw_json"`
	CreatedAt  string   `gorm:"column:created_at;size:40;not null;default:'';index;comment:快照创建时间RFC3339Nano" json:"created_at"`
}

func (TokenSnapshot) TableName() string {
	return "t_radar_market_snapshot"
}

type SignalEvent struct {
	ID         int64         `gorm:"column:id;primaryKey;comment:主键ID" json:"id"`
	TokenID    *int64        `gorm:"column:token_id;index;comment:标的主键ID" json:"token_id"`
	Source     string        `gorm:"column:source;size:32;not null;index;comment:扫描器来源" json:"source"`
	Chain      string        `gorm:"column:chain;size:64;not null;index:idx_radar_signal_chain_address,priority:1;comment:链或市场标识" json:"chain"`
	Address    string        `gorm:"column:address;size:255;not null;index:idx_radar_signal_chain_address,priority:2;comment:标准化后的地址或合约标识" json:"address"`
	Symbol     string        `gorm:"column:symbol;size:64;not null;default:'';comment:展示用符号" json:"symbol"`
	SignalType string        `gorm:"column:signal_type;size:64;not null;index;comment:信号类型编码" json:"signal_type"`
	Priority   string        `gorm:"column:priority;size:16;not null;default:'low';index;comment:优先级" json:"priority"`
	Score      float64       `gorm:"column:score;not null;default:0;comment:信号分数" json:"score"`
	Reason     string        `gorm:"column:reason;type:text;not null;default:'';comment:信号说明" json:"reason"`
	TagsJSON   string        `gorm:"column:tags_json;type:text;not null;default:'[]';comment:信号标签JSON" json:"tags_json"`
	RawJSON    string        `gorm:"column:raw_json;type:text;not null;default:'{}';comment:原始信号JSON" json:"raw_json"`
	DedupeKey  string        `gorm:"column:dedupe_key;size:255;not null;uniqueIndex;comment:去重键" json:"dedupe_key"`
	CreatedAt  string        `gorm:"column:created_at;size:40;not null;default:'';index;comment:信号创建时间RFC3339Nano" json:"created_at"`
	PushedAt   *string       `gorm:"column:pushed_at;size:40;index;comment:推送时间RFC3339Nano" json:"pushed_at"`
	Token      *TokenProfile `gorm:"foreignKey:TokenID" json:"-"`
}

func (SignalEvent) TableName() string {
	return "t_radar_signal_event"
}

type WatchlistItem struct {
	ID        int64  `gorm:"column:id;primaryKey;comment:主键ID" json:"id"`
	Chain     string `gorm:"column:chain;size:64;not null;index:idx_radar_watchlist_chain_address,priority:1;comment:链或市场标识" json:"chain"`
	Address   string `gorm:"column:address;size:255;not null;index:idx_radar_watchlist_chain_address,priority:2;comment:标准化后的地址或合约标识" json:"address"`
	Symbol    string `gorm:"column:symbol;size:64;not null;default:'';comment:展示用符号" json:"symbol"`
	Name      string `gorm:"column:name;size:255;not null;default:'';comment:展示用名称" json:"name"`
	Status    string `gorm:"column:status;size:32;not null;default:'watch';comment:观察状态" json:"status"`
	Note      string `gorm:"column:note;type:text;not null;default:'';comment:备注" json:"note"`
	UpdatedAt string `gorm:"column:updated_at;size:40;not null;default:'';index;comment:更新时间RFC3339Nano" json:"updated_at"`
}

func (WatchlistItem) TableName() string {
	return "t_radar_watchlist"
}

type ScannerRun struct {
	ID            int64  `gorm:"column:id;primaryKey;comment:主键ID" json:"id"`
	Scanner       string `gorm:"column:scanner;size:32;not null;index;comment:扫描器名称" json:"scanner"`
	StartedAt     string `gorm:"column:started_at;size:40;not null;default:'';index;comment:开始时间RFC3339Nano" json:"started_at"`
	FinishedAt    string `gorm:"column:finished_at;size:40;not null;default:'';comment:结束时间RFC3339Nano" json:"finished_at"`
	Status        string `gorm:"column:status;size:16;not null;default:'ok';index;comment:运行状态" json:"status"`
	Error         string `gorm:"column:error;type:text;not null;default:'';comment:错误信息" json:"error"`
	SignalCount   int64  `gorm:"column:signal_count;not null;default:0;comment:新增信号数" json:"signal_count"`
	SnapshotCount int64  `gorm:"column:snapshot_count;not null;default:0;comment:新增快照数" json:"snapshot_count"`
	MetadataJSON  string `gorm:"column:metadata_json;type:text;not null;default:'{}';comment:运行元数据JSON" json:"metadata_json"`
}

func (ScannerRun) TableName() string {
	return "t_radar_scanner_run"
}

type AppSetting struct {
	Key       string `gorm:"column:key;primaryKey;size:128;comment:配置键" json:"key"`
	ValueJSON string `gorm:"column:value_json;type:text;not null;default:'{}';comment:配置值JSON" json:"value_json"`
	UpdatedAt string `gorm:"column:updated_at;size:40;not null;default:'';comment:更新时间RFC3339Nano" json:"updated_at"`
}

func (AppSetting) TableName() string {
	return "t_sys_app_setting"
}

// ScannerState 保存扫描器自己的轻量状态。
//
// 业务上它用于替代 Python 版本里的本地 JSON/SQLite 状态文件，例如：
// S1 的 Alpha 项目生命周期、S3 的热度历史、S5 的动量推送计数。
type ScannerState struct {
	ID        int64  `gorm:"column:id;primaryKey;comment:主键ID" json:"id"`
	Source    string `gorm:"column:source;size:32;not null;index:idx_radar_scanner_state_source_key,priority:1;comment:扫描器编号" json:"source"`
	Key       string `gorm:"column:key;size:255;not null;index:idx_radar_scanner_state_source_key,priority:2;comment:状态键" json:"key"`
	ValueJSON string `gorm:"column:value_json;type:text;not null;default:'{}';comment:状态内容JSON" json:"value_json"`
	UpdatedAt string `gorm:"column:updated_at;size:40;not null;default:'';index;comment:更新时间RFC3339Nano" json:"updated_at"`
}

func (ScannerState) TableName() string {
	return "t_radar_scanner_state"
}
