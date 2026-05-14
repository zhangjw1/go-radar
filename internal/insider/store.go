package insider

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Store struct {
	db *gorm.DB
}

func NewStore(db *gorm.DB) *Store {
	return &Store{db: db}
}

func (s *Store) DB() *gorm.DB {
	return s.db
}

func (s *Store) CreateWallet(address, label string) (*Wallet, error) {
	wallet := &Wallet{Address: strings.TrimSpace(address), Label: strings.TrimSpace(label)}
	if err := s.db.Create(wallet).Error; err != nil {
		return nil, err
	}
	return wallet, nil
}

func (s *Store) ListWallets() ([]Wallet, error) {
	var wallets []Wallet
	err := s.db.Order("created_at desc").Find(&wallets).Error
	return wallets, err
}

func (s *Store) GetWallet(id int64) (*Wallet, error) {
	var wallet Wallet
	if err := s.db.First(&wallet, id).Error; err != nil {
		return nil, err
	}
	return &wallet, nil
}

func (s *Store) UpdateWallet(id int64, label string) (*Wallet, error) {
	wallet, err := s.GetWallet(id)
	if err != nil {
		return nil, err
	}
	wallet.Label = strings.TrimSpace(label)
	if err := s.db.Save(wallet).Error; err != nil {
		return nil, err
	}
	return wallet, nil
}

func (s *Store) DeleteWallet(id int64) error {
	return s.db.Delete(&Wallet{}, id).Error
}

func (s *Store) UpsertTokenAccounts(walletID int64, accounts []TokenAccount) error {
	if len(accounts) == 0 {
		return nil
	}
	now := time.Now()
	for i := range accounts {
		accounts[i].WalletID = walletID
		accounts[i].LastUpdated = now
	}
	return s.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "wallet_id"}, {Name: "mint_address"}},
		DoUpdates: clause.Assignments(map[string]any{
			"balance":      gorm.Expr("excluded.balance"),
			"token_name":   gorm.Expr("case when excluded.token_name <> '' then excluded.token_name else t_insider_token_account.token_name end"),
			"decimals":     gorm.Expr("excluded.decimals"),
			"last_updated": gorm.Expr("excluded.last_updated"),
		}),
	}).Create(&accounts).Error
}

func (s *Store) UpdateTokenUSDValue(id int64, value float64) error {
	return s.db.Model(&TokenAccount{}).Where("id = ?", id).Update("usd_value", value).Error
}

func (s *Store) TokenAccounts(walletID int64) ([]TokenAccount, error) {
	var accounts []TokenAccount
	err := s.db.Where("wallet_id = ?", walletID).Order("usd_value desc").Find(&accounts).Error
	return accounts, err
}

func (s *Store) InsertPrices(records []PriceRecord) error {
	if len(records) == 0 {
		return nil
	}
	return s.db.Create(&records).Error
}

func (s *Store) LatestPrices(mints []string) (map[string]float64, error) {
	prices := map[string]float64{}
	if len(mints) == 0 {
		return prices, nil
	}
	for _, mint := range mints {
		var rec PriceRecord
		err := s.db.Where("mint_address = ?", mint).Order("recorded_at desc").First(&rec).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		prices[mint] = rec.PriceUSD
	}
	return prices, nil
}

func (s *Store) InsertTransactions(txs []Transaction) (int64, error) {
	if len(txs) == 0 {
		return 0, nil
	}
	result := s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "signature"}, {Name: "mint_address"}, {Name: "tx_type"}},
		DoNothing: true,
	}).Create(&txs)
	return result.RowsAffected, result.Error
}

func (s *Store) Transactions(walletID int64, limit, offset int) ([]Transaction, error) {
	var txs []Transaction
	err := s.db.Where("wallet_id = ?", walletID).Order("block_time desc").Limit(limit).Offset(offset).Find(&txs).Error
	return txs, err
}

func (s *Store) LatestSignature(walletID int64) (string, error) {
	var tx Transaction
	err := s.db.Where("wallet_id = ?", walletID).Order("block_time desc").First(&tx).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	return tx.Signature, err
}

type MintPnL struct {
	MintAddress string
	TokenName   string
	TotalBought float64
	TotalSold   float64
	BuyCost     float64
	SellRevenue float64
	RealizedPnL float64
}

func (s *Store) MintPnL(walletID int64, since *time.Time) ([]MintPnL, error) {
	query := s.db.Model(&Transaction{}).Select(`
		mint_address,
		max(token_name) as token_name,
		sum(case when tx_type = 'buy' then amount else 0 end) as total_bought,
		sum(case when tx_type = 'sell' then amount else 0 end) as total_sold,
		sum(case when tx_type = 'buy' then sol_amount else 0 end) as buy_cost,
		sum(case when tx_type = 'sell' then sol_amount else 0 end) as sell_revenue
	`).Where("wallet_id = ?", walletID)
	if since != nil {
		query = query.Where("block_time >= ?", *since)
	}
	var rows []MintPnL
	if err := query.Group("mint_address").Find(&rows).Error; err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].RealizedPnL = rows[i].SellRevenue - rows[i].BuyCost
	}
	return rows, nil
}

func (s *Store) CountTransactions(walletID int64, txType string, since *time.Time) (int64, error) {
	query := s.db.Model(&Transaction{}).Where("wallet_id = ?", walletID)
	if txType != "" {
		query = query.Where("tx_type = ?", txType)
	}
	if since != nil {
		query = query.Where("block_time >= ?", *since)
	}
	var count int64
	err := query.Count(&count).Error
	return count, err
}

func (s *Store) LastActive(walletID int64) (*time.Time, error) {
	var tx Transaction
	err := s.db.Where("wallet_id = ?", walletID).Order("block_time desc").First(&tx).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &tx.BlockTime, nil
}

func (s *Store) ListRules() ([]AlertRule, error) {
	var rules []AlertRule
	if err := s.db.Order("created_at desc").Find(&rules).Error; err != nil {
		return nil, err
	}
	for i := range rules {
		hydrateRule(&rules[i])
		if rules[i].WalletID != nil {
			wallet, err := s.GetWallet(*rules[i].WalletID)
			if err == nil {
				rules[i].Wallet = wallet
			}
		}
	}
	return rules, nil
}

func (s *Store) EnabledRules() ([]AlertRule, error) {
	var rules []AlertRule
	if err := s.db.Where("enabled = ?", true).Find(&rules).Error; err != nil {
		return nil, err
	}
	for i := range rules {
		hydrateRule(&rules[i])
	}
	return rules, nil
}

func (s *Store) GetRule(id int64) (*AlertRule, error) {
	var rule AlertRule
	if err := s.db.First(&rule, id).Error; err != nil {
		return nil, err
	}
	hydrateRule(&rule)
	return &rule, nil
}

func (s *Store) SaveRule(rule *AlertRule) error {
	prepareRule(rule)
	return s.db.Save(rule).Error
}

func (s *Store) ListHistory(limit, offset int) ([]AlertHistory, error) {
	var rows []AlertHistory
	if err := s.db.Order("created_at desc").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, err
	}
	for i := range rows {
		hydrateHistory(&rows[i])
		if rows[i].WalletID != nil {
			wallet, err := s.GetWallet(*rows[i].WalletID)
			if err == nil {
				rows[i].Wallet = wallet
			}
		}
	}
	return rows, nil
}

func (s *Store) CreateHistory(history *AlertHistory) error {
	prepareHistory(history)
	return s.db.Create(history).Error
}

func (s *Store) LatestSnapshot(walletID int64) (*WalletSnapshot, error) {
	var snap WalletSnapshot
	err := s.db.Where("wallet_id = ?", walletID).Order("created_at desc").First(&snap).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &snap, err
}

func (s *Store) CreateSnapshot(walletID int64, total float64, mints []string) error {
	encoded, _ := json.Marshal(mints)
	return s.db.Create(&WalletSnapshot{
		WalletID:        walletID,
		TotalBalanceUSD: total,
		TokenMintsJSON:  string(encoded),
		CreatedAt:       time.Now(),
	}).Error
}

func (s *Store) ListChannels() ([]NotificationChannel, error) {
	var channels []NotificationChannel
	err := s.db.Order("created_at desc").Find(&channels).Error
	return channels, err
}

func (s *Store) GetChannel(id int64) (*NotificationChannel, error) {
	var channel NotificationChannel
	if err := s.db.First(&channel, id).Error; err != nil {
		return nil, err
	}
	return &channel, nil
}

func (s *Store) SaveChannel(channel *NotificationChannel) error {
	channel.ChannelType = strings.ToLower(strings.TrimSpace(channel.ChannelType))
	return s.db.Save(channel).Error
}

func hydrateRule(rule *AlertRule) {
	_ = json.Unmarshal([]byte(rule.ChannelIDsJSON), &rule.ChannelIDs)
	if rule.ChannelIDs == nil {
		rule.ChannelIDs = []int64{}
	}
}

func prepareRule(rule *AlertRule) {
	encoded, _ := json.Marshal(rule.ChannelIDs)
	rule.ChannelIDsJSON = string(encoded)
}

func hydrateHistory(history *AlertHistory) {
	if history.DataJSON == "" {
		history.Data = map[string]any{}
		return
	}
	_ = json.Unmarshal([]byte(history.DataJSON), &history.Data)
	if history.Data == nil {
		history.Data = map[string]any{}
	}
}

func prepareHistory(history *AlertHistory) {
	encoded, _ := json.Marshal(history.Data)
	history.DataJSON = string(encoded)
}

func mintsFromSnapshot(snapshot *WalletSnapshot) map[string]bool {
	result := map[string]bool{}
	if snapshot == nil || snapshot.TokenMintsJSON == "" {
		return result
	}
	var mints []string
	if json.Unmarshal([]byte(snapshot.TokenMintsJSON), &mints) == nil {
		for _, mint := range mints {
			if strings.TrimSpace(mint) != "" {
				result[mint] = true
			}
		}
	}
	return result
}
