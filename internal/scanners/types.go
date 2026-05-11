package scanners

// TokenPayload 是扫描器向存储层提交的代币基础资料。
type TokenPayload struct {
	Chain           string   // Chain 是链或市场标识。
	Address         string   // Address 是合约地址或内部资产标识。
	Symbol          string   // Symbol 是代币符号。
	Name            string   // Name 是代币名称。
	NarrativeTheme  string   // NarrativeTheme 是扫描器识别出的叙事主题。
	NarrativeTags   []string // NarrativeTags 是更细粒度的叙事标签。
	Description     string   // Description 是代币简介。
	SocialLinksJSON string   // SocialLinksJSON 是社交链接的 JSON 字符串。
}

// SnapshotPayload 是扫描器向 snapshots 表提交的一次行情快照。
type SnapshotPayload struct {
	Source     string         // Source 是产生快照的扫描器编号。
	Chain      string         // Chain 是链或市场标识。
	Address    string         // Address 是合约地址或内部资产标识。
	Symbol     string         // Symbol 是代币符号。
	Name       string         // Name 是代币名称。
	Price      *float64       // Price 是当前价格。
	MC         *float64       // MC 是市值或估算市值。
	Liq        *float64       // Liq 是流动性金额。
	Volume     *float64       // Volume 是成交量。
	Holders    *int64         // Holders 是持有人数量。
	SmartMoney *int64         // SmartMoney 是智能钱相关计数。
	FundingPct *float64       // FundingPct 是资金费率百分比。
	OIUSD      *float64       // OIUSD 是未平仓合约美元价值。
	OID6H      *float64       // OID6H 是 6 小时 OI 变化。
	Buys1H     *int64         // Buys1H 是 1 小时买入次数。
	Sells1H    *int64         // Sells1H 是 1 小时卖出次数。
	AgeH       *float64       // AgeH 是代币年龄小时数。
	Raw        map[string]any // Raw 保存原始响应和辅助计算数据。
}

// SignalPayload 是扫描器向 signals 表提交的候选信号。
type SignalPayload struct {
	Source     string         // Source 是产生信号的扫描器编号。
	Chain      string         // Chain 是链或市场标识。
	Address    string         // Address 是合约地址或内部资产标识。
	Symbol     string         // Symbol 是代币符号。
	Name       string         // Name 是代币名称。
	SignalType string         // SignalType 是信号类型，例如 alpha_discovery、momentum、resonance。
	Priority   string         // Priority 是信号优先级：high、medium、low。
	Score      float64        // Score 是扫描器计算出的综合分数。
	Reason     string         // Reason 是触发信号的可读原因。
	Tags       []string       // Tags 是用于页面展示和后续筛选的标签。
	Raw        map[string]any // Raw 保存原始响应和辅助计算数据。
	Token      *TokenPayload  // Token 是可选的代币资料补充，会同步更新 tokens 表。
	DedupeKey  string         // DedupeKey 是可选自定义去重键；为空时由存储层按时间桶生成。
}

// Result 是单次扫描任务的统一返回值。
type Result struct {
	ScannerName string            // ScannerName 是扫描器编号。
	Snapshots   []SnapshotPayload // Snapshots 是本次扫描采集到的行情快照。
	Signals     []SignalPayload   // Signals 是本次扫描产生的候选信号。
	Metadata    map[string]any    // Metadata 是任务级别额外信息，会写入 scanner_runs.metadata_json。
	Warnings    []string          // Warnings 是非致命告警，任务仍可成功完成。
}
