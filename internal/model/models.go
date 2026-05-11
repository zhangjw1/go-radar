package model

// TokenProfile 对应 tokens 表，保存代币的基础资料和叙事信息。
type TokenProfile struct {
	ID                int64  `gorm:"column:id;primaryKey" json:"id"`                    // ID 是 tokens 表主键。
	Chain             string `gorm:"column:chain" json:"chain"`                         // Chain 是链或市场标识，例如 eth、bsc、base、binance_perp。
	Address           string `gorm:"column:address" json:"address"`                     // Address 是归一化后的合约地址或内部资产标识。
	Symbol            string `gorm:"column:symbol" json:"symbol"`                       // Symbol 是代币符号，展示时通常转为大写。
	Name              string `gorm:"column:name" json:"name"`                           // Name 是代币名称，缺失时可退化为 Symbol。
	NarrativeTheme    string `gorm:"column:narrative_theme" json:"narrative_theme"`     // NarrativeTheme 是项目所属叙事主题，例如 AI、RWA、Meme。
	NarrativeTagsJSON string `gorm:"column:narrative_tags_json" json:"narrative_tags_json"` // NarrativeTagsJSON 保存叙事标签数组的 JSON 字符串。
	SocialLinksJSON   string `gorm:"column:social_links_json" json:"social_links_json"` // SocialLinksJSON 保存官网、Twitter、Telegram 等链接 JSON。
	Description       string `gorm:"column:description" json:"description"`             // Description 是项目简介或扫描器生成的摘要。
	FirstSeenAt       string `gorm:"column:first_seen_at" json:"first_seen_at"`         // FirstSeenAt 是首次入库时间，使用 SQLite 中的字符串时间格式。
	LastSeenAt        string `gorm:"column:last_seen_at" json:"last_seen_at"`           // LastSeenAt 是最近一次被扫描或更新的时间。
}

// TableName 固定 TokenProfile 对应的数据库表名。
func (TokenProfile) TableName() string {
	return "tokens"
}

// TokenSnapshot 对应 snapshots 表，保存一次扫描得到的市场快照。
type TokenSnapshot struct {
	ID         int64    `gorm:"column:id;primaryKey" json:"id"`       // ID 是 snapshots 表主键。
	TokenID    int64    `gorm:"column:token_id" json:"token_id"`      // TokenID 关联 tokens.id。
	Source     string   `gorm:"column:source" json:"source"`          // Source 是产生快照的扫描器编号，例如 s2、s5。
	Price      *float64 `gorm:"column:price" json:"price"`            // Price 是扫描时价格，缺失时为 nil。
	MC         *float64 `gorm:"column:mc" json:"mc"`                  // MC 是市值或估算市值。
	Liq        *float64 `gorm:"column:liq" json:"liq"`                // Liq 是流动性金额。
	Volume     *float64 `gorm:"column:volume" json:"volume"`          // Volume 是成交量，具体周期由扫描器来源决定。
	Holders    *int64   `gorm:"column:holders" json:"holders"`        // Holders 是持有人数量。
	SmartMoney *int64   `gorm:"column:smart_money" json:"smart_money"` // SmartMoney 是智能钱相关人数或计数。
	FundingPct *float64 `gorm:"column:funding_pct" json:"funding_pct"` // FundingPct 是资金费率百分比。
	OIUSD      *float64 `gorm:"column:oi_usd" json:"oi_usd"`          // OIUSD 是未平仓合约美元价值。
	OID6H      *float64 `gorm:"column:oi_d6h" json:"oi_d6h"`          // OID6H 是 6 小时 OI 变化。
	Buys1H     *int64   `gorm:"column:buys_1h" json:"buys_1h"`        // Buys1H 是 1 小时买入次数。
	Sells1H    *int64   `gorm:"column:sells_1h" json:"sells_1h"`      // Sells1H 是 1 小时卖出次数。
	AgeH       *float64 `gorm:"column:age_h" json:"age_h"`            // AgeH 是代币创建至今小时数。
	RawJSON    string   `gorm:"column:raw_json" json:"raw_json"`      // RawJSON 保存原始接口返回或中间计算数据。
	CreatedAt  string   `gorm:"column:created_at" json:"created_at"`  // CreatedAt 是快照创建时间。
}

// TableName 固定 TokenSnapshot 对应的数据库表名。
func (TokenSnapshot) TableName() string {
	return "snapshots"
}

// SignalEvent 对应 signals 表，保存扫描器产生的可行动信号。
type SignalEvent struct {
	ID         int64         `gorm:"column:id;primaryKey" json:"id"`        // ID 是 signals 表主键。
	TokenID    *int64        `gorm:"column:token_id" json:"token_id"`       // TokenID 关联 tokens.id，历史数据可能为空。
	Source     string        `gorm:"column:source" json:"source"`           // Source 是信号来源扫描器，例如 s1、s2、system。
	Chain      string        `gorm:"column:chain" json:"chain"`             // Chain 是链或市场标识。
	Address    string        `gorm:"column:address" json:"address"`         // Address 是归一化后的合约地址或内部资产标识。
	Symbol     string        `gorm:"column:symbol" json:"symbol"`           // Symbol 是信号展示用代币符号。
	SignalType string        `gorm:"column:signal_type" json:"signal_type"` // SignalType 是信号类型，例如 momentum、resonance。
	Priority   string        `gorm:"column:priority" json:"priority"`       // Priority 是优先级：high、medium、low。
	Score      float64       `gorm:"column:score" json:"score"`             // Score 是扫描器计算出的信号分数。
	Reason     string        `gorm:"column:reason" json:"reason"`           // Reason 是信号触发原因，直接展示给用户。
	TagsJSON   string        `gorm:"column:tags_json" json:"tags_json"`     // TagsJSON 保存标签数组 JSON。
	RawJSON    string        `gorm:"column:raw_json" json:"raw_json"`       // RawJSON 保存扫描器原始上下文。
	DedupeKey  string        `gorm:"column:dedupe_key" json:"dedupe_key"`   // DedupeKey 是去重键，用于避免同一时间桶重复入库。
	CreatedAt  string        `gorm:"column:created_at" json:"created_at"`   // CreatedAt 是信号创建时间。
	PushedAt   *string       `gorm:"column:pushed_at" json:"pushed_at"`     // PushedAt 是 Telegram 推送时间；为空表示尚未推送。
	Token      *TokenProfile `gorm:"foreignKey:TokenID" json:"-"`           // Token 是可选的 GORM 关联对象，API JSON 中不直接输出。
}

// TableName 固定 SignalEvent 对应的数据库表名。
func (SignalEvent) TableName() string {
	return "signals"
}

// WatchlistItem 对应 watchlist 表，保存用户关注的代币。
type WatchlistItem struct {
	ID        int64  `gorm:"column:id;primaryKey" json:"id"` // ID 是 watchlist 表主键。
	Chain     string `gorm:"column:chain" json:"chain"`      // Chain 是关注代币所在链。
	Address   string `gorm:"column:address" json:"address"`  // Address 是关注代币地址。
	Symbol    string `gorm:"column:symbol" json:"symbol"`     // Symbol 是展示用代币符号。
	Name      string `gorm:"column:name" json:"name"`         // Name 是展示用代币名称。
	Status    string `gorm:"column:status" json:"status"`     // Status 是观察状态，例如 active、paused。
	Note      string `gorm:"column:note" json:"note"`         // Note 是用户备注。
	UpdatedAt string `gorm:"column:updated_at" json:"updated_at"` // UpdatedAt 是最近更新时间。
}

// TableName 固定 WatchlistItem 对应的数据库表名。
func (WatchlistItem) TableName() string {
	return "watchlist"
}

// ScannerRun 对应 scanner_runs 表，记录每次扫描任务执行结果。
type ScannerRun struct {
	ID            int64  `gorm:"column:id;primaryKey" json:"id"`           // ID 是 scanner_runs 表主键。
	Scanner       string `gorm:"column:scanner" json:"scanner"`            // Scanner 是扫描器编号。
	StartedAt     string `gorm:"column:started_at" json:"started_at"`      // StartedAt 是任务开始时间。
	FinishedAt    string `gorm:"column:finished_at" json:"finished_at"`    // FinishedAt 是任务结束时间。
	Status        string `gorm:"column:status" json:"status"`              // Status 是任务状态，例如 ok、error、skipped。
	Error         string `gorm:"column:error" json:"error"`                // Error 是失败原因，成功时为空。
	SignalCount   int64  `gorm:"column:signal_count" json:"signal_count"` // SignalCount 是本次新增信号数。
	SnapshotCount int64  `gorm:"column:snapshot_count" json:"snapshot_count"` // SnapshotCount 是本次新增快照数。
	MetadataJSON  string `gorm:"column:metadata_json" json:"metadata_json"` // MetadataJSON 保存任务额外信息，例如 warnings 和推送数量。
}

// TableName 固定 ScannerRun 对应的数据库表名。
func (ScannerRun) TableName() string {
	return "scanner_runs"
}

// AppSetting 对应 settings 表，保存运行时可调参数。
type AppSetting struct {
	Key       string `gorm:"column:key;primaryKey" json:"key"`     // Key 是设置项名称。
	ValueJSON string `gorm:"column:value_json" json:"value_json"`  // ValueJSON 是设置值的 JSON 表示。
	UpdatedAt  string `gorm:"column:updated_at" json:"updated_at"` // UpdatedAt 是最近更新时间。
}

// TableName 固定 AppSetting 对应的数据库表名。
func (AppSetting) TableName() string {
	return "settings"
}
