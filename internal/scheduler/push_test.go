package scheduler

import (
	"strings"
	"testing"
	"time"

	"go-radar/internal/model"
	"go-radar/internal/scanners"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestDecidePushMatchesCriticalPolicies(t *testing.T) {
	resonance := &model.SignalEvent{Source: "system", SignalType: "resonance", Priority: "high"}
	if decision := decidePush(resonance, false); decision.channel != "immediate" || decision.cooldownExempt {
		t.Fatalf("unexpected resonance decision: %#v", decision)
	}

	s5MediumMomentum := &model.SignalEvent{Source: "s5", SignalType: "momentum", Priority: "medium"}
	if decision := decidePush(s5MediumMomentum, false); decision.channel != "immediate" || decision.quotaKey != "s5_momentum_medium" {
		t.Fatalf("unexpected s5 decision: %#v", decision)
	}

	s3Medium := &model.SignalEvent{Source: "s3", SignalType: "heat", Priority: "medium"}
	if decision := decidePush(s3Medium, false); decision.channel != "digest" {
		t.Fatalf("unexpected s3 decision: %#v", decision)
	}
}

func TestS5MediumMomentumQuotaUsesSetting(t *testing.T) {
	db := openSchedulerTestDB(t)
	s := New(db, true)

	if got := s.immediateQuotaLimit("s5_momentum_medium"); got != 1 {
		t.Fatalf("expected default quota 1, got %d", got)
	}

	db.Create(&model.AppSetting{Key: "s5_momentum_medium_quota", ValueJSON: "2"})
	if got := s.immediateQuotaLimit("s5_momentum_medium"); got != 2 {
		t.Fatalf("expected configured quota 2, got %d", got)
	}
}

func TestCreateResonanceSignalsForCrossSourceToken(t *testing.T) {
	db := openSchedulerTestDB(t)
	s := New(db, true)

	payloads := []scanners.SignalPayload{
		{Source: "s3", Chain: "bsc", Address: "0xabc", Symbol: "ABC", Name: "ABC", SignalType: "heat", Priority: "medium", Score: 70, Reason: "heat"},
		{Source: "s5", Chain: "bsc", Address: "0xabc", Symbol: "ABC", Name: "ABC", SignalType: "momentum", Priority: "medium", Score: 80, Reason: "momentum"},
	}
	var newSignals []*model.SignalEvent
	for _, payload := range payloads {
		stored, created, err := scanners.StoreSignalEvent(db, payload, 30)
		if err != nil {
			t.Fatalf("store signal: %v", err)
		}
		if created {
			newSignals = append(newSignals, stored)
		}
	}

	resonance, err := s.createResonanceSignals(newSignals)
	if err != nil {
		t.Fatalf("create resonance: %v", err)
	}
	if len(resonance) != 1 {
		t.Fatalf("expected one deduped resonance signal, got %d", len(resonance))
	}
	if resonance[0].Source != "system" || resonance[0].SignalType != "resonance" || resonance[0].Priority != "high" {
		t.Fatalf("unexpected resonance: %#v", resonance[0])
	}
}

func TestFindBlockingRecentPushHonorsPriorityUpgrade(t *testing.T) {
	db := openSchedulerTestDB(t)
	s := New(db, true)
	pushedAt := time.Now().UTC().Format(time.RFC3339Nano)
	db.Create(&model.SignalEvent{
		Source:     "s5",
		Chain:      "bsc",
		Address:    "0xabc",
		Symbol:     "ABC",
		SignalType: "momentum",
		Priority:   "medium",
		Score:      80,
		Reason:     "old",
		DedupeKey:  "old",
		CreatedAt:  pushedAt,
		PushedAt:   &pushedAt,
	})

	medium := &model.SignalEvent{Chain: "bsc", Address: "0xabc", Priority: "medium"}
	blocking, err := s.findBlockingRecentPush(medium, pushDecision{channel: "immediate"}, false, map[string]*model.SignalEvent{})
	if err != nil {
		t.Fatalf("find blocking: %v", err)
	}
	if blocking == nil {
		t.Fatalf("expected medium signal to be blocked")
	}

	high := &model.SignalEvent{Chain: "bsc", Address: "0xabc", Priority: "high"}
	blocking, err = s.findBlockingRecentPush(high, pushDecision{channel: "immediate"}, false, map[string]*model.SignalEvent{})
	if err != nil {
		t.Fatalf("find blocking: %v", err)
	}
	if blocking != nil {
		t.Fatalf("expected high priority upgrade to bypass cooldown")
	}
}

func TestFormatSignalMessageMatchesPythonTelegramStyle(t *testing.T) {
	createdAt := "2026-05-12T01:02:03Z"
	token := &model.TokenProfile{
		Chain:           "ethereum",
		Address:         "0x9f8f72aa9304c8b593d555f12ef6589cc3a579a2",
		Symbol:          "MKR",
		Name:            "Maker",
		SocialLinksJSON: `{"twitter":"https://x.example/mkr","telegram":"https://t.me/mkr","website":"https://makerdao.com"}`,
	}
	signal := &model.SignalEvent{
		Source:     "s7",
		Chain:      "ethereum",
		Address:    token.Address,
		Symbol:     "MKR",
		SignalType: "vitalik_sell",
		Priority:   "high",
		Score:      95,
		Reason:     "test reason",
		TagsJSON:   `["vitalik","dex"]`,
		RawJSON:    `{"recipient_type":"dex","recipient_name":"Uniswap","amount":1234.5,"usd_value":2500000,"price_usd":123.456,"tx_hash":"0xabc"}`,
		CreatedAt:  createdAt,
	}

	text := formatSignalMessage(signal, token)

	for _, want := range []string{
		"🐋 <b>S7 V神卖币</b>",
		"🚨 <b>Maker · V神卖币</b>",
		"⏰ 05-12 09:02",
		"🧭 路径: DEX -&gt; Uniswap",
		"💸 数量: 1,234.5000 MKR",
		"💰 估值: $2.50M",
		"📄 合约：<code>0x9f8f72aa9304c8b593d555f12ef6589cc3a579a2</code>",
		"🏷 S7 V神卖币 | #vitalik #dex",
		`🔎 交易: <a href="https://etherscan.io/tx/0xabc">etherscan</a>`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected message to contain %q, got:\n%s", want, text)
		}
	}
}

func TestFormatS3DigestMatchesPythonTelegramStyle(t *testing.T) {
	signals := []*model.SignalEvent{{
		Source:     "s3",
		Chain:      "binance_perp",
		Address:    "btcusdt",
		Symbol:     "BTC",
		SignalType: "heat_plus_oi",
		Priority:   "medium",
		Score:      82.5,
		Reason:     "heat and oi",
		RawJSON:    `{"px_chg":3.21,"oi_d6h":8.9,"funding_pct":-0.0123,"vol":123456789}`,
	}}

	text := formatS3Digest(signals)

	for _, want := range []string{
		"🔥 <b>S3 热度摘要</b>",
		"<i>10 分钟窗口内值得看的合约热度信号</i>",
		"<b>1. BTC</b> · 热度 + OI · <b>medium</b> (score 82.5)",
		"<pre>24h +3.2% | OI +8.9% | 费率 -0.012% | 成交额 $123.46M</pre>",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected digest to contain %q, got:\n%s", want, text)
		}
	}
}

func TestFormatResonanceSignalMessageMatchesScannerStyle(t *testing.T) {
	signal := &model.SignalEvent{
		Source:     "system",
		Chain:      "binance_perp",
		Address:    "crvusdt",
		Symbol:     "CRV",
		SignalType: "resonance",
		Priority:   "high",
		Score:      77.5,
		Reason:     "Cross-source resonance: s2, s3",
		TagsJSON:   `["resonance","s2","s3"]`,
		RawJSON:    `{"sources":["s2","s3"],"base_signal_id":42}`,
		CreatedAt:  "2026-05-12T09:36:00Z",
	}

	text := formatSignalMessage(signal, nil)

	for _, want := range []string{
		"🛰 <b>系统共振</b>",
		"⚡ <b>CRV · 跨源确认</b>",
		"⏰ 05-12 17:36",
		"🧭 来源: s2 + s3",
		"📊 强度: 2 路雷达   优先级: high",
		"🎯 分数: 77.5   市场: Binance 合约",
		"📝 <i>多个雷达来源同时命中同一标的，信号强度高于单一路径，适合优先复核。</i>",
		"🔎 跨源共振: s2, s3",
		"🏷 系统共振 | #resonance #s2 #s3",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected resonance message to contain %q, got:\n%s", want, text)
		}
	}
}

func TestCopyItemsForSignalsDedupesContracts(t *testing.T) {
	signals := []*model.SignalEvent{
		{Source: "s5", Address: "0x1111111111111111111111111111111111111111", Symbol: "AAA"},
		{Source: "system", Address: "0x1111111111111111111111111111111111111111", Symbol: "AAA"},
	}

	items := copyItemsForSignals(signals)

	if len(items) != 1 || items[0].Label != "复制 AAA" || items[0].Text != "0x1111111111111111111111111111111111111111" {
		t.Fatalf("unexpected copy items: %#v", items)
	}
}

func openSchedulerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.TokenProfile{}, &model.TokenSnapshot{}, &model.SignalEvent{}, &model.WatchlistItem{}, &model.ScannerRun{}, &model.AppSetting{}); err != nil {
		t.Fatalf("migrate schema: %v", err)
	}
	return db
}
