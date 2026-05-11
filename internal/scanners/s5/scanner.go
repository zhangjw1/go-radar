package s5

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"go-radar/internal/model"
	"go-radar/internal/scanners"

	"gorm.io/gorm"
)

// Scanner 实现 S5 新币发现与动量扫描器。
//
// 业务目标：从 GMGN 新发/热门榜、Flap launchpad 等来源发现早期 token，
// 结合安全检查、叙事分类和连续市值上涨状态，生成 narrative_tagged、flap_support、
// momentum 等信号。
type Scanner struct {
	db     *gorm.DB     // db 用于读取历史快照和 settings 表中的 S5 阈值。
	client *http.Client // client 是带代理和超时配置的 HTTP 客户端。
}

// tokenData 是 S5 对单个候选新币聚合后的业务对象。
//
// 它不是数据库模型，而是 GMGN/Flap/GoPlus/DexScreener 多个来源合并后的中间态，
// 后续会被转换成 SnapshotPayload、SignalPayload 和 TokenPayload。
type tokenData struct {
	Address       string         // Address 是候选 token 合约地址。
	Chain         string         // Chain 是候选 token 所在链。
	Name          string         // Name 是 GMGN 返回的 token 名称。
	Symbol        string         // Symbol 是 GMGN 返回的 token 符号。
	MC            float64        // MC 是市值或 FDV，用于新币过滤和评分。
	Liq           float64        // Liq 是流动性金额，用于过滤低质量池子。
	Volume        float64        // Volume 是榜单周期内成交额。
	Holders       int64          // Holders 是持有人数量。
	SmartMoney    int64          // SmartMoney 是 GMGN 智能钱相关计数。
	Chg1H         float64        // Chg1H 是 1 小时涨跌幅。
	Chg24H        float64        // Chg24H 是 24 小时涨跌幅。
	AgeH          *float64       // AgeH 是 token 创建至今小时数。
	Price         float64        // Price 是当前价格。
	Buys1H        int64          // Buys1H 是榜单周期内买入次数。
	Sells1H       int64          // Sells1H 是榜单周期内卖出次数。
	Launchpad     string         // Launchpad 标记来源 launchpad，例如 flap。
	Twitter       string         // Twitter 是项目 Twitter 信息。
	Telegram      string         // Telegram 是项目 Telegram 信息。
	Website       string         // Website 是项目官网信息。
	BuyRatio      float64        // BuyRatio 是买卖次数比例，用于 Flap 托底/回暖判断。
	SupportReason string         // SupportReason 是 Flap 支撑信号的可读原因。
	Raw           map[string]any // Raw 保存 GMGN 原始榜单行和补充字段。
}

// rankSpec 描述一次 GMGN 榜单拉取方式。
//
// 业务上 S5 会组合不同链、不同排序维度，尽量覆盖“刚创建”和“正在交易放量”两类新币。
type rankSpec struct {
	chain    string // chain 是 GMGN 链标识。
	interval string // interval 是榜单时间窗口。
	orderBy  string // orderBy 是 GMGN 排序字段。
	limit    int    // limit 是拉取数量上限。
}

// NewScanner 创建 S5 扫描器实例。
func NewScanner(db *gorm.DB) *Scanner {
	return &Scanner{db: db, client: scanners.NewHTTPClient()}
}

// Scan 执行一次 S5 扫描：获取候选新币、做叙事/安全/动量判断，并输出快照和信号。
func (s *Scanner) Scan(ctx context.Context) (scanners.Result, error) {
	result := scanners.Result{ScannerName: "s5", Metadata: map[string]any{}}

	tokens, warnings := s.fetchNewTokens(ctx)
	result.Warnings = append(result.Warnings, warnings...)
	flapTokens, warnings := s.fetchFlapTokens(ctx)
	result.Warnings = append(result.Warnings, warnings...)

	flapByAddress := map[string]tokenData{}
	combined := map[string]tokenData{}
	for _, token := range tokens {
		combined[token.Address] = token
	}
	for _, token := range flapTokens {
		flapByAddress[token.Address] = token
		if _, ok := combined[token.Address]; !ok {
			combined[token.Address] = token
		}
	}

	for _, token := range combined {
		previousRows, err := s.recentMomentumRows(token.Chain, token.Address, settingInt(s.db, "s5_signal_lookback", "S5_SIGNAL_LOOKBACK", 20))
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("recent_snapshots_failed:%s:%v", token.Address, err))
		}
		currentRow := MomentumRow{MC: token.MC, Volume: token.Volume, Price: token.Price, Buys1H: float64(token.Buys1H)}
		momentum := EvaluateMomentum(
			previousRows,
			currentRow,
			settingInt(s.db, "s5_momentum_consecutive_up", "S5_MOMENTUM_CONSECUTIVE_UP", 3),
			settingFloat(s.db, "s5_min_gain_pct", "S5_MIN_GAIN_PCT", 5),
		)

		snapshot := scanners.SnapshotPayload{
			Source:     "s5",
			Chain:      token.Chain,
			Address:    token.Address,
			Symbol:     token.Symbol,
			Name:       token.Name,
			Price:      floatPtr(token.Price),
			MC:         floatPtr(token.MC),
			Liq:        floatPtr(token.Liq),
			Volume:     floatPtr(token.Volume),
			Holders:    intPtr(token.Holders),
			SmartMoney: intPtr(token.SmartMoney),
			Buys1H:     intPtr(token.Buys1H),
			Sells1H:    intPtr(token.Sells1H),
			AgeH:       token.AgeH,
			Raw:        token.Raw,
		}
		if momentum.Reason != "no_state_change" {
			result.Snapshots = append(result.Snapshots, snapshot)
		}

		category, matched := ClassifyNarrative(token.Name, token.Symbol, token.Chain)
		if category == "spam" {
			continue
		}

		stars := 1
		discoveryTags := []string{}
		if category != "check_novelty" {
			discoveryTags = append(discoveryTags, category)
		}
		if category == "musk_trump" || category == "binance_cz" {
			stars = 3
		} else if category == "celebrity_viral" {
			stars = 2
		}
		flapToken, isFlap := flapByAddress[token.Address]
		if isFlap {
			stars = maxInt(stars, 2)
			discoveryTags = append(discoveryTags, "flap")
		}
		discoveryTags = append(discoveryTags, matched...)

		shouldConsiderDiscovery := stars >= 2 || isFlap
		var safety map[string]any
		var desc map[string]string
		ensureEnrichment := func() (map[string]any, map[string]string) {
			if safety == nil {
				safety = s.checkTokenSafety(ctx, token.Chain, token.Address)
			}
			if desc == nil {
				desc = s.fetchTokenDescription(ctx, token.Chain, token.Address)
			}
			return safety, desc
		}

		if shouldConsiderDiscovery {
			safety, desc = ensureEnrichment()
			if isSafe(safety) {
				payload := buildTokenPayload(token, discoveryTags, desc)
				if isFlap {
					score := ScoreDiscoverySignal(stars, token.MC, token.Liq) + minFloat(flapToken.BuyRatio*5, 8)
					result.Signals = append(result.Signals, scanners.SignalPayload{
						Source:     "s5",
						Chain:      token.Chain,
						Address:    token.Address,
						Symbol:     token.Symbol,
						Name:       token.Name,
						SignalType: "flap_support",
						Priority:   StarsToPriority(stars),
						Score:      round2(score),
						Reason:     flapToken.SupportReason,
						Tags:       discoveryTags,
						Raw:        map[string]any{"safety": safety, "buy_ratio": flapToken.BuyRatio},
						Token:      payload,
					})
				} else if stars >= 2 {
					result.Signals = append(result.Signals, scanners.SignalPayload{
						Source:     "s5",
						Chain:      token.Chain,
						Address:    token.Address,
						Symbol:     token.Symbol,
						Name:       token.Name,
						SignalType: "narrative_tagged",
						Priority:   StarsToPriority(stars),
						Score:      ScoreDiscoverySignal(stars, token.MC, token.Liq),
						Reason:     "Narrative tags matched: " + strings.Join(firstStrings(discoveryTags, 3), ", "),
						Tags:       discoveryTags,
						Raw:        map[string]any{"safety": safety},
						Token:      payload,
					})
				}
			}
		}

		if !momentum.Triggered {
			continue
		}
		safety, desc = ensureEnrichment()
		if !isSafe(safety) {
			continue
		}
		momentumTags := append([]string{}, discoveryTags...)
		if momentum.BuysOK {
			momentumTags = append(momentumTags, "buys_up")
		}
		result.Signals = append(result.Signals, scanners.SignalPayload{
			Source:     "s5",
			Chain:      token.Chain,
			Address:    token.Address,
			Symbol:     token.Symbol,
			Name:       token.Name,
			SignalType: "momentum",
			Priority:   StarsToPriority(maxInt(stars, 2)),
			Score:      ScoreMomentumSignal(maxInt(stars, 2), momentum.PctGain, token.SmartMoney),
			Reason:     fmt.Sprintf("Market cap rose for %d consecutive rounds, +%.1f%%.", settingInt(s.db, "s5_momentum_consecutive_up", "S5_MOMENTUM_CONSECUTIVE_UP", 3), momentum.PctGain),
			Tags:       momentumTags,
			Raw:        map[string]any{"momentum": momentum, "safety": safety},
			Token:      buildTokenPayload(token, momentumTags, desc),
		})
	}

	result.Metadata = map[string]any{
		"token_count":    len(tokens),
		"flap_count":     len(flapTokens),
		"combined_count":  len(combined),
		"warnings":        result.Warnings,
		"scanner_backend": "go",
	}
	return result, nil
}

// fetchNewTokens 从 GMGN 多链榜单拉取新币候选，CLI 不可用时退回 HTTP 接口。
func (s *Scanner) fetchNewTokens(ctx context.Context) ([]tokenData, []string) {
	tokens := []tokenData{}
	warnings := []string{}
	seen := map[string]bool{}
	specs := []rankSpec{
		{"eth", "1h", "creation_timestamp", 50},
		{"eth", "1h", "swaps", 50},
		{"bsc", "1h", "creation_timestamp", 50},
		{"bsc", "1h", "swaps", 50},
		{"base", "1h", "creation_timestamp", 50},
		{"base", "1h", "swaps", 50},
	}
	for _, spec := range specs {
		items, err := s.marketTrendingCLI(ctx, spec.chain, spec.interval, spec.orderBy, "desc", spec.limit, nil)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("gmgn_cli_trending_failed:%s:%s:%v", spec.chain, spec.orderBy, err))
			continue
		}
		for _, item := range items {
			token := mapTrendingToken(item, spec.chain, spec.interval)
			if !s.acceptCandidate(token, seen, false) {
				continue
			}
			seen[token.Address] = true
			tokens = append(tokens, token)
		}
	}
	if len(tokens) > 0 {
		return tokens, warnings
	}

	for _, chain := range []string{"eth", "bsc", "base"} {
		urls := []string{
			fmt.Sprintf("https://gmgn.ai/defi/quotation/v1/rank/%s/swaps/1h?orderby=open_timestamp&direction=desc&limit=100", chain),
			fmt.Sprintf("https://gmgn.ai/defi/quotation/v1/rank/%s/swaps/1h?orderby=swaps&direction=desc&limit=50", chain),
		}
		for _, rawURL := range urls {
			items, err := s.gmgnRankHTTP(ctx, rawURL)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("gmgn_new_tokens_failed:%s:%v", chain, err))
				continue
			}
			for _, item := range items {
				token := mapTrendingToken(item, chain, "1h")
				if !s.acceptCandidate(token, seen, false) {
					continue
				}
				seen[token.Address] = true
				tokens = append(tokens, token)
			}
		}
	}
	return tokens, warnings
}

// fetchFlapTokens 拉取 Flap launchpad 候选，并筛出可能出现支撑/回暖的 token。
func (s *Scanner) fetchFlapTokens(ctx context.Context) ([]tokenData, []string) {
	warnings := []string{}
	items, err := s.marketTrendingCLI(ctx, "bsc", "24h", "volume", "desc", 30, []string{"flap"})
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("gmgn_cli_flap_failed:%v", err))
		items, err = s.gmgnRankHTTP(ctx, "https://gmgn.ai/defi/quotation/v1/rank/bsc/swaps/24h?launchpad=flap&orderby=volume&direction=desc&limit=30")
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("gmgn_flap_failed:%v", err))
			return nil, warnings
		}
	}
	candidates := []tokenData{}
	seen := map[string]bool{}
	for _, item := range items {
		token := mapTrendingToken(item, "bsc", "24h")
		token.Launchpad = "flap"
		if !s.acceptCandidate(token, seen, true) {
			continue
		}
		buyRatio := float64(token.Buys1H) / float64(maxInt64(token.Sells1H, 1))
		support, reason := flapSupport(token.Chg1H, token.Chg24H, buyRatio)
		if !support || buyRatio < 1 {
			continue
		}
		token.BuyRatio = buyRatio
		token.SupportReason = reason
		token.Raw["buy_ratio"] = buyRatio
		token.Raw["support_reason"] = reason
		seen[token.Address] = true
		candidates = append(candidates, token)
	}
	return candidates, warnings
}

// acceptCandidate 根据市值、流动性、持有人数量和去重状态过滤低质量候选。
func (s *Scanner) acceptCandidate(token tokenData, seen map[string]bool, requireHolders bool) bool {
	if token.Address == "" || seen[token.Address] {
		return false
	}
	minMC := settingFloat(s.db, "s5_min_mc", "S5_MIN_MC", 1000)
	maxMC := settingFloat(s.db, "s5_max_mc", "S5_MAX_MC", 10_000_000)
	minLiq := settingFloat(s.db, "s5_min_liq", "S5_MIN_LIQ", 500)
	if token.MC < minMC || token.Liq < minLiq || token.MC > maxMC {
		return false
	}
	return !requireHolders || token.Holders >= 5
}

// recentMomentumRows 读取同一 token 的历史 S5 快照，供连续上涨判断使用。
func (s *Scanner) recentMomentumRows(chain string, address string, limit int) ([]MomentumRow, error) {
	var rows []model.TokenSnapshot
	err := s.db.Joins("JOIN tokens ON snapshots.token_id = tokens.id").
		Where("tokens.chain = ? AND tokens.address = ? AND snapshots.source = ?", strings.ToLower(chain), scanners.NormalizeAddress(address), "s5").
		Order("snapshots.created_at desc").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := []MomentumRow{}
	for i := len(rows) - 1; i >= 0; i-- {
		row := rows[i]
		out = append(out, MomentumRow{
			MC:     floatVal(row.MC),
			Volume: floatVal(row.Volume),
			Price:  floatVal(row.Price),
			Buys1H: float64(intVal(row.Buys1H)),
		})
	}
	return out, nil
}

// marketTrendingCLI 调用 gmgn-cli 获取榜单数据，优先复用本项目已有 GMGN 能力。
func (s *Scanner) marketTrendingCLI(ctx context.Context, chain string, interval string, orderBy string, direction string, limit int, platforms []string) ([]map[string]any, error) {
	cliPath := resolveGMGNCLIPath()
	args := []string{"market", "trending", "--chain", chain, "--interval", interval, "--order-by", orderBy, "--direction", direction, "--limit", fmt.Sprintf("%d", limit), "--raw"}
	for _, platform := range platforms {
		args = append(args, "--platform", platform)
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" && strings.HasSuffix(strings.ToLower(cliPath), ".cmd") {
		cmdArgs := append([]string{"/c", cliPath}, args...)
		cmd = exec.CommandContext(ctx, "cmd", cmdArgs...)
	} else {
		cmd = exec.CommandContext(ctx, cliPath, args...)
	}
	cmd.Env = proxyEnv()
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("%v: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, err
	}
	var payload struct {
		Data struct {
			Rank []map[string]any `json:"rank"`
		} `json:"data"`
		Rank []map[string]any `json:"rank"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		return nil, err
	}
	if len(payload.Data.Rank) > 0 {
		return payload.Data.Rank, nil
	}
	return payload.Rank, nil
}

// gmgnRankHTTP 直接访问 GMGN Web 接口，作为 gmgn-cli 不可用时的兜底方案。
func (s *Scanner) gmgnRankHTTP(ctx context.Context, rawURL string) ([]map[string]any, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", os.Getenv("GMGN_USER_AGENT"))
	if request.Header.Get("User-Agent") == "" {
		request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Referer", "https://gmgn.ai/")
	request.Header.Set("Origin", "https://gmgn.ai")
	if cookie := strings.TrimSpace(os.Getenv("GMGN_COOKIE")); cookie != "" {
		request.Header.Set("Cookie", cookie)
	}
	if apiKey := strings.TrimSpace(os.Getenv("GMGN_API_KEY")); apiKey != "" {
		request.Header.Set("X-APIKEY", apiKey)
	}
	response, err := s.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("GET %s returned %s", rawURL, response.Status)
	}
	var payload struct {
		Rank []map[string]any `json:"rank"`
		Data struct {
			Rank []map[string]any `json:"rank"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if len(payload.Rank) > 0 {
		return payload.Rank, nil
	}
	return payload.Data.Rank, nil
}

// checkTokenSafety 使用 GoPlus 检查 honeypot、mintable 和税费等基础安全项。
func (s *Scanner) checkTokenSafety(ctx context.Context, chain string, address string) map[string]any {
	chainID := map[string]string{"ethereum": "1", "eth": "1", "bsc": "56", "base": "8453"}[strings.ToLower(chain)]
	if chainID == "" {
		chainID = "1"
	}
	params := url.Values{"contract_addresses": []string{address}}
	var payload struct {
		Result map[string]map[string]string `json:"result"`
	}
	if err := scanners.GetJSON(ctx, s.client, "https://api.gopluslabs.io/api/v1/token_security/"+chainID, params, &payload); err != nil {
		return map[string]any{"safe": false, "reason": "goplus_unavailable", "error": err.Error()}
	}
	row := payload.Result[strings.ToLower(address)]
	honeypot := row["is_honeypot"] == "1"
	mintable := row["is_mintable"] == "1"
	return map[string]any{
		"safe":     !honeypot && !mintable,
		"honeypot": honeypot,
		"mintable": mintable,
		"sell_tax": scanners.ParseFloat(row["sell_tax"]),
		"buy_tax":  scanners.ParseFloat(row["buy_tax"]),
	}
}

// fetchTokenDescription 从 DexScreener 补充社交链接和官网信息。
func (s *Scanner) fetchTokenDescription(ctx context.Context, chain string, address string) map[string]string {
	var response struct {
		Pairs []struct {
			Info struct {
				Socials []struct {
					Type string `json:"type"`
					URL  string `json:"url"`
				} `json:"socials"`
				Websites []struct {
					Label string `json:"label"`
					URL   string `json:"url"`
				} `json:"websites"`
			} `json:"info"`
		} `json:"pairs"`
	}
	if err := scanners.GetJSON(ctx, s.client, "https://api.dexscreener.com/latest/dex/tokens/"+address, nil, &response); err != nil || len(response.Pairs) == 0 {
		return map[string]string{"description": "", "twitter": "", "telegram": "", "website": ""}
	}
	out := map[string]string{"description": "", "twitter": "", "telegram": "", "website": ""}
	for _, social := range response.Pairs[0].Info.Socials {
		switch strings.ToLower(social.Type) {
		case "twitter":
			out["twitter"] = social.URL
		case "telegram":
			out["telegram"] = social.URL
		}
	}
	for _, website := range response.Pairs[0].Info.Websites {
		if strings.EqualFold(website.Label, "website") {
			out["website"] = website.URL
		}
	}
	return out
}

// mapTrendingToken 将 GMGN 榜单原始行映射为 S5 统一 tokenData。
func mapTrendingToken(item map[string]any, chain string, interval string) tokenData {
	address := strings.ToLower(anyString(firstAny(item, "address", "token_address")))
	creationTimestamp := anyFloat(firstAny(item, "creation_timestamp", "open_timestamp"))
	var ageH *float64
	if creationTimestamp > 0 {
		age := (float64(time.Now().Unix()) - creationTimestamp) / 3600
		ageH = &age
	}
	token := tokenData{
		Address:    address,
		Chain:      chain,
		Name:       fallbackString(anyString(item["name"]), "?"),
		Symbol:     fallbackString(anyString(item["symbol"]), "?"),
		MC:         anyFloat(firstAny(item, "market_cap", "fdv")),
		Liq:        anyFloat(item["liquidity"]),
		Volume:     anyFloat(item["volume"]),
		Holders:    anyInt(item["holder_count"]),
		SmartMoney: anyInt(item["smart_degen_count"]),
		Chg1H:      anyFloat(item["price_change_percent1h"]),
		Chg24H:     anyFloat(item["price_change_percent"]),
		AgeH:       ageH,
		Price:      anyFloat(item["price"]),
		Buys1H:     anyInt(item["buys"]),
		Sells1H:    anyInt(item["sells"]),
		Launchpad:  anyString(firstAny(item, "launchpad_platform", "launchpad")),
		Twitter:    anyString(item["twitter_username"]),
		Telegram:   anyString(item["telegram"]),
		Website:    anyString(item["website"]),
		Raw:        item,
	}
	token.Raw["gmgn_interval"] = interval
	return token
}

// buildTokenPayload 将 S5 中间对象转换为通用 TokenPayload，供存储层更新 tokens 表。
func buildTokenPayload(token tokenData, tags []string, descInfo map[string]string) *scanners.TokenPayload {
	socialLinks := map[string]string{}
	for _, key := range []string{"twitter", "telegram", "website"} {
		if descInfo[key] != "" {
			socialLinks[key] = descInfo[key]
		}
	}
	encoded, _ := json.Marshal(socialLinks)
	return &scanners.TokenPayload{
		Chain:           token.Chain,
		Address:         token.Address,
		Symbol:          token.Symbol,
		Name:            token.Name,
		NarrativeTheme:  NormalizeTheme(token.Name, token.Symbol),
		NarrativeTags:   tags,
		Description:     descInfo["description"],
		SocialLinksJSON: string(encoded),
	}
}

// flapSupport 判断 Flap token 是否出现托底或反弹迹象，并返回可读原因。
func flapSupport(chg1H float64, chg24H float64, buyRatio float64) (bool, string) {
	switch {
	case chg24H < -10 && chg1H > chg24H*0.3:
		return true, fmt.Sprintf("24h drawdown %.0f%% with 1h stabilizing %+0.f%%", chg24H, chg1H)
	case chg24H >= -10 && chg24H <= 30 && chg1H > -5 && buyRatio > 1.1:
		return true, fmt.Sprintf("Bottoming range with buy/sell ratio %.2f", buyRatio)
	case chg24H < -30 && chg1H > 10:
		return true, fmt.Sprintf("Rebound after %.0f%% drawdown, 1h %+0.f%%", chg24H, chg1H)
	default:
		return false, ""
	}
}

// resolveGMGNCLIPath 解析当前系统下可执行的 gmgn-cli 路径。
func resolveGMGNCLIPath() string {
	name := "gmgn-cli"
	if runtime.GOOS == "windows" {
		name = "gmgn-cli.cmd"
	}
	candidates := []string{
		filepath.Join("node_modules", ".bin", name),
		name,
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return name
}

// proxyEnv 为 gmgn-cli 构造代理环境变量，保持和 Go HTTP 客户端一致。
func proxyEnv() []string {
	env := os.Environ()
	proxy := strings.TrimSpace(os.Getenv("SELECTIVE_PROXY_URL"))
	proxyKeys := []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "all_proxy", "GIT_HTTP_PROXY", "GIT_HTTPS_PROXY", "git_http_proxy", "git_https_proxy"}
	if proxy != "" {
		env = upsertEnv(env, "NO_PROXY", "localhost,127.0.0.1,::1")
		env = upsertEnv(env, "no_proxy", "localhost,127.0.0.1,::1")
		for _, key := range proxyKeys {
			env = upsertEnv(env, key, proxy)
		}
		return env
	}
	if scanners.EnvBool("HTTP_TRUST_ENV", false) {
		return env
	}
	for _, key := range proxyKeys {
		env = removeEnv(env, key)
	}
	return env
}

func settingFloat(db *gorm.DB, key string, envKey string, fallback float64) float64 {
	var row model.AppSetting
	if db != nil && db.Where("key = ?", key).First(&row).Error == nil {
		var value float64
		if json.Unmarshal([]byte(row.ValueJSON), &value) == nil {
			return value
		}
	}
	return scanners.EnvFloat(envKey, fallback)
}

func settingInt(db *gorm.DB, key string, envKey string, fallback int) int {
	var row model.AppSetting
	if db != nil && db.Where("key = ?", key).First(&row).Error == nil {
		var value int
		if json.Unmarshal([]byte(row.ValueJSON), &value) == nil {
			return value
		}
	}
	return scanners.EnvInt(envKey, fallback)
}

func isSafe(safety map[string]any) bool {
	value, ok := safety["safe"].(bool)
	return ok && value
}

func floatPtr(value float64) *float64 {
	return &value
}

func intPtr(value int64) *int64 {
	return &value
}

func floatVal(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func intVal(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func anyString(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func anyFloat(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		out, _ := typed.Float64()
		return out
	case string:
		return scanners.ParseFloat(typed)
	default:
		return 0
	}
}

func anyInt(value any) int64 {
	return int64(anyFloat(value))
}

func firstAny(item map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := item[key]; ok && value != nil {
			return value
		}
	}
	return nil
}

func fallbackString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" || value == "<nil>" {
		return fallback
	}
	return value
}

func firstStrings(values []string, limit int) []string {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}

func maxInt64(left int64, right int64) int64 {
	if left > right {
		return left
	}
	return right
}

func minFloat(left float64, right float64) float64 {
	if left < right {
		return left
	}
	return right
}

func round2(value float64) float64 {
	return float64(int(value*100+0.5)) / 100
}

func upsertEnv(env []string, key string, value string) []string {
	prefix := key + "="
	for i, item := range env {
		if strings.HasPrefix(item, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

func removeEnv(env []string, key string) []string {
	prefix := key + "="
	out := env[:0]
	for _, item := range env {
		if !strings.HasPrefix(item, prefix) {
			out = append(out, item)
		}
	}
	return out
}
