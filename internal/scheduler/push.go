package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"go-radar/internal/model"
	"go-radar/internal/scanners"
	radartelegram "go-radar/internal/telegram"

	"gorm.io/gorm"
)

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
			Reason:     "Cross-source resonance: " + strings.Join(sources, ", "),
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
			limit := immediateQuotaLimit(decision.quotaKey)
			if limit > 0 && quotaCounts[decision.quotaKey] >= limit {
				continue
			}
		}
		blocking, err := s.findBlockingRecentPush(signal, decision, isWatchlisted, recentPushes)
		if err != nil {
			return pushedIDs, err
		}
		if blocking != nil {
			log.Printf("suppressed repeat push for %s on %s after %s/%s", signal.Symbol, signal.Source, blocking.Source, blocking.SignalType)
			continue
		}
		if err := notifier.SendText(ctx, s.formatSignalMessage(signal)); err != nil {
			return pushedIDs, err
		}
		if decision.quotaKey != "" {
			quotaCounts[decision.quotaKey]++
		}
		pushedIDs = append(pushedIDs, signal.ID)
		recentPushes[tokenKey(signal.Chain, signal.Address)] = signal
	}

	if scannerName == "s3" {
		digestIDs, err := s.maybeSendS3Digest(ctx, notifier, pushedIDs, recentPushes)
		if err != nil {
			return pushedIDs, err
		}
		pushedIDs = append(pushedIDs, digestIDs...)
	}
	return pushedIDs, nil
}

// markSignalsPushed 将已发送成功的信号标记 pushed_at。
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
	if err := notifier.SendText(ctx, formatS3Digest(digestSignals)); err != nil {
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
	if signal.Source == "system" && signal.SignalType == "resonance" {
		return pushDecision{channel: "immediate", cooldownExempt: true}
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
func immediateQuotaLimit(key string) int {
	if key == "s5_momentum_medium" {
		return 3
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
func (s *Scheduler) formatSignalMessage(signal *model.SignalEvent) string {
	source := html.EscapeString(strings.ToUpper(signal.Source))
	symbol := html.EscapeString(strings.ToUpper(signal.Symbol))
	message := fmt.Sprintf("<b>%s · %s</b>\n%s · %s · score %.1f\n%s", source, symbol, html.EscapeString(signal.SignalType), html.EscapeString(signal.Priority), signal.Score, html.EscapeString(signal.Reason))
	if tokenURL := explorerURL(signal.Chain, signal.Address); tokenURL != "" {
		message += "\n" + html.EscapeString(tokenURL)
	}
	return message
}

// formatS3Digest 生成 S3 摘要推送的 HTML 文本。
func formatS3Digest(signals []*model.SignalEvent) string {
	lines := []string{"<b>S3 Heat Digest</b>"}
	for _, signal := range signals {
		lines = append(lines, fmt.Sprintf("%s · %s · %.1f · %s", html.EscapeString(strings.ToUpper(signal.Symbol)), html.EscapeString(signal.Priority), signal.Score, html.EscapeString(signal.Reason)))
	}
	return strings.Join(lines, "\n")
}

// explorerURL 根据链和地址生成区块浏览器 token 链接。
func explorerURL(chain string, address string) string {
	switch strings.ToLower(chain) {
	case "eth", "ethereum":
		return "https://etherscan.io/token/" + address
	case "bsc":
		return "https://bscscan.com/token/" + address
	case "base":
		return "https://basescan.org/token/" + address
	default:
		return ""
	}
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
