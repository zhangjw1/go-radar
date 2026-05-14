package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"go-radar/internal/model"
	"go-radar/internal/scanners"
	radartelegram "go-radar/internal/telegram"

	"gorm.io/gorm"
)

var (
	bjLocation      = time.FixedZone("Asia/Shanghai", 8*60*60)
	evmAddressRE    = regexp.MustCompile(`^0x[a-fA-F0-9]{40}$`)
	base58AddressRE = regexp.MustCompile(`^[1-9A-HJ-NP-Za-km-z]{32,44}$`)
)

var s3TypeLabels = map[string]string{
	"heat":                          "热度信号",
	"heat_report":                   "S3 热度总结",
	"heat_plus_oi":                  "热度 + OI",
	"heat_plus_negative_funding":    "热度 + 负费率",
	"heat_plus_oi_negative_funding": "热度 + OI + 负费率",
	"oi_anomaly":                    "OI 异动",
}

var s3TypeSummaries = map[string]string{
	"heat":                          "热度开始聚集，值得放进观察名单继续盯。",
	"heat_report":                   "S3 每轮热度、资金费率和 OI 聚合总结。",
	"heat_plus_oi":                  "热度已经起来，未平仓总额同步增加，属于更强确认。",
	"heat_plus_negative_funding":    "热度有了，负费率说明空头拥挤，容易形成逼空逻辑。",
	"heat_plus_oi_negative_funding": "热度、持仓增长和负费率同时出现，是 S3 内部更强的组合确认。",
	"oi_anomaly":                    "未平仓总额变化很大，但未必有热度配合，需要更谨慎判断。",
}

var s5TypeLabels = map[string]string{
	"flap_support":     "FLAP 支撑",
	"narrative_tagged": "叙事命中",
	"momentum":         "连续动量",
}

var s5TypeSummaries = map[string]string{
	"flap_support":     "底部支撑或企稳形态出现，适合先放进观察名单。",
	"narrative_tagged": "命中了热点叙事标签，值得继续跟踪后续资金动作。",
	"momentum":         "连续上涨并达到动量阈值，属于链上更强确认。",
}

var s2TypeLabels = map[string]string{
	"funding_flip_oi_rising": "费率翻转 + OI",
}

var s2TypeSummaries = map[string]string{
	"funding_flip_oi_rising": "费率由正转负，且未平仓合约上升，常用于观察逼空环境。",
}

var s1TypeLabels = map[string]string{
	"alpha_discovery": "币安公告发现",
	"alpha_countdown": "Alpha 倒计时",
	"alpha_launch":    "Alpha 上线",
	"alpha_followup":  "Alpha 跟踪",
	"alpha_anomaly":   "Alpha 异动",
}

var s1TypeSummaries = map[string]string{
	"alpha_discovery": "来自 Binance 公告 / Alpha 事件的发现信号，偏事件驱动。",
	"alpha_countdown": "上线前关键时间窗口提醒。",
	"alpha_launch":    "项目进入上线窗口，记录首个市场快照。",
	"alpha_followup":  "上线后的 30 分钟节奏跟踪。",
	"alpha_anomaly":   "上线后价格或市值出现大幅异动。",
}

var s1KindLabels = map[string]string{
	"listing": "Will List",
	"airdrop": "HODLer Airdrop",
	"alpha":   "Binance Alpha",
	"other":   "公告信号",
}

var s7TypeLabels = map[string]string{
	"vitalik_sell": "V神卖币",
}

var s7TypeSummaries = map[string]string{
	"vitalik_sell": "监测到 V 神地址向疑似卖出路径转出代币，值得立即关注。",
}

var sourceLabels = map[string]string{
	"s7":     "S7 V神卖币",
	"s1":     "S1 币安公告",
	"s2":     "S2 费率翻转",
	"s3":     "S3 热度确认",
	"s5":     "S5 链上发现",
	"system": "系统共振",
}

var chainLabels = map[string]string{
	"binance_perp":  "Binance 合约",
	"binance_alpha": "币安公告",
	"eth":           "Ethereum",
	"ethereum":      "Ethereum",
	"bsc":           "BSC",
	"base":          "Base",
	"sol":           "Solana",
	"solana":        "Solana",
}

// pushDecision 是单条信号经过推送策略后的路由结果。
type pushDecision struct {
	channel        string // channel 表示发送通道：immediate 立即推送、digest 摘要推送、skip 不推送。
	quotaKey       string // quotaKey 表示该类信号共享的限额桶，空值表示不限额。
	cooldownExempt bool   // cooldownExempt 表示该信号是否跳过 token 级别冷却时间。
}

// createResonanceSignals 根据同一 token 的多来源信号生成 system/resonance 共振信号。
func (s *Scheduler) createResonanceSignals(baseSignals []*model.SignalEvent) ([]*model.SignalEvent, error) {
	created := []*model.SignalEvent{}
	for _, signal := range baseSignals {
		if signal == nil || signal.Source == "system" {
			continue
		}
		sources, err := s.activeSourcesForToken(signal.Chain, signal.Address, settingInt(s.db, "resonance_lookback_minutes", "RESONANCE_LOOKBACK_MINUTES", 360))
		if err != nil {
			return created, err
		}
		if len(sources) < 2 {
			continue
		}
		sort.Strings(sources)
		payload := scanners.SignalPayload{
			Source:     "system",
			Chain:      signal.Chain,
			Address:    signal.Address,
			Symbol:     signal.Symbol,
			Name:       signal.Symbol,
			SignalType: "resonance",
			Priority:   "high",
			Score:      computeResonanceScore(len(sources), signal.Priority, signal.Score),
			Reason:     "跨源共振: " + strings.Join(sources, ", "),
			Tags:       append([]string{"resonance"}, sources...),
			Raw:        map[string]any{"sources": sources, "base_signal_id": signal.ID},
		}
		stored, wasCreated, err := scanners.StoreSignalEvent(s.db, payload, settingInt(s.db, "signal_time_bucket_minutes", "SIGNAL_TIME_BUCKET_MINUTES", 30))
		if err != nil {
			return created, err
		}
		if wasCreated && stored != nil {
			created = append(created, stored)
		}
	}
	return created, nil
}

// pushSignals 对新信号执行推送策略，并返回已成功发送的 signal id。
func (s *Scheduler) pushSignals(ctx context.Context, baseSignals []*model.SignalEvent, resonanceSignals []*model.SignalEvent, scannerName string) ([]int64, error) {
	notifier, err := s.telegramNotifier()
	if err != nil {
		return nil, err
	}
	if !notifier.Enabled() {
		return nil, nil
	}

	pushedIDs := []int64{}
	recentPushes := map[string]*model.SignalEvent{}
	resonanceTokens := map[string]bool{}
	for _, signal := range resonanceSignals {
		if signal != nil {
			resonanceTokens[tokenKey(signal.Chain, signal.Address)] = true
		}
	}

	quotaCounts := map[string]int{}
	orderedSignals := append([]*model.SignalEvent{}, resonanceSignals...)
	orderedSignals = append(orderedSignals, baseSignals...)
	for _, signal := range orderedSignals {
		if signal == nil {
			continue
		}
		if signal.Source != "system" && resonanceTokens[tokenKey(signal.Chain, signal.Address)] {
			continue
		}
		isWatchlisted, err := s.isWatchlisted(signal.Chain, signal.Address)
		if err != nil {
			return pushedIDs, err
		}
		decision := decidePush(signal, isWatchlisted)
		if decision.channel != "immediate" {
			continue
		}
		if decision.quotaKey != "" {
			limit := s.immediateQuotaLimit(decision.quotaKey)
			if limit > 0 && quotaCounts[decision.quotaKey] >= limit {
				continue
			}
		}
		blocking, err := s.findBlockingRecentPush(signal, decision, isWatchlisted, recentPushes)
		if err != nil {
			return pushedIDs, err
		}
		if blocking != nil {
			logInfo("suppressed repeat push for %s on %s after %s/%s", signal.Symbol, signal.Source, blocking.Source, blocking.SignalType)
			continue
		}
		token := s.tokenForSignal(signal)
		if err := notifier.SendText(ctx, formatSignalMessage(signal, token), copyItemsForSignal(signal, token)...); err != nil {
			return pushedIDs, err
		}
		if decision.quotaKey != "" {
			quotaCounts[decision.quotaKey]++
		}
		pushedIDs = append(pushedIDs, signal.ID)
		recentPushes[tokenKey(signal.Chain, signal.Address)] = signal
	}

	if scannerName == "s3" && !hasForcePushedSignal(baseSignals) {
		digestIDs, err := s.maybeSendS3Digest(ctx, notifier, pushedIDs, recentPushes)
		if err != nil {
			return pushedIDs, err
		}
		pushedIDs = append(pushedIDs, digestIDs...)
	}
	return pushedIDs, nil
}

// markSignalsPushed 将已发送成功的信号标记 pushed_at。
func hasForcePushedSignal(signals []*model.SignalEvent) bool {
	for _, signal := range signals {
		if signal != nil && rawBool(parseRaw(signal.RawJSON)["force_push"]) {
			return true
		}
	}
	return false
}

func (s *Scheduler) markSignalsPushed(signalIDs []int64) error {
	if len(signalIDs) == 0 {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	return s.db.Model(&model.SignalEvent{}).Where("id IN ?", signalIDs).Update("pushed_at", now).Error
}

// maybeSendS3Digest 在冷却期满足时，把 S3 medium 信号合并成摘要推送。
func (s *Scheduler) maybeSendS3Digest(ctx context.Context, notifier *radartelegram.Notifier, excludedIDs []int64, recentPushes map[string]*model.SignalEvent) ([]int64, error) {
	lastDigest, err := s.lastS3DigestTime()
	if err != nil {
		return nil, err
	}
	cooldown := settingInt(s.db, "s3_digest_cooldown_minutes", "S3_DIGEST_COOLDOWN_MINUTES", 10)
	if lastDigest != nil && time.Since(*lastDigest) < time.Duration(cooldown)*time.Minute {
		return nil, nil
	}

	since := time.Now().UTC().Add(-time.Duration(cooldown) * time.Minute)
	if lastDigest != nil {
		since = *lastDigest
	}
	var signals []model.SignalEvent
	query := s.db.Where("source = ? AND pushed_at IS NULL AND created_at >= ? AND priority IN ?", "s3", since.Format(time.RFC3339Nano), []string{"high", "medium"}).
		Order("score desc, created_at desc").
		Limit(40)
	if len(excludedIDs) > 0 {
		query = query.Where("id NOT IN ?", excludedIDs)
	}
	if err := query.Find(&signals).Error; err != nil {
		return nil, err
	}

	digestSignals := []*model.SignalEvent{}
	seenTokens := map[string]bool{}
	for i := range signals {
		signal := &signals[i]
		key := tokenKey(signal.Chain, signal.Address)
		if seenTokens[key] {
			continue
		}
		seenTokens[key] = true
		isWatchlisted, err := s.isWatchlisted(signal.Chain, signal.Address)
		if err != nil {
			return nil, err
		}
		decision := decidePush(signal, isWatchlisted)
		if decision.channel != "digest" {
			continue
		}
		blocking, err := s.findBlockingRecentPush(signal, decision, isWatchlisted, recentPushes)
		if err != nil {
			return nil, err
		}
		if blocking != nil {
			continue
		}
		digestSignals = append(digestSignals, signal)
		if len(digestSignals) >= 5 {
			break
		}
	}
	if len(digestSignals) == 0 {
		return nil, nil
	}
	if err := notifier.SendText(ctx, formatS3Digest(digestSignals), copyItemsForSignals(digestSignals)...); err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(digestSignals))
	for _, signal := range digestSignals {
		ids = append(ids, signal.ID)
	}
	return ids, nil
}

// activeSourcesForToken 查询某个 token 在回看窗口内触发过信号的来源列表。
func (s *Scheduler) activeSourcesForToken(chain string, address string, lookbackMinutes int) ([]string, error) {
	cutoff := time.Now().UTC().Add(-time.Duration(lookbackMinutes) * time.Minute).Format(time.RFC3339Nano)
	var sources []string
	err := s.db.Model(&model.SignalEvent{}).
		Distinct("source").
		Where("chain = ? AND address = ? AND created_at >= ? AND source != ?", strings.ToLower(chain), scanners.NormalizeAddress(address), cutoff, "system").
		Order("source").
		Pluck("source", &sources).Error
	return sources, err
}

// isWatchlisted 判断 token 是否在观察名单中。
func (s *Scheduler) isWatchlisted(chain string, address string) (bool, error) {
	var item model.WatchlistItem
	err := s.db.Where("chain = ? AND address = ?", strings.ToLower(chain), scanners.NormalizeAddress(address)).First(&item).Error
	if err == nil {
		return true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return false, err
}

// findBlockingRecentPush 判断是否存在冷却期内更高或相同优先级的已推送信号。
func (s *Scheduler) findBlockingRecentPush(signal *model.SignalEvent, decision pushDecision, isWatchlisted bool, recentPushes map[string]*model.SignalEvent) (*model.SignalEvent, error) {
	if decision.cooldownExempt {
		return nil, nil
	}
	cooldown := settingInt(s.db, "token_push_cooldown_minutes", "TOKEN_PUSH_COOLDOWN_MINUTES", 180)
	if isWatchlisted {
		cooldown = settingInt(s.db, "watchlist_cooldown_minutes", "WATCHLIST_COOLDOWN_MINUTES", 30)
	}
	if cooldown <= 0 {
		return nil, nil
	}
	if recent := recentPushes[tokenKey(signal.Chain, signal.Address)]; recent != nil {
		if priorityValue(signal.Priority) <= priorityValue(recent.Priority) {
			return recent, nil
		}
		return nil, nil
	}
	cutoff := time.Now().UTC().Add(-time.Duration(cooldown) * time.Minute).Format(time.RFC3339Nano)
	var recent model.SignalEvent
	err := s.db.Where("chain = ? AND address = ? AND pushed_at IS NOT NULL AND pushed_at >= ?", signal.Chain, signal.Address, cutoff).
		Order("pushed_at desc").
		First(&recent).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if priorityValue(signal.Priority) > priorityValue(recent.Priority) {
		return nil, nil
	}
	return &recent, nil
}

// lastS3DigestTime 查询最近一次 S3 摘要推送时间。
func (s *Scheduler) lastS3DigestTime() (*time.Time, error) {
	var signal model.SignalEvent
	err := s.db.Where("source = ? AND pushed_at IS NOT NULL", "s3").Order("pushed_at desc").First(&signal).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if signal.PushedAt == nil {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, *signal.PushedAt)
	if err != nil {
		return nil, nil
	}
	return &parsed, nil
}

// telegramNotifier 从 settings 表和环境变量合成 Telegram 推送器配置。
func (s *Scheduler) telegramNotifier() (*radartelegram.Notifier, error) {
	overrides := s.settingsMap()
	proxyURL := effectiveString(overrides, "tg_proxy_url")
	if proxyURL == "" {
		proxyURL = effectiveString(overrides, "selective_proxy_url")
	}
	return radartelegram.New(radartelegram.Settings{
		BotToken: effectiveString(overrides, "tg_bot_token"),
		ChatID:   effectiveString(overrides, "tg_chat_id"),
		ProxyURL: proxyURL,
		TrustEnv: effectiveBool(overrides, "http_trust_env"),
	})
}

// tokenForSignal 读取信号关联的 token 基础资料，用于 Telegram 消息展示。
func (s *Scheduler) tokenForSignal(signal *model.SignalEvent) *model.TokenProfile {
	if signal == nil || signal.TokenID == nil {
		return nil
	}
	var token model.TokenProfile
	if err := s.db.First(&token, *signal.TokenID).Error; err != nil {
		return nil
	}
	return &token
}

// settingsMap 将 settings 表转换成 key-value map，供推送策略读取运行时配置。
func (s *Scheduler) settingsMap() map[string]any {
	var rows []model.AppSetting
	if err := s.db.Find(&rows).Error; err != nil {
		return map[string]any{}
	}
	result := make(map[string]any, len(rows))
	for _, row := range rows {
		var value any
		if json.Unmarshal([]byte(row.ValueJSON), &value) != nil {
			value = row.ValueJSON
		}
		result[row.Key] = value
	}
	return result
}

// decidePush 根据来源、优先级、观察名单状态和 raw 数据决定推送通道。
func decidePush(signal *model.SignalEvent, isWatchlisted bool) pushDecision {
	raw := parseRaw(signal.RawJSON)
	if rawBool(raw["force_push"]) {
		return pushDecision{channel: "immediate", cooldownExempt: true}
	}
	if signal.Source == "system" && signal.SignalType == "resonance" {
		return pushDecision{channel: "immediate"}
	}
	if signal.Source == "s7" {
		if signal.Priority == "high" {
			return pushDecision{channel: "immediate"}
		}
		if isWatchlisted && rawFloat(raw["usd_value"]) >= 100_000 {
			return pushDecision{channel: "immediate"}
		}
		return pushDecision{channel: "skip"}
	}
	if signal.Source == "s5" {
		if signal.SignalType == "momentum" {
			if signal.Priority == "high" {
				return pushDecision{channel: "immediate"}
			}
			if signal.Priority == "medium" {
				return pushDecision{channel: "immediate", quotaKey: "s5_momentum_medium"}
			}
		}
		if isWatchlisted && signal.Priority == "high" {
			return pushDecision{channel: "immediate"}
		}
		return pushDecision{channel: "skip"}
	}
	if signal.Source == "s3" {
		if signal.Priority == "high" {
			return pushDecision{channel: "immediate"}
		}
		if signal.Priority == "medium" {
			return pushDecision{channel: "digest"}
		}
		return pushDecision{channel: "skip"}
	}
	if signal.Source == "s2" {
		if signal.Priority == "high" || (isWatchlisted && signal.Priority == "medium") {
			return pushDecision{channel: "immediate"}
		}
		return pushDecision{channel: "skip"}
	}
	if signal.Source == "s1" {
		if signal.Priority == "high" && strings.EqualFold(fmt.Sprint(raw["announcement_kind"]), "listing") {
			return pushDecision{channel: "immediate"}
		}
		if isWatchlisted && signal.Priority == "high" {
			return pushDecision{channel: "immediate"}
		}
		return pushDecision{channel: "skip"}
	}
	if isWatchlisted && signal.Priority == "high" {
		return pushDecision{channel: "immediate"}
	}
	return pushDecision{channel: "skip"}
}

// immediateQuotaLimit 返回立即推送通道的单轮限额。
func (s *Scheduler) immediateQuotaLimit(key string) int {
	if key == "s5_momentum_medium" {
		return settingInt(s.db, "s5_momentum_medium_quota", "S5_MOMENTUM_MEDIUM_QUOTA", 1)
	}
	return 0
}

// computeResonanceScore 根据来源数量、原信号优先级和分数计算共振分。
func computeResonanceScore(sourceCount int, priority string, baseScore float64) float64 {
	return round2(baseScore + float64(sourceCount*15) + float64(priorityValue(priority)*5))
}

// priorityValue 将 high/medium/low 转换为可比较的数值。
func priorityValue(priority string) int {
	switch priority {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

// formatSignalMessage 生成 Telegram 单条信号 HTML 文本。
func formatSignalMessage(signal *model.SignalEvent, token *model.TokenProfile) string {
	if signal.Source == "system" && signal.SignalType == "resonance" {
		return formatResonanceSignalMessage(signal, token)
	}
	switch signal.Source {
	case "s7":
		return formatS7SignalMessage(signal, token)
	case "s3":
		return formatS3SignalMessage(signal)
	case "s5":
		return formatS5SignalMessage(signal, token)
	case "s2":
		return formatS2SignalMessage(signal, token)
	case "s1":
		return formatS1SignalMessage(signal, token)
	}

	tags := parseStringList(signal.TagsJSON)
	raw := parseRaw(signal.RawJSON)
	tokenLinks := parseRaw("{}")
	if token != nil {
		tokenLinks = parseRaw(token.SocialLinksJSON)
	}
	title := signal.Symbol
	if token != nil && strings.TrimSpace(token.Name) != "" {
		title = token.Name
	}
	sourceLabel := labelOr(sourceLabels, signal.Source, strings.ToUpper(signal.Source))
	chainLabel := labelOr(chainLabels, signal.Chain, signal.Chain)
	tagLine := ""
	if len(tags) > 0 {
		tagLine = "\n标签：<code>" + html.EscapeString(tagText(tags, "")) + "</code>"
	}
	linkLine := ""
	if len(tokenLinks) > 0 {
		links := []string{}
		for _, key := range []string{"twitter", "telegram", "website", "etherscan", "dexscreener"} {
			if value := rawString(tokenLinks[key]); value != "" {
				links = append(links, fmt.Sprintf("%s: %s", key, value))
			}
		}
		if len(links) > 0 {
			if len(links) > 3 {
				links = links[:3]
			}
			linkLine = "\n" + html.EscapeString(strings.Join(links, " | "))
		}
	}
	metrics := []string{}
	if raw["mc"] != nil {
		metrics = append(metrics, "市值   $"+formatCompactNumber(raw["mc"]))
	}
	if raw["liq"] != nil {
		metrics = append(metrics, "流动性 $"+formatCompactNumber(raw["liq"]))
	}
	if raw["oi_d6h"] != nil {
		metrics = append(metrics, "OI "+fmtSigned(raw["oi_d6h"], 1, "%"))
	}
	if raw["funding_pct"] != nil {
		metrics = append(metrics, "费率 "+fmtSigned(raw["funding_pct"], 3, "%"))
	}
	metricBlock := ""
	if len(metrics) > 0 {
		metricBlock = "<pre>" + html.EscapeString(strings.Join(metrics, " | ")) + "</pre>"
	}
	return fmt.Sprintf(
		"🔔 <b>%s</b> · <b>%s</b>\n<i>%s</i>\n\n%s<b>优先级</b> %s   <b>分数</b> %.1f\n<b>市场</b> %s%s%s%s",
		html.EscapeString(sourceLabel),
		html.EscapeString(title),
		html.EscapeString(signal.Reason),
		metricBlock,
		html.EscapeString(signal.Priority),
		signal.Score,
		html.EscapeString(chainLabel),
		formatContractLine(signal, token),
		tagLine,
		linkLine,
	)
}

func formatResonanceSignalMessage(signal *model.SignalEvent, token *model.TokenProfile) string {
	raw := parseRaw(signal.RawJSON)
	tags := parseStringList(signal.TagsJSON)
	sources := rawStringSlice(raw["sources"])
	if len(sources) == 0 {
		for _, tag := range tags {
			if strings.HasPrefix(tag, "s") && tag != "system" {
				sources = append(sources, tag)
			}
		}
	}
	sort.Strings(sources)
	sourceText := "-"
	if len(sources) > 0 {
		sourceText = strings.Join(sources, " + ")
	}
	title := signal.Symbol
	if token != nil && strings.TrimSpace(token.Name) != "" {
		title = token.Name
	}
	chainLabel := labelOr(chainLabels, signal.Chain, signal.Chain)
	summary := "多个雷达来源同时命中同一标的，信号强度高于单一路径，适合优先复核。"
	lines := []string{
		"🛰 <b>系统共振</b>",
		fmt.Sprintf("⚡ <b>%s · 跨源确认</b>", html.EscapeString(title)),
		"⏰ " + formatBJTime(signal.CreatedAt),
		"",
	}
	lines = appendEscapedLines(lines,
		fmt.Sprintf("🧭 来源: %s", sourceText),
		fmt.Sprintf("📊 强度: %d 路雷达   优先级: %s", len(sources), signal.Priority),
		fmt.Sprintf("🎯 分数: %.1f   市场: %s", signal.Score, chainLabel),
	)
	if contractLine := formatContractLine(signal, token); contractLine != "" {
		lines = append(lines, contractLine)
	}
	lines = append(lines,
		"",
		"📝 <i>"+html.EscapeString(summary)+"</i>",
		"🔎 "+html.EscapeString(resonanceReasonText(signal.Reason, sources)),
		"🏷 系统共振 | "+html.EscapeString(tagText(tags, "#resonance")),
	)
	return strings.Join(lines, "\n")
}

// formatS3Digest 生成 S3 摘要推送的 HTML 文本。
func formatS3Digest(signals []*model.SignalEvent) string {
	lines := []string{"🔥 <b>S3 热度摘要</b>", "<i>10 分钟窗口内值得看的合约热度信号</i>", ""}
	for idx, signal := range signals {
		raw := parseRaw(signal.RawJSON)
		lines = append(lines, fmt.Sprintf(
			"<b>%d. %s</b> · %s · <b>%s</b> (score %.1f)",
			idx+1,
			html.EscapeString(signal.Symbol),
			html.EscapeString(labelOr(s3TypeLabels, signal.SignalType, signal.SignalType)),
			html.EscapeString(signal.Priority),
			signal.Score,
		))
		lines = append(lines, "<i>"+html.EscapeString(signal.Reason)+"</i>")
		metricParts := []string{}
		if raw["px_chg"] != nil {
			metricParts = append(metricParts, "24h "+fmtSigned(raw["px_chg"], 1, "%"))
		}
		if raw["oi_d6h"] != nil {
			metricParts = append(metricParts, "OI "+fmtSigned(raw["oi_d6h"], 1, "%"))
		}
		if raw["funding_pct"] != nil {
			metricParts = append(metricParts, "费率 "+fmtSigned(raw["funding_pct"], 3, "%"))
		}
		if raw["vol"] != nil {
			metricParts = append(metricParts, "成交额 $"+formatCompactNumber(raw["vol"]))
		}
		if len(metricParts) > 0 {
			lines = append(lines, "<pre>"+html.EscapeString(strings.Join(metricParts, " | "))+"</pre>")
		}
		if contractLine := strings.TrimSpace(formatContractLine(signal, nil)); contractLine != "" {
			lines = append(lines, contractLine)
		}
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func formatS3SignalMessage(signal *model.SignalEvent) string {
	raw := parseRaw(signal.RawJSON)
	if signal.SignalType == "heat_report" {
		report := rawString(raw["report_text"])
		if report == "" {
			report = signal.Reason
		}
		return "🔥 <b>S3 热度做多雷达</b>\n<pre>" + html.EscapeString(report) + "</pre>"
	}
	tags := parseStringList(signal.TagsJSON)
	signalLabel := labelOr(s3TypeLabels, signal.SignalType, signal.SignalType)
	summary := labelOr(s3TypeSummaries, signal.SignalType, signal.Reason)
	metricLines := []string{
		fmt.Sprintf("💰 价格: %s   24h: %s", formatPrice(raw["price"]), fmtSigned(raw["px_chg"], 1, "%")),
		fmt.Sprintf("📉 费率: %s   📊 OI: %s", fmtSigned(raw["funding_pct"], 3, "%"), fmtSigned(raw["oi_d6h"], 1, "%")),
		fmt.Sprintf("🌐 成交额: $%s   市值: $%s", formatCompactNumber(raw["vol"]), formatCompactNumber(raw["est_mcap"])),
		"🧾 现货: " + boolCN(raw["has_spot"]),
	}
	if bias := rawString(raw["funding_bias_label"]); bias != "" {
		metricLines = append(metricLines, "费率方向: "+bias)
	}
	if heat, ok := rawFloatOK(raw["heat"]); ok {
		metricLines[3] += fmt.Sprintf("   热度分: %.0f", heat)
	}
	if raw["oi_usd"] != nil {
		metricLines = append(metricLines, "📦 未平仓: $"+formatCompactNumber(raw["oi_usd"]))
	}
	lines := []string{
		"🔥 <b>S3 热度确认</b>",
		fmt.Sprintf("🚀 <b>%s · %s</b>", html.EscapeString(signal.Symbol), html.EscapeString(signalLabel)),
		"⏰ " + formatBJTime(signal.CreatedAt),
		"",
	}
	lines = appendEscapedLines(lines, metricLines...)
	if contractLine := formatContractLine(signal, nil); contractLine != "" {
		lines = append(lines, contractLine)
	}
	lines = append(lines,
		"",
		"📝 <i>"+html.EscapeString(summary)+"</i>",
		"🔎 "+html.EscapeString(signal.Reason),
		"🏷 S3 热度确认 | "+html.EscapeString(tagText(tags, "#s3")),
	)
	return strings.Join(lines, "\n")
}

func formatS7SignalMessage(signal *model.SignalEvent, token *model.TokenProfile) string {
	raw := parseRaw(signal.RawJSON)
	tags := parseStringList(signal.TagsJSON)
	signalLabel := labelOr(s7TypeLabels, signal.SignalType, signal.SignalType)
	summary := labelOr(s7TypeSummaries, signal.SignalType, signal.Reason)
	tokenName := signal.Symbol
	if token != nil && strings.TrimSpace(token.Name) != "" {
		tokenName = token.Name
	}
	recipientType := mapLabel(map[string]string{"dex": "DEX", "cex": "CEX", "pool": "LP 池"}, strings.ToLower(rawString(raw["recipient_type"])), strings.ToUpper(rawStringDefault(raw["recipient_type"], "unknown")))
	recipientName := rawStringDefault(raw["recipient_name"], "-")
	amountLine := "💸 数量: " + signal.Symbol
	if amount, ok := rawFloatOK(raw["amount"]); ok {
		amountLine = fmt.Sprintf("💸 数量: %s %s", formatFloatComma(amount, 4), signal.Symbol)
	}
	usdLine := "💰 估值: 未知"
	if raw["usd_value"] != nil {
		usdLine = "💰 估值: $" + formatCompactNumber(raw["usd_value"])
	}
	priceLine := "📈 价格: 未知"
	if raw["price"] != nil {
		priceLine = "📈 价格: " + formatPrice(raw["price"])
	} else if raw["price_usd"] != nil {
		priceLine = "📈 价格: " + formatPrice(raw["price_usd"])
	}
	lines := []string{
		"🐋 <b>S7 V神卖币</b>",
		fmt.Sprintf("🚨 <b>%s · %s</b>", html.EscapeString(tokenName), html.EscapeString(signalLabel)),
		"⏰ " + formatBJTime(signal.CreatedAt),
		"",
	}
	lines = appendEscapedLines(lines,
		"🧭 路径: "+recipientType+" -> "+recipientName,
		amountLine,
		usdLine,
		priceLine,
	)
	if contractLine := formatContractLine(signal, token); contractLine != "" {
		lines = append(lines, contractLine)
	}
	lines = append(lines,
		"",
		"📝 <i>"+html.EscapeString(summary)+"</i>",
		"📍 "+html.EscapeString(signal.Reason),
		"🏷 S7 V神卖币 | "+html.EscapeString(tagText(tags, "#s7")),
	)
	if txURL := rawString(raw["etherscan_url"]); txURL != "" {
		lines = append(lines, fmt.Sprintf(`🔎 交易: <a href="%s">etherscan</a>`, html.EscapeString(txURL)))
	} else if txHash := rawString(raw["tx_hash"]); strings.HasPrefix(txHash, "0x") {
		lines = append(lines, fmt.Sprintf(`🔎 交易: <a href="%s">etherscan</a>`, html.EscapeString("https://etherscan.io/tx/"+txHash)))
	}
	if chartURL := rawString(raw["dexscreener_url"]); chartURL != "" {
		lines = append(lines, fmt.Sprintf(`📊 图表: <a href="%s">dexscreener</a>`, html.EscapeString(chartURL)))
	} else if address := extractCopyableAddress(signal, token); address != "" {
		lines = append(lines, fmt.Sprintf(`📊 图表: <a href="%s">dexscreener</a>`, html.EscapeString("https://dexscreener.com/ethereum/"+address)))
	}
	return strings.Join(lines, "\n")
}

func formatS5SignalMessage(signal *model.SignalEvent, token *model.TokenProfile) string {
	raw := parseRaw(signal.RawJSON)
	tags := parseStringList(signal.TagsJSON)
	signalLabel := labelOr(s5TypeLabels, signal.SignalType, signal.SignalType)
	summary := labelOr(s5TypeSummaries, signal.SignalType, signal.Reason)
	chainLabel := labelOr(chainLabels, signal.Chain, signal.Chain)
	safety := rawMap(raw["safety"])
	momentum := rawMap(raw["momentum"])
	social := map[string]any{}
	if token != nil {
		social = parseRaw(token.SocialLinksJSON)
	}
	metricLines := []string{fmt.Sprintf("🧭 链: %s   优先级: %s", chainLabel, signal.Priority)}
	if raw["buy_ratio"] != nil {
		metricLines = append(metricLines, "🧲 买卖比: "+formatFixed(raw["buy_ratio"], 2))
	}
	if pctGain := firstPresent(momentum, "pct_gain", "PctGain"); pctGain != nil {
		metricLines = append(metricLines, "📈 动量: +"+formatFixed(pctGain, 1)+"%")
	}
	if len(safety) > 0 {
		line := "🛡 安全: 失败"
		if rawBool(safety["safe"]) {
			line = "🛡 安全: 通过"
		}
		if safety["sell_tax"] != nil {
			line += "   卖税: " + formatFixed(safety["sell_tax"], 2)
		}
		metricLines = append(metricLines, line)
	}
	socialNames := []string{}
	for _, pair := range []struct{ key, name string }{{"twitter", "Twitter"}, {"telegram", "TG"}, {"website", "Web"}} {
		if rawString(social[pair.key]) != "" {
			socialNames = append(socialNames, pair.name)
		}
	}
	if len(socialNames) > 0 {
		metricLines = append(metricLines, "🔗 社区: "+strings.Join(socialNames, " / "))
	}
	lines := []string{
		"🧭 <b>S5 链上发现</b>",
		fmt.Sprintf("🧨 <b>%s · %s</b>", html.EscapeString(signal.Symbol), html.EscapeString(signalLabel)),
		"⏰ " + formatBJTime(signal.CreatedAt),
		"",
	}
	lines = appendEscapedLines(lines, metricLines...)
	if contractLine := formatContractLine(signal, token); contractLine != "" {
		lines = append(lines, contractLine)
	}
	lines = append(lines,
		"",
		"📝 <i>"+html.EscapeString(summary)+"</i>",
		"🔎 "+html.EscapeString(signal.Reason),
		"🏷 S5 链上发现 | "+html.EscapeString(tagText(tags, "#s5")),
	)
	return strings.Join(lines, "\n")
}

func formatS2SignalMessage(signal *model.SignalEvent, token *model.TokenProfile) string {
	raw := parseRaw(signal.RawJSON)
	if direction := rawString(raw["position_direction"]); direction != "" {
		raw["direction_line"] = strings.TrimSpace(rawString(raw["funding_flip_label"]) + " / " + direction)
	}
	tags := parseStringList(signal.TagsJSON)
	signalLabel := labelOr(s2TypeLabels, signal.SignalType, signal.SignalType)
	summary := labelOr(s2TypeSummaries, signal.SignalType, signal.Reason)
	metricLines := []string{
		fmt.Sprintf("📉 费率: %s → %s", fmtSigned(raw["previous_funding_pct"], 3, "%"), fmtSigned(raw["current_funding_pct"], 3, "%")),
		"🧭 方向: " + rawStringDefault(raw["direction_line"], "-"),
		fmt.Sprintf("📊 OI: %s   现货: %s", fmtSigned(raw["oi_change_pct"], 1, "%"), boolCN(raw["has_spot"])),
		"🌐 成交额: $" + formatCompactNumber(raw["volume_usd"]),
	}
	if segments := rawFloatSlice(raw["oi_segments"]); len(segments) > 0 {
		parts := make([]string, 0, len(segments))
		for _, value := range segments {
			parts = append(parts, fmt.Sprintf("%.1fM", value/1_000_000))
		}
		metricLines = append(metricLines, "📦 OI轨迹: "+strings.Join(parts, " > "))
	}
	lines := []string{
		"🧮 <b>S2 费率翻转</b>",
		fmt.Sprintf("⚡ <b>%s · %s</b>", html.EscapeString(signal.Symbol), html.EscapeString(signalLabel)),
		"⏰ " + formatBJTime(signal.CreatedAt),
		"",
	}
	lines = appendEscapedLines(lines, metricLines...)
	if contractLine := formatContractLine(signal, token); contractLine != "" {
		lines = append(lines, contractLine)
	}
	lines = append(lines,
		"",
		"📝 <i>"+html.EscapeString(summary)+"</i>",
		"🔎 "+html.EscapeString(signal.Reason),
		"🏷 S2 费率翻转 | "+html.EscapeString(tagText(tags, "#s2")),
	)
	return strings.Join(lines, "\n")
}

func formatS1SignalMessage(signal *model.SignalEvent, token *model.TokenProfile) string {
	raw := parseRaw(signal.RawJSON)
	if signal.SignalType != "alpha_discovery" {
		lines := []string{
			"📪 <b>S1 Binance Alpha</b>",
			fmt.Sprintf("<b>%s · %s</b>", html.EscapeString(signal.Symbol), html.EscapeString(labelOr(s1TypeLabels, signal.SignalType, signal.SignalType))),
			"Time: " + formatBJTime(signal.CreatedAt),
			"",
			"Stage: " + html.EscapeString(rawStringDefault(raw["push_type"], signal.SignalType)),
			"Launch: " + html.EscapeString(rawStringDefault(raw["launch_time"], "-")),
			fmt.Sprintf("Price: %s   MC: $%s   FDV: $%s", formatPrice(raw["price"]), formatCompactNumber(raw["mcap"]), formatCompactNumber(raw["fdv"])),
		}
		if raw["change_pct"] != nil {
			lines = append(lines, "Change: "+html.EscapeString(fmtSigned(raw["change_pct"], 1, "%")))
		}
		if contractLine := formatContractLine(signal, token); contractLine != "" {
			lines = append(lines, contractLine)
		}
		lines = append(lines, "", html.EscapeString(signal.Reason))
		return strings.Join(lines, "\n")
	}
	tags := parseStringList(signal.TagsJSON)
	signalLabel := labelOr(s1TypeLabels, signal.SignalType, signal.SignalType)
	summary := labelOr(s1TypeSummaries, signal.SignalType, signal.Reason)
	kind := labelOr(s1KindLabels, rawStringDefault(raw["announcement_kind"], "other"), rawStringDefault(raw["announcement_kind"], "公告信号"))
	metricLines := []string{
		"📌 类型: " + kind,
		"📅 发现日期: " + rawStringDefault(raw["launch_date"], "-"),
		fmt.Sprintf("💰 价格: %s   市值: $%s", formatPrice(raw["price"]), formatCompactNumber(raw["mcap"])),
		"🏦 FDV: $" + formatCompactNumber(raw["fdv"]),
	}
	if vcs := rawStringSlice(raw["vcs"]); len(vcs) > 0 {
		if len(vcs) > 3 {
			vcs = vcs[:3]
		}
		metricLines = append(metricLines, "🏛 机构: "+strings.Join(vcs, " / "))
	}
	lines := []string{
		"📰 <b>S1 币安公告</b>",
		fmt.Sprintf("📣 <b>%s · %s</b>", html.EscapeString(signal.Symbol), html.EscapeString(signalLabel)),
		"⏰ " + formatBJTime(signal.CreatedAt),
		"",
	}
	lines = appendEscapedLines(lines, metricLines...)
	if contractLine := formatContractLine(signal, token); contractLine != "" {
		lines = append(lines, contractLine)
	}
	lines = append(lines,
		"",
		"📝 <i>"+html.EscapeString(summary)+"</i>",
		"🔎 "+html.EscapeString(signal.Reason),
		"🏷 S1 币安公告 | "+html.EscapeString(tagText(tags, "#s1")),
	)
	return strings.Join(lines, "\n")
}

// tokenKey 生成推送策略内部使用的 token 归一化键。
func tokenKey(chain string, address string) string {
	return strings.ToLower(chain) + "|" + scanners.NormalizeAddress(address)
}

// parseRaw 将 signals.raw_json 转为 map，解析失败时返回空 map。
func parseRaw(rawJSON string) map[string]any {
	var raw map[string]any
	if json.Unmarshal([]byte(rawJSON), &raw) != nil {
		return map[string]any{}
	}
	return raw
}

func parseStringList(jsonText string) []string {
	var values []string
	if json.Unmarshal([]byte(jsonText), &values) == nil {
		return values
	}
	var anys []any
	if json.Unmarshal([]byte(jsonText), &anys) != nil {
		return nil
	}
	result := make([]string, 0, len(anys))
	for _, value := range anys {
		if text := strings.TrimSpace(fmt.Sprint(value)); text != "" {
			result = append(result, text)
		}
	}
	return result
}

func formatContractLine(signal *model.SignalEvent, token *model.TokenProfile) string {
	address := extractCopyableAddress(signal, token)
	if address == "" {
		return ""
	}
	return "\n📄 合约：<code>" + html.EscapeString(address) + "</code>"
}

func copyItemsForSignal(signal *model.SignalEvent, token *model.TokenProfile) []radartelegram.CopyItem {
	address := extractCopyableAddress(signal, token)
	if address == "" {
		return nil
	}
	return []radartelegram.CopyItem{{Label: "复制合约", Text: address}}
}

func copyItemsForSignals(signals []*model.SignalEvent) []radartelegram.CopyItem {
	items := []radartelegram.CopyItem{}
	seen := map[string]bool{}
	for _, signal := range signals {
		address := extractCopyableAddress(signal, nil)
		if address == "" || seen[address] {
			continue
		}
		seen[address] = true
		label := "复制合约"
		if strings.TrimSpace(signal.Symbol) != "" {
			label = "复制 " + signal.Symbol
		}
		if len([]rune(label)) > 20 {
			label = string([]rune(label)[:20])
		}
		items = append(items, radartelegram.CopyItem{Label: label, Text: address})
		if len(items) >= 6 {
			break
		}
	}
	return items
}

func extractCopyableAddress(signal *model.SignalEvent, token *model.TokenProfile) string {
	candidates := []string{}
	if token != nil {
		candidates = append(candidates, strings.TrimSpace(token.Address))
	}
	if signal != nil {
		candidates = append(candidates, strings.TrimSpace(signal.Address))
	}
	for _, candidate := range candidates {
		if evmAddressRE.MatchString(candidate) || base58AddressRE.MatchString(candidate) {
			return candidate
		}
	}
	return ""
}

func appendEscapedLines(lines []string, values ...string) []string {
	for _, value := range values {
		lines = append(lines, html.EscapeString(value))
	}
	return lines
}

func tagText(tags []string, fallback string) string {
	if len(tags) == 0 {
		return fallback
	}
	limit := len(tags)
	if limit > 6 {
		limit = 6
	}
	parts := make([]string, 0, limit)
	for _, tag := range tags[:limit] {
		parts = append(parts, "#"+tag)
	}
	return strings.Join(parts, " ")
}

func resonanceReasonText(reason string, sources []string) string {
	reason = strings.TrimSpace(reason)
	if strings.HasPrefix(reason, "Cross-source resonance:") {
		sourceText := strings.TrimSpace(strings.TrimPrefix(reason, "Cross-source resonance:"))
		if sourceText == "" && len(sources) > 0 {
			sourceText = strings.Join(sources, ", ")
		}
		return "跨源共振: " + sourceText
	}
	if reason != "" {
		return reason
	}
	if len(sources) > 0 {
		return "跨源共振: " + strings.Join(sources, ", ")
	}
	return "跨源共振"
}

func formatBJTime(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		parsed, err = time.Parse(time.RFC3339, value)
	}
	if err != nil {
		return "-"
	}
	return parsed.In(bjLocation).Format("01-02 15:04")
}

func fmtSigned(value any, decimals int, suffix string) string {
	number, ok := rawFloatOK(value)
	if !ok {
		return "-"
	}
	return fmt.Sprintf("%+.*f%s", decimals, number, suffix)
}

func formatPrice(value any) string {
	number, ok := rawFloatOK(value)
	if !ok {
		return "-"
	}
	switch {
	case number >= 100:
		return fmt.Sprintf("%.2f", number)
	case number >= 1:
		return trimFloat(fmt.Sprintf("%.4f", number))
	case number >= 0.01:
		return trimFloat(fmt.Sprintf("%.5f", number))
	default:
		return trimFloat(fmt.Sprintf("%.8f", number))
	}
}

func formatCompactNumber(value any) string {
	number, ok := rawFloatOK(value)
	if !ok {
		return "-"
	}
	abs := number
	if abs < 0 {
		abs = -abs
	}
	switch {
	case abs >= 1_000_000_000:
		return fmt.Sprintf("%.2fB", number/1_000_000_000)
	case abs >= 1_000_000:
		return fmt.Sprintf("%.2fM", number/1_000_000)
	case abs >= 1_000:
		return fmt.Sprintf("%.2fK", number/1_000)
	default:
		return fmt.Sprintf("%.2f", number)
	}
}

func formatFixed(value any, decimals int) string {
	number, ok := rawFloatOK(value)
	if !ok {
		return "-"
	}
	return fmt.Sprintf("%.*f", decimals, number)
}

func formatFloatComma(value float64, decimals int) string {
	formatted := fmt.Sprintf("%.*f", decimals, value)
	parts := strings.SplitN(formatted, ".", 2)
	whole := parts[0]
	sign := ""
	if strings.HasPrefix(whole, "-") {
		sign = "-"
		whole = strings.TrimPrefix(whole, "-")
	}
	var groups []string
	for len(whole) > 3 {
		groups = append([]string{whole[len(whole)-3:]}, groups...)
		whole = whole[:len(whole)-3]
	}
	groups = append([]string{whole}, groups...)
	result := sign + strings.Join(groups, ",")
	if len(parts) == 2 {
		result += "." + parts[1]
	}
	return result
}

func trimFloat(value string) string {
	value = strings.TrimRight(value, "0")
	return strings.TrimRight(value, ".")
}

func boolCN(value any) string {
	if rawBool(value) {
		return "有"
	}
	return "无"
}

func labelOr(labels map[string]string, key string, fallback string) string {
	if value, ok := labels[key]; ok {
		return value
	}
	return fallback
}

func mapLabel(labels map[string]string, key string, fallback string) string {
	if value, ok := labels[key]; ok {
		return value
	}
	return fallback
}

func rawMap(value any) map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		return typed
	default:
		return map[string]any{}
	}
}

func firstPresent(values map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := values[key]; ok {
			return value
		}
	}
	return nil
}

func rawString(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func rawStringDefault(value any, fallback string) string {
	if text := rawString(value); text != "" {
		return text
	}
	return fallback
}

func rawBool(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return typed == "1" || strings.EqualFold(typed, "true") || strings.EqualFold(typed, "yes") || strings.EqualFold(typed, "on")
	case float64:
		return typed != 0
	case int:
		return typed != 0
	default:
		return false
	}
}

func rawFloatOK(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	case string:
		parsed, err := strconvParseFloat(typed)
		return parsed, err == nil && strings.TrimSpace(typed) != ""
	case nil:
		return 0, false
	default:
		return 0, false
	}
}

func rawFloatSlice(value any) []float64 {
	values, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make([]float64, 0, len(values))
	for _, item := range values {
		if number, ok := rawFloatOK(item); ok {
			result = append(result, number)
		}
	}
	return result
}

func rawStringSlice(value any) []string {
	values, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(values))
	for _, item := range values {
		if text := rawString(item); text != "" {
			result = append(result, text)
		}
	}
	return result
}

// rawFloat 从 raw_json 中的动态类型值读取 float64。
func rawFloat(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case int:
		return float64(typed)
	case string:
		value, _ := strconvParseFloat(typed)
		return value
	case nil:
		return 0
	default:
		return 0
	}
}

// effectiveString 优先读取 settings 表覆盖值，缺失时读取同名环境变量。
func effectiveString(overrides map[string]any, key string) string {
	if value, ok := overrides[key]; ok {
		return strings.TrimSpace(fmt.Sprint(value))
	}
	return strings.TrimSpace(os.Getenv(strings.ToUpper(key)))
}

// effectiveBool 优先读取 settings 表覆盖值，缺失时读取同名环境变量。
func effectiveBool(overrides map[string]any, key string) bool {
	if value, ok := overrides[key]; ok {
		switch typed := value.(type) {
		case bool:
			return typed
		case string:
			return typed == "1" || strings.EqualFold(typed, "true") || strings.EqualFold(typed, "yes") || strings.EqualFold(typed, "on")
		}
	}
	return envBool(strings.ToUpper(key), false)
}

// settingInt 优先读取 settings 表整数配置，缺失时读取环境变量和 fallback。
func settingInt(db *gorm.DB, key string, envKey string, fallback int) int {
	var row model.AppSetting
	if db != nil && db.Where("key = ?", key).First(&row).Error == nil {
		var value int
		if json.Unmarshal([]byte(row.ValueJSON), &value) == nil {
			return value
		}
		var valueFloat float64
		if json.Unmarshal([]byte(row.ValueJSON), &valueFloat) == nil {
			return int(valueFloat)
		}
	}
	return envInt(envKey, fallback)
}

func settingString(db *gorm.DB, key string, envKey string, fallback string) string {
	var row model.AppSetting
	if db != nil && db.Where("key = ?", key).First(&row).Error == nil {
		var value string
		if json.Unmarshal([]byte(row.ValueJSON), &value) == nil && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	if raw := strings.TrimSpace(os.Getenv(envKey)); raw != "" {
		return raw
	}
	return fallback
}

// round2 将浮点数四舍五入到两位小数。
func round2(value float64) float64 {
	return float64(int(value*100+0.5)) / 100
}

// strconvParseFloat 用 fmt.Sscanf 兼容简单数字字符串解析。
func strconvParseFloat(value string) (float64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	var parsed float64
	_, err := fmt.Sscanf(value, "%f", &parsed)
	return parsed, err
}
