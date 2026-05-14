package insider

import "time"

const (
	EngineService = "service"
	EngineLegacy  = "legacy"

	RuleBalanceChange = "balance_change"
	RuleNewToken      = "new_token"

	NativeSOLMint = "So11111111111111111111111111111111111111112"
)

type Wallet struct {
	ID        int64     `gorm:"column:id;primaryKey;comment:主键ID" json:"id"`
	Address   string    `gorm:"column:address;uniqueIndex;size:64;not null;comment:钱包地址" json:"address"`
	Label     string    `gorm:"column:label;size:255;not null;default:'';comment:钱包标签" json:"label"`
	CreatedAt time.Time `gorm:"column:created_at;comment:创建时间" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;comment:更新时间" json:"updated_at"`
}

func (Wallet) TableName() string { return "t_insider_wallet" }

func (w Wallet) LabelOrAddress() string {
	if w.Label != "" {
		return w.Label
	}
	return w.Address
}

type TokenAccount struct {
	ID          int64     `gorm:"column:id;primaryKey;comment:主键ID" json:"id"`
	WalletID    int64     `gorm:"column:wallet_id;uniqueIndex:idx_insider_wallet_mint;not null;comment:钱包主键ID" json:"wallet_id"`
	MintAddress string    `gorm:"column:mint_address;uniqueIndex:idx_insider_wallet_mint;size:64;not null;comment:代币Mint地址" json:"mint_address"`
	TokenName   string    `gorm:"column:token_name;size:255;not null;default:'';comment:代币名称" json:"token_name"`
	Balance     float64   `gorm:"column:balance;not null;default:0;comment:代币余额" json:"balance"`
	Decimals    int       `gorm:"column:decimals;not null;default:0;comment:代币精度" json:"decimals"`
	USDValue    float64   `gorm:"column:usd_value;not null;default:0;comment:美元价值" json:"usd_value"`
	LastUpdated time.Time `gorm:"column:last_updated;comment:最近更新时间" json:"last_updated"`
}

func (TokenAccount) TableName() string { return "t_insider_token_account" }

type TokenHolding struct {
	MintAddress  string  `json:"mint_address"`
	TokenName    string  `json:"token_name"`
	Balance      float64 `json:"balance"`
	Decimals     int     `json:"decimals"`
	USDValue     float64 `json:"usd_value"`
	TotalBought  float64 `json:"total_bought"`
	TotalSold    float64 `json:"total_sold"`
	Remaining    float64 `json:"remaining"`
	CostBasis    float64 `json:"cost_basis"`
	CurrentValue float64 `json:"current_value"`
	PnL          float64 `json:"pnl"`
	PnLPercent   float64 `json:"pnl_percent"`
}

type Transaction struct {
	ID          int64     `gorm:"column:id;primaryKey;comment:主键ID" json:"id"`
	WalletID    int64     `gorm:"column:wallet_id;index;not null;comment:钱包主键ID" json:"wallet_id"`
	Signature   string    `gorm:"column:signature;uniqueIndex:idx_insider_tx_unique;size:128;not null;comment:交易签名" json:"signature"`
	MintAddress string    `gorm:"column:mint_address;uniqueIndex:idx_insider_tx_unique;size:64;not null;comment:代币Mint地址" json:"mint_address"`
	TokenName   string    `gorm:"column:token_name;size:255;not null;default:'';comment:代币名称" json:"token_name"`
	TxType      string    `gorm:"column:tx_type;uniqueIndex:idx_insider_tx_unique;size:20;not null;comment:交易类型" json:"tx_type"`
	Amount      float64   `gorm:"column:amount;not null;default:0;comment:代币数量" json:"amount"`
	PriceAtTime float64   `gorm:"column:price_at_time;not null;default:0;comment:交易时价格" json:"price_at_time"`
	SolAmount   float64   `gorm:"column:sol_amount;not null;default:0;comment:SOL数量" json:"sol_amount"`
	BlockTime   time.Time `gorm:"column:block_time;index;not null;comment:区块时间" json:"block_time"`
	CreatedAt   time.Time `gorm:"column:created_at;comment:创建时间" json:"created_at"`
}

func (Transaction) TableName() string { return "t_insider_transaction" }

type PriceRecord struct {
	ID          int64     `gorm:"column:id;primaryKey;comment:主键ID" json:"id"`
	MintAddress string    `gorm:"column:mint_address;index;size:64;not null;comment:代币Mint地址" json:"mint_address"`
	PriceUSD    float64   `gorm:"column:price_usd;not null;default:0;comment:美元价格" json:"price_usd"`
	Source      string    `gorm:"column:source;size:50;not null;default:'jupiter';comment:价格来源" json:"source"`
	RecordedAt  time.Time `gorm:"column:recorded_at;index;comment:记录时间" json:"recorded_at"`
}

func (PriceRecord) TableName() string { return "t_insider_price_record" }

type AlertRule struct {
	ID             int64     `gorm:"column:id;primaryKey;comment:主键ID" json:"id"`
	WalletID       *int64    `gorm:"column:wallet_id;index;comment:钱包主键ID" json:"wallet_id"`
	RuleType       string    `gorm:"column:rule_type;size:50;not null;comment:规则类型" json:"rule_type"`
	Threshold      float64   `gorm:"column:threshold;not null;default:0;comment:阈值" json:"threshold"`
	ChannelIDsJSON string    `gorm:"column:channel_ids_json;type:text;not null;default:'[]';comment:通知渠道ID列表JSON" json:"-"`
	Enabled        bool      `gorm:"column:enabled;not null;default:true;comment:是否启用" json:"enabled"`
	CreatedAt      time.Time `gorm:"column:created_at;comment:创建时间" json:"created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at;comment:更新时间" json:"updated_at"`
	Wallet         *Wallet   `gorm:"-" json:"wallet,omitempty"`
	ChannelIDs     []int64   `gorm:"-" json:"channel_ids"`
}

func (AlertRule) TableName() string { return "t_insider_alert_rule" }

type AlertHistory struct {
	ID          int64          `gorm:"column:id;primaryKey;comment:主键ID" json:"id"`
	WalletID    *int64         `gorm:"column:wallet_id;index;comment:钱包主键ID" json:"wallet_id"`
	AlertRuleID *int64         `gorm:"column:alert_rule_id;comment:告警规则主键ID" json:"alert_rule_id"`
	AlertType   string         `gorm:"column:alert_type;size:50;not null;comment:告警类型" json:"alert_type"`
	Message     string         `gorm:"column:message;type:text;not null;default:'';comment:告警消息" json:"message"`
	Level       string         `gorm:"column:level;size:20;not null;default:'info';comment:告警级别" json:"level"`
	DataJSON    string         `gorm:"column:data_json;type:text;comment:告警载荷JSON" json:"-"`
	CreatedAt   time.Time      `gorm:"column:created_at;index;comment:创建时间" json:"created_at"`
	Wallet      *Wallet        `gorm:"-" json:"wallet,omitempty"`
	Data        map[string]any `gorm:"-" json:"data"`
}

func (AlertHistory) TableName() string { return "t_insider_alert_history" }

type WalletSnapshot struct {
	ID              int64     `gorm:"column:id;primaryKey;comment:主键ID" json:"id"`
	WalletID        int64     `gorm:"column:wallet_id;index;not null;comment:钱包主键ID" json:"wallet_id"`
	TotalBalanceUSD float64   `gorm:"column:total_balance_usd;not null;default:0;comment:总美元余额" json:"total_balance_usd"`
	TokenMintsJSON  string    `gorm:"column:token_mints_json;type:text;comment:代币Mint列表JSON" json:"-"`
	CreatedAt       time.Time `gorm:"column:created_at;index;comment:创建时间" json:"created_at"`
}

func (WalletSnapshot) TableName() string { return "t_insider_wallet_snapshot" }

type NotificationChannel struct {
	ID          int64     `gorm:"column:id;primaryKey;comment:主键ID" json:"id"`
	Name        string    `gorm:"column:name;size:100;not null;comment:渠道名称" json:"name"`
	ChannelType string    `gorm:"column:channel_type;size:20;not null;comment:渠道类型" json:"channel_type"`
	Recipient   string    `gorm:"column:recipient;size:255;not null;default:'';comment:接收对象" json:"recipient"`
	WebhookURL  string    `gorm:"column:webhook_url;size:500;not null;default:'';comment:回调地址" json:"webhook_url"`
	BotToken    string    `gorm:"column:bot_token;size:255;not null;default:'';comment:机器人令牌" json:"bot_token"`
	ChatID      string    `gorm:"column:chat_id;size:100;not null;default:'';comment:会话ID" json:"chat_id"`
	Enabled     bool      `gorm:"column:enabled;not null;default:true;comment:是否启用" json:"enabled"`
	CreatedAt   time.Time `gorm:"column:created_at;comment:创建时间" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at;comment:更新时间" json:"updated_at"`
}

func (NotificationChannel) TableName() string { return "t_insider_notification_channel" }

type Analytics struct {
	WalletID        int64      `json:"wallet_id"`
	WinRate         float64    `json:"win_rate"`
	TotalPnL        float64    `json:"total_pnl"`
	RealizedPnL     float64    `json:"realized_pnl"`
	UnrealizedPnL   float64    `json:"unrealized_pnl"`
	AvgHoldTime     string     `json:"avg_hold_time"`
	TxCount         int64      `json:"tx_count"`
	BuyCount        int64      `json:"buy_count"`
	SellCount       int64      `json:"sell_count"`
	TotalCost       float64    `json:"total_cost"`
	AvgCostPerToken float64    `json:"avg_cost_per_token"`
	LastActiveTime  *time.Time `json:"last_active_time"`
	Period          string     `json:"period"`
}

type SyncStatus struct {
	Syncing   bool   `json:"syncing"`
	LastError string `json:"last_error"`
	Engine    string `json:"engine"`
}
