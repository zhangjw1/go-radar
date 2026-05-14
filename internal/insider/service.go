package insider

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

type Engine interface {
	Name() string
	Sync(ctx context.Context) error
	SyncBalances(ctx context.Context) error
	SyncPrices(ctx context.Context) error
	SyncTransactions(ctx context.Context) error
	EvaluateAlerts(ctx context.Context) error
}

type Service struct {
	store  *Store
	mu     sync.Mutex
	status SyncStatus
}

func NewService(store *Store) *Service {
	return &Service{store: store, status: SyncStatus{Engine: EngineService}}
}

func (s *Service) Status(engine string) SyncStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	status := s.status
	status.Engine = engine
	return status
}

func (s *Service) RunSync(ctx context.Context, engine Engine) error {
	s.mu.Lock()
	if s.status.Syncing {
		s.mu.Unlock()
		return fmt.Errorf("sync already in progress")
	}
	s.status.Syncing = true
	s.status.LastError = ""
	s.status.Engine = engine.Name()
	s.mu.Unlock()

	err := engine.Sync(ctx)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.Syncing = false
	if err != nil {
		s.status.LastError = err.Error()
	}
	return err
}

func (s *Service) CreateWallet(address, label string) (*Wallet, error) {
	address = strings.TrimSpace(address)
	if len(address) < 32 || len(address) > 64 {
		return nil, fmt.Errorf("invalid wallet address")
	}
	return s.store.CreateWallet(address, label)
}

func (s *Service) Portfolio(walletID int64) ([]TokenHolding, error) {
	accounts, err := s.store.TokenAccounts(walletID)
	if err != nil {
		return nil, err
	}
	mints := make([]string, 0, len(accounts))
	for _, account := range accounts {
		mints = append(mints, account.MintAddress)
	}
	prices, err := s.store.LatestPrices(mints)
	if err != nil {
		return nil, err
	}
	pnls, err := s.store.MintPnL(walletID, nil)
	if err != nil {
		return nil, err
	}
	pnlMap := map[string]MintPnL{}
	for _, pnl := range pnls {
		pnlMap[pnl.MintAddress] = pnl
	}
	holdings := make([]TokenHolding, 0, len(accounts))
	for _, account := range accounts {
		holding := TokenHolding{
			MintAddress: account.MintAddress,
			TokenName:   account.TokenName,
			Balance:     account.Balance,
			Decimals:    account.Decimals,
			USDValue:    account.USDValue,
		}
		if price, ok := prices[account.MintAddress]; ok {
			holding.CurrentValue = account.Balance * price
			holding.USDValue = holding.CurrentValue
		} else {
			holding.CurrentValue = account.USDValue
		}
		if pnl, ok := pnlMap[account.MintAddress]; ok {
			holding.TotalBought = pnl.TotalBought
			holding.TotalSold = pnl.TotalSold
			holding.Remaining = pnl.TotalBought - pnl.TotalSold
			holding.CostBasis = pnl.BuyCost
			holding.PnL = pnl.SellRevenue - pnl.BuyCost + holding.CurrentValue
			if holding.CostBasis > 0 {
				holding.PnLPercent = holding.PnL / holding.CostBasis * 100
			}
		}
		holdings = append(holdings, holding)
	}
	return holdings, nil
}

func (s *Service) Analytics(walletID int64, period string) (*Analytics, error) {
	var since *time.Time
	now := time.Now()
	switch period {
	case "7d":
		t := now.AddDate(0, 0, -7)
		since = &t
	case "30d":
		t := now.AddDate(0, 0, -30)
		since = &t
	default:
		period = "all"
	}
	pnls, err := s.store.MintPnL(walletID, since)
	if err != nil {
		return nil, err
	}
	analytics := &Analytics{WalletID: walletID, Period: period}
	closed := 0
	wins := 0
	for _, pnl := range pnls {
		analytics.RealizedPnL += pnl.RealizedPnL
		analytics.TotalCost += pnl.BuyCost
		if pnl.TotalSold > 0 {
			closed++
			if pnl.SellRevenue > pnl.BuyCost {
				wins++
			}
		}
	}
	if closed > 0 {
		analytics.WinRate = float64(wins) / float64(closed) * 100
	}
	holdings, err := s.Portfolio(walletID)
	if err != nil {
		return nil, err
	}
	for _, holding := range holdings {
		analytics.UnrealizedPnL += holding.CurrentValue
	}
	analytics.UnrealizedPnL -= analytics.TotalCost - analytics.RealizedPnL
	analytics.TotalPnL = analytics.RealizedPnL + analytics.UnrealizedPnL
	analytics.TxCount, _ = s.store.CountTransactions(walletID, "", since)
	analytics.BuyCount, _ = s.store.CountTransactions(walletID, "buy", since)
	analytics.SellCount, _ = s.store.CountTransactions(walletID, "sell", since)
	if len(pnls) > 0 {
		analytics.AvgCostPerToken = analytics.TotalCost / float64(len(pnls))
	}
	analytics.LastActiveTime, _ = s.store.LastActive(walletID)
	return analytics, nil
}

type EngineConfig struct {
	SolanaRPCURL  string
	HeliusAPIKey  string
	HeliusBaseURL string
	ScanMode      string
	IncludeTokens []string
	ExcludeTokens []string
}

func LoadEngineConfig() EngineConfig {
	return EngineConfig{
		SolanaRPCURL:  firstEnv("INSIDER_SOLANA_RPC_URL", "SOLANA_RPC_URL", "NETWORK_URL"),
		HeliusAPIKey:  firstEnv("INSIDER_HELIUS_API_KEY", "HELIUS_API_KEY"),
		HeliusBaseURL: firstEnv("INSIDER_HELIUS_BASE_URL", "HELIUS_BASE_URL"),
		ScanMode:      firstEnv("INSIDER_SCAN_MODE"),
		IncludeTokens: splitCSV(os.Getenv("INSIDER_INCLUDE_TOKENS")),
		ExcludeTokens: splitCSV(os.Getenv("INSIDER_EXCLUDE_TOKENS")),
	}
}

type ServiceEngine struct {
	store   *Store
	cfg     EngineConfig
	rpc     *RPCClient
	helius  *HeliusClient
	jupiter *JupiterClient
}

func NewServiceEngine(store *Store, cfg EngineConfig) *ServiceEngine {
	return &ServiceEngine{
		store:   store,
		cfg:     cfg,
		rpc:     NewRPCClient(cfg.SolanaRPCURL),
		helius:  NewHeliusClient(cfg.HeliusAPIKey, cfg.HeliusBaseURL),
		jupiter: NewJupiterClient(),
	}
}

func (e *ServiceEngine) Name() string { return EngineService }

func (e *ServiceEngine) Sync(ctx context.Context) error {
	if err := e.SyncBalances(ctx); err != nil {
		return err
	}
	if err := e.SyncPrices(ctx); err != nil {
		return err
	}
	if err := e.SyncTransactions(ctx); err != nil {
		return err
	}
	return e.EvaluateAlerts(ctx)
}

func (e *ServiceEngine) SyncBalances(ctx context.Context) error {
	wallets, err := e.store.ListWallets()
	if err != nil {
		return err
	}
	for _, wallet := range wallets {
		accounts, err := e.rpc.TokenAccounts(ctx, wallet.Address)
		if err != nil {
			return fmt.Errorf("sync wallet %s: %w", wallet.Address, err)
		}
		if sol, ok, err := e.rpc.SOLBalance(ctx, wallet.Address); err == nil && ok {
			accounts = append(accounts, sol)
		}
		filtered := make([]TokenAccount, 0, len(accounts))
		mints := make([]string, 0, len(accounts))
		for _, account := range accounts {
			if e.includeToken(account.MintAddress) {
				filtered = append(filtered, account)
				mints = append(mints, account.MintAddress)
			}
		}
		names := e.helius.TokenNames(ctx, mints)
		for i := range filtered {
			if filtered[i].TokenName == "" {
				filtered[i].TokenName = names[filtered[i].MintAddress]
			}
		}
		if err := e.store.UpsertTokenAccounts(wallet.ID, filtered); err != nil {
			return err
		}
	}
	return nil
}

func (e *ServiceEngine) SyncPrices(ctx context.Context) error {
	wallets, err := e.store.ListWallets()
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	allAccounts := []TokenAccount{}
	for _, wallet := range wallets {
		accounts, err := e.store.TokenAccounts(wallet.ID)
		if err != nil {
			return err
		}
		for _, account := range accounts {
			seen[account.MintAddress] = true
			allAccounts = append(allAccounts, account)
		}
	}
	mints := make([]string, 0, len(seen))
	for mint := range seen {
		mints = append(mints, mint)
	}
	prices, err := e.jupiter.Prices(ctx, mints)
	if err != nil {
		return err
	}
	now := time.Now()
	records := make([]PriceRecord, 0, len(prices))
	for mint, price := range prices {
		records = append(records, PriceRecord{MintAddress: mint, PriceUSD: price, Source: "jupiter", RecordedAt: now})
	}
	if err := e.store.InsertPrices(records); err != nil {
		return err
	}
	for _, account := range allAccounts {
		if price, ok := prices[account.MintAddress]; ok {
			_ = e.store.UpdateTokenUSDValue(account.ID, account.Balance*price)
		}
	}
	return nil
}

func (e *ServiceEngine) SyncTransactions(ctx context.Context) error {
	wallets, err := e.store.ListWallets()
	if err != nil {
		return err
	}
	for _, wallet := range wallets {
		latest, err := e.store.LatestSignature(wallet.ID)
		if err != nil {
			return err
		}
		raw, err := e.helius.Transactions(ctx, wallet.Address, latest)
		if err != nil {
			return err
		}
		_, err = e.store.InsertTransactions(ParseTransactions(wallet, raw))
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *ServiceEngine) EvaluateAlerts(ctx context.Context) error {
	return evaluateAlerts(ctx, e.store)
}

func (e *ServiceEngine) includeToken(mint string) bool {
	mode := strings.ToLower(strings.TrimSpace(e.cfg.ScanMode))
	if mode == "" || mode == "all" {
		return true
	}
	if mode == "whitelist" {
		return containsString(e.cfg.IncludeTokens, mint)
	}
	if mode == "blacklist" {
		return !containsString(e.cfg.ExcludeTokens, mint)
	}
	return true
}

type LegacyEngine struct {
	service *ServiceEngine
}

func NewLegacyEngine(store *Store, cfg EngineConfig) *LegacyEngine {
	return &LegacyEngine{service: NewServiceEngine(store, cfg)}
}

func (e *LegacyEngine) Name() string { return EngineLegacy }

func (e *LegacyEngine) Sync(ctx context.Context) error {
	if err := e.SyncBalances(ctx); err != nil {
		return err
	}
	return e.EvaluateAlerts(ctx)
}

func (e *LegacyEngine) SyncBalances(ctx context.Context) error {
	return e.service.SyncBalances(ctx)
}

func (e *LegacyEngine) SyncPrices(ctx context.Context) error {
	return nil
}

func (e *LegacyEngine) SyncTransactions(ctx context.Context) error {
	return nil
}

func (e *LegacyEngine) EvaluateAlerts(ctx context.Context) error {
	return evaluateAlerts(ctx, e.service.store)
}

func evaluateAlerts(ctx context.Context, store *Store) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	rules, err := store.EnabledRules()
	if err != nil {
		return err
	}
	wallets, err := store.ListWallets()
	if err != nil {
		return err
	}
	global := []AlertRule{}
	byWallet := map[int64][]AlertRule{}
	for _, rule := range rules {
		if rule.WalletID == nil {
			global = append(global, rule)
			continue
		}
		byWallet[*rule.WalletID] = append(byWallet[*rule.WalletID], rule)
	}
	for _, wallet := range wallets {
		applicable := append([]AlertRule{}, global...)
		applicable = append(applicable, byWallet[wallet.ID]...)
		accounts, err := store.TokenAccounts(wallet.ID)
		if err != nil {
			return err
		}
		total := 0.0
		currentMints := map[string]bool{}
		mints := []string{}
		for _, account := range accounts {
			total += account.USDValue
			if account.Balance > 0 {
				currentMints[account.MintAddress] = true
				mints = append(mints, account.MintAddress)
			}
		}
		prev, err := store.LatestSnapshot(wallet.ID)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		for _, rule := range applicable {
			switch rule.RuleType {
			case RuleBalanceChange:
				evaluateBalanceRule(store, wallet, rule, prev, total)
			case RuleNewToken:
				evaluateNewTokenRule(store, wallet, rule, prev, currentMints)
			}
		}
		_ = store.CreateSnapshot(wallet.ID, total, mints)
	}
	return nil
}

func evaluateBalanceRule(store *Store, wallet Wallet, rule AlertRule, prev *WalletSnapshot, total float64) {
	if prev == nil || prev.TotalBalanceUSD <= 0 || rule.Threshold <= 0 {
		return
	}
	change := (total - prev.TotalBalanceUSD) / prev.TotalBalanceUSD
	if math.Abs(change) < rule.Threshold {
		return
	}
	level := "warning"
	if math.Abs(change) >= rule.Threshold*2 {
		level = "critical"
	}
	msg := fmt.Sprintf("Wallet %s balance changed %.2f%% (%.2f -> %.2f USD)", wallet.LabelOrAddress(), change*100, prev.TotalBalanceUSD, total)
	ruleID := rule.ID
	_ = store.CreateHistory(&AlertHistory{
		WalletID:    &wallet.ID,
		AlertRuleID: &ruleID,
		AlertType:   RuleBalanceChange,
		Message:     msg,
		Level:       level,
		Data: map[string]any{
			"wallet_id":      wallet.ID,
			"wallet_address": wallet.Address,
			"wallet_label":   wallet.Label,
			"previous_usd":   prev.TotalBalanceUSD,
			"current_usd":    total,
			"change_ratio":   change,
			"threshold":      rule.Threshold,
		},
	})
}

func evaluateNewTokenRule(store *Store, wallet Wallet, rule AlertRule, prev *WalletSnapshot, current map[string]bool) {
	if prev == nil {
		return
	}
	previous := mintsFromSnapshot(prev)
	newMints := []string{}
	for mint := range current {
		if !previous[mint] {
			newMints = append(newMints, mint)
		}
	}
	if len(newMints) == 0 {
		return
	}
	ruleID := rule.ID
	msg := fmt.Sprintf("Wallet %s added %d token holdings", wallet.LabelOrAddress(), len(newMints))
	_ = store.CreateHistory(&AlertHistory{
		WalletID:    &wallet.ID,
		AlertRuleID: &ruleID,
		AlertType:   RuleNewToken,
		Message:     msg,
		Level:       "info",
		Data: map[string]any{
			"wallet_id":      wallet.ID,
			"wallet_address": wallet.Address,
			"wallet_label":   wallet.Label,
			"new_mints":      newMints,
			"count":          len(newMints),
		},
	})
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if strings.TrimSpace(os.Getenv(key)) != "" {
			return strings.TrimSpace(os.Getenv(key))
		}
	}
	return ""
}

func splitCSV(raw string) []string {
	items := []string{}
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			items = append(items, item)
		}
	}
	return items
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), needle) {
			return true
		}
	}
	return false
}
