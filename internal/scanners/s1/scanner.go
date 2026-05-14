package s1

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"go-radar/internal/scanners"

	"gorm.io/gorm"
)

const binanceAnnouncementAPI = "https://www.binance.com/bapi/composite/v1/public/cms/article/list/query"

// Scanner 实现 S1 币安 Alpha 公告生命周期扫描器。
//
// 业务目标：复刻 Python 版 announcement_listener、aggregation_worker、
// post_launch_monitor 三段核心流程，同时保留 Go 版统一入库和推送策略。
type Scanner struct {
	db     *gorm.DB     // db 用于保存扫描状态、快照和信号。
	client *http.Client // client 是带代理和超时配置的 HTTP 客户端。
}

// article 是 Binance 公告接口返回的单篇公告。
//
// 业务上它代表一个“可能触发 S1 信号的原始新闻源”，后续会从标题中提取 symbol、
// 公告类型和上线日期。
type article struct {
	Code        string `json:"code"`        // Code 是 Binance 文章唯一编号。
	Title       string `json:"title"`       // Title 是规则过滤、symbol 提取和叙事抽取的主要输入。
	ReleaseDate int64  `json:"releaseDate"` // ReleaseDate 是公告发布时间毫秒时间戳。
	CatalogID   int    // CatalogID 标记公告来自哪个 Binance 栏目。
}

// coinGeckoData 是 S1 对公告代币做外部补全后的市场画像。
//
// 业务上它用于判断项目分层、叙事主题、机构标签和上线后跟踪指标；
// CoinGecko 查不到时，S1 仍可用公告本身生成较弱信号。
type coinGeckoData struct {
	Found             bool     // Found 表示是否在 CoinGecko 找到匹配币种。
	Price             float64  // Price 是当前美元价格。
	FDV               float64  // FDV 是完全稀释估值。
	MCap              float64  // MCap 是流通市值。
	TotalSupply       float64  // TotalSupply 是总供应量。
	CirculatingSupply float64  // CirculatingSupply 是流通供应量。
	Chain             string   // Chain 是 CoinGecko platform 名称。
	Contract          string   // Contract 是 CoinGecko 给出的合约地址。
	Categories        []string // Categories 是 CoinGecko 分类，用于叙事和 VC 识别。
	Description       string   // Description 是 CoinGecko 项目简介。
}

// alphaProject 是 S1 在扫描器状态里持久化的项目生命周期对象。
//
// 它对应 Python 版 projects 表和 pushes 表的合并形态：项目资料、评级结果、
// 上线时间、已推送阶段都放在这里，避免每轮重复推送。
type alphaProject struct {
	ID                string            `json:"id"`
	Symbol            string            `json:"symbol"`
	Name              string            `json:"name"`
	LaunchTime        string            `json:"launch_time"`
	Source            string            `json:"source"`
	RawText           string            `json:"raw_text"`
	Tier              string            `json:"tier"`
	TierReason        string            `json:"tier_reason"`
	Narrative         string            `json:"narrative"`
	NarrativeDesc     string            `json:"narrative_desc"`
	VCs               []string          `json:"vcs"`
	IsDarling         bool              `json:"is_darling"`
	OpenPrice         float64           `json:"open_price"`
	TotalSupply       float64           `json:"total_supply"`
	CirculatingSupply float64           `json:"circulating_supply"`
	FDV               float64           `json:"fdv"`
	CirculatingMCap   float64           `json:"circulating_mcap"`
	Chain             string            `json:"chain"`
	Contract          string            `json:"contract"`
	Excluded          bool              `json:"excluded"`
	ExcludeReason     string            `json:"exclude_reason"`
	DiscoveredAt      string            `json:"discovered_at"`
	UpdatedAt         string            `json:"updated_at"`
	Pushes            map[string]string `json:"pushes"`
}

// narrativeExtract 是 LLM 或规则降级后得到的项目研究结论。
type narrativeExtract struct {
	Narrative     string   `json:"narrative"`
	NarrativeDesc string   `json:"narrative_desc"`
	VCs           []string `json:"vcs"`
	IsDarling     bool     `json:"is_darling"`
	ExcludeReason string   `json:"exclude_reason"`
}

// NewScanner 创建 S1 扫描器实例。
func NewScanner(db *gorm.DB) *Scanner {
	return &Scanner{db: db, client: scanners.NewHTTPClient()}
}

// Scan 执行一次 S1 扫描。
//
// Go 版调度器是按 tick 调用单个 Scan，因此这里把 Python 版三个协程的关键动作折叠到一次扫描中：
// 1. 拉公告并发现新项目；
// 2. 对 PENDING 项目做 CoinGecko + LLM/规则聚合、评级和 discovery 推送；
// 3. 对活跃项目做 T-3h、T-30m、上线瞬间、上线后 30min*4 和异常监控。
func (s *Scanner) Scan(ctx context.Context) (scanners.Result, error) {
	result := scanners.Result{ScannerName: "s1", Metadata: map[string]any{}}
	projects, err := s.loadProjects()
	if err != nil {
		return result, err
	}

	announcements, err := s.fetchAnnouncements(ctx, 20)
	if err != nil {
		return result, err
	}
	newCandidates := s.discoverProjects(announcements, projects)

	aggregated := 0
	for id, project := range projects {
		if project.Excluded || project.Tier != "PENDING" {
			continue
		}
		signals, snapshots, warnings := s.aggregateProject(ctx, &project)
		result.Signals = append(result.Signals, signals...)
		result.Snapshots = append(result.Snapshots, snapshots...)
		result.Warnings = append(result.Warnings, warnings...)
		projects[id] = project
		aggregated++
	}

	monitorSignals, monitorSnapshots, warnings := s.monitorProjects(ctx, projects)
	result.Signals = append(result.Signals, monitorSignals...)
	result.Snapshots = append(result.Snapshots, monitorSnapshots...)
	result.Warnings = append(result.Warnings, warnings...)

	if err := s.saveProjects(projects); err != nil {
		return result, err
	}
	result.Metadata = map[string]any{
		"announcement_count": len(announcements),
		"new_candidates":     newCandidates,
		"aggregated":         aggregated,
		"active_projects":    countActiveProjects(projects),
		"warnings":           result.Warnings,
	}
	return result, nil
}

func (s *Scanner) discoverProjects(announcements []article, projects map[string]alphaProject) int {
	newCandidates := 0
	for _, item := range announcements {
		if item.Title == "" || !IsTrigger(item.Title) {
			continue
		}
		symbol := ExtractSymbol(item.Title)
		if symbol == "" {
			continue
		}
		launchTime, launchDate := releaseTimes(item.ReleaseDate)
		pid := ProjectID(symbol, launchDate)
		if _, ok := projects[pid]; ok {
			continue
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		projects[pid] = alphaProject{
			ID:           pid,
			Symbol:       strings.ToUpper(symbol),
			Name:         ExtractName(item.Title),
			LaunchTime:   launchTime,
			Source:       "binance_announcement",
			RawText:      item.Title,
			Tier:         "PENDING",
			VCs:          []string{},
			DiscoveredAt: now,
			UpdatedAt:    now,
			Pushes:       map[string]string{},
		}
		newCandidates++
	}
	return newCandidates
}

func (s *Scanner) aggregateProject(ctx context.Context, project *alphaProject) ([]scanners.SignalPayload, []scanners.SnapshotPayload, []string) {
	signals := []scanners.SignalPayload{}
	snapshots := []scanners.SnapshotPayload{}
	warnings := []string{}

	cg, cgWarnings := s.fetchCoinGecko(ctx, project.Symbol)
	warnings = append(warnings, cgWarnings...)
	extract, llmWarning := s.extractNarrative(ctx, project.RawText, project.Symbol, project.Name, cg)
	if llmWarning != "" {
		warnings = append(warnings, llmWarning)
	}
	if extract.ExcludeReason == "already_tge" || extract.ExcludeReason == "meme_only" {
		project.Excluded = true
		project.ExcludeReason = extract.ExcludeReason
		project.Tier = "EXCLUDED"
		project.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		return signals, snapshots, warnings
	}

	tier, tierReason := RateProject(cg.MCap, cg.FDV, extract.VCs, extract.Narrative, extract.IsDarling)
	project.Tier = tier
	project.TierReason = tierReason
	project.Narrative = extract.Narrative
	project.NarrativeDesc = extract.NarrativeDesc
	project.VCs = extract.VCs
	project.IsDarling = extract.IsDarling
	project.OpenPrice = cg.Price
	project.TotalSupply = cg.TotalSupply
	project.CirculatingSupply = cg.CirculatingSupply
	project.FDV = cg.FDV
	project.CirculatingMCap = cg.MCap
	project.Chain = MapChain(cg.Chain)
	project.Contract = strings.ToLower(cg.Contract)
	project.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if project.Pushes == nil {
		project.Pushes = map[string]string{}
	}

	raw := rawForProject(project, "discovery", cg.Price, cg.MCap, cg.FDV, 0)
	address := projectAddress(*project)
	chain := firstNonEmpty(project.Chain, "binance_alpha")
	price := cg.Price
	mcap := cg.MCap
	snapshots = append(snapshots, scanners.SnapshotPayload{
		Source:  "s1",
		Chain:   chain,
		Address: address,
		Symbol:  project.Symbol,
		Name:    firstNonEmpty(project.Name, project.Symbol),
		Price:   optionalFloat(price),
		MC:      optionalFloat(mcap),
		Raw:     raw,
	})
	if _, pushed := project.Pushes["discovery"]; pushed {
		return signals, snapshots, warnings
	}
	project.Pushes["discovery"] = time.Now().UTC().Format(time.RFC3339Nano)
	signals = append(signals, scanners.SignalPayload{
		Source:     "s1",
		Chain:      chain,
		Address:    address,
		Symbol:     project.Symbol,
		Name:       firstNonEmpty(project.Name, project.Symbol),
		SignalType: "alpha_discovery",
		Priority:   priorityForTier(tier),
		Score:      ScoreTier(tier, cg.FDV, extract.IsDarling),
		Reason:     fmt.Sprintf("%s tier - %s", tier, tierReason),
		Tags:       tagsForProject(*project, DetectAnnouncementKind(project.RawText)),
		Raw:        raw,
		ForcePush:  true,
		Token: &scanners.TokenPayload{
			Chain:          chain,
			Address:        address,
			Symbol:         project.Symbol,
			Name:           firstNonEmpty(project.Name, project.Symbol),
			NarrativeTheme: project.Narrative,
			NarrativeTags:  tagsForProject(*project, DetectAnnouncementKind(project.RawText)),
			Description:    firstNonEmpty(project.NarrativeDesc, cg.Description),
		},
		DedupeKey: "s1|" + project.ID + "|discovery",
	})
	return signals, snapshots, warnings
}

func (s *Scanner) monitorProjects(ctx context.Context, projects map[string]alphaProject) ([]scanners.SignalPayload, []scanners.SnapshotPayload, []string) {
	signals := []scanners.SignalPayload{}
	snapshots := []scanners.SnapshotPayload{}
	warnings := []string{}
	now := time.Now().UTC()

	for id, project := range projects {
		if project.Excluded || project.LaunchTime == "" || project.Tier == "" || project.Tier == "PENDING" || project.Tier == "EXCLUDED" || project.Tier == "ERROR" {
			continue
		}
		launch, err := parseProjectTime(project.LaunchTime)
		if err != nil {
			continue
		}
		if project.Pushes == nil {
			project.Pushes = map[string]string{}
		}
		delta := launch.Sub(now)
		minutesToLaunch := int(delta.Minutes())

		if delta >= 3*time.Hour-5*time.Minute && delta <= 3*time.Hour+5*time.Minute {
			signals = append(signals, project.maybeLifecycleSignal("t_minus_3h", "alpha_countdown", "medium", float64(ScoreTier(project.Tier, project.FDV, project.IsDarling)-8), "T-3h launch reminder", 0, 0, 0, float64(minutesToLaunch))...)
		} else if delta >= 30*time.Minute-150*time.Second && delta <= 30*time.Minute+150*time.Second {
			signals = append(signals, project.maybeLifecycleSignal("t_minus_30m", "alpha_countdown", "high", ScoreTier(project.Tier, project.FDV, project.IsDarling), "T-30m launch reminder", 0, 0, 0, float64(minutesToLaunch))...)
		} else if delta >= -5*time.Minute && delta <= 0 {
			cg, cgWarnings := s.fetchCoinGecko(ctx, project.Symbol)
			warnings = append(warnings, cgWarnings...)
			if cg.Price > 0 {
				signals = append(signals, project.maybeLifecycleSignal("at_launch", "alpha_launch", "high", ScoreTier(project.Tier, cg.FDV, project.IsDarling)+5, "Token is live", cg.Price, cg.MCap, cg.FDV, 0)...)
				snapshots = append(snapshots, projectSnapshot(project, cg, "at_launch", 0))
			}
		} else if delta < 0 && -delta <= 150*time.Minute {
			minutesAfter := int((-delta).Minutes())
			for idx, target := range []int{30, 60, 90, 120} {
				if absInt(minutesAfter-target) > 5 {
					continue
				}
				pushType := fmt.Sprintf("post_30m_%d", idx+1)
				if _, ok := project.Pushes[pushType]; ok {
					continue
				}
				cg, cgWarnings := s.fetchCoinGecko(ctx, project.Symbol)
				warnings = append(warnings, cgWarnings...)
				if cg.Price <= 0 {
					break
				}
				openPrice := project.OpenPrice
				if openPrice <= 0 {
					openPrice = cg.Price
				}
				change := 0.0
				if openPrice > 0 {
					change = (cg.Price - openPrice) / openPrice * 100
				}
				signals = append(signals, project.maybeLifecycleSignal(pushType, "alpha_followup", priorityForFollowup(project.Tier, idx+1), ScoreTier(project.Tier, cg.FDV, project.IsDarling), fmt.Sprintf("+%dmin follow-up %+0.1f%%", target, change), cg.Price, cg.MCap, cg.FDV, change)...)
				snapshots = append(snapshots, projectSnapshot(project, cg, pushType, change))
				if change >= 100 {
					signals = append(signals, project.maybeLifecycleSignal("anomaly_double", "alpha_anomaly", "high", 95, "Market cap doubled after launch", cg.Price, cg.MCap, cg.FDV, change)...)
				} else if change <= -50 {
					signals = append(signals, project.maybeLifecycleSignal("anomaly_halve", "alpha_anomaly", "high", 90, "Market cap halved after launch", cg.Price, cg.MCap, cg.FDV, change)...)
				}
				break
			}
		}
		project.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		projects[id] = project
	}
	return signals, snapshots, warnings
}

func (p *alphaProject) maybeLifecycleSignal(pushType string, signalType string, priority string, score float64, reason string, price float64, mcap float64, fdv float64, change float64) []scanners.SignalPayload {
	if p.Pushes == nil {
		p.Pushes = map[string]string{}
	}
	if _, ok := p.Pushes[pushType]; ok {
		return nil
	}
	p.Pushes[pushType] = time.Now().UTC().Format(time.RFC3339Nano)
	raw := rawForProject(p, pushType, price, mcap, fdv, change)
	return []scanners.SignalPayload{{
		Source:     "s1",
		Chain:      firstNonEmpty(p.Chain, "binance_alpha"),
		Address:    projectAddress(*p),
		Symbol:     p.Symbol,
		Name:       firstNonEmpty(p.Name, p.Symbol),
		SignalType: signalType,
		Priority:   priority,
		Score:      score,
		Reason:     reason,
		Tags:       tagsForProject(*p, pushType),
		Raw:        raw,
		ForcePush:  true,
		Token: &scanners.TokenPayload{
			Chain:          firstNonEmpty(p.Chain, "binance_alpha"),
			Address:        projectAddress(*p),
			Symbol:         p.Symbol,
			Name:           firstNonEmpty(p.Name, p.Symbol),
			NarrativeTheme: p.Narrative,
			NarrativeTags:  tagsForProject(*p, pushType),
			Description:    p.NarrativeDesc,
		},
		DedupeKey: "s1|" + p.ID + "|" + pushType,
	}}
}

func (s *Scanner) loadProjects() (map[string]alphaProject, error) {
	projects := map[string]alphaProject{}
	_, err := scanners.LoadState(s.db, "s1", "projects", &projects)
	if err != nil {
		return map[string]alphaProject{}, err
	}
	if projects == nil {
		projects = map[string]alphaProject{}
	}
	return projects, nil
}

func (s *Scanner) saveProjects(projects map[string]alphaProject) error {
	cutoff := time.Now().UTC().AddDate(0, 0, -14)
	for id, project := range projects {
		parsed, err := parseProjectTime(firstNonEmpty(project.UpdatedAt, project.DiscoveredAt))
		if err == nil && parsed.Before(cutoff) {
			delete(projects, id)
		}
	}
	return scanners.SaveState(s.db, "s1", "projects", projects)
}

func (s *Scanner) fetchAnnouncements(ctx context.Context, limit int) ([]article, error) {
	all := []article{}
	seen := map[string]bool{}
	for _, catalogID := range []int{48, 161, 93} {
		params := url.Values{}
		params.Set("type", "1")
		params.Set("catalogId", fmt.Sprint(catalogID))
		params.Set("pageNo", "1")
		params.Set("pageSize", fmt.Sprint(limit))
		var response announcementResponse
		if err := scanners.GetJSON(ctx, s.client, binanceAnnouncementAPI, params, &response); err != nil {
			return nil, err
		}
		for _, catalog := range response.Data.Catalogs {
			for _, item := range catalog.Articles {
				if item.Code != "" && seen[item.Code] {
					continue
				}
				if item.Code != "" {
					seen[item.Code] = true
				}
				item.CatalogID = catalogID
				all = append(all, item)
			}
		}
	}
	return all, nil
}

func (s *Scanner) fetchCoinGecko(ctx context.Context, symbol string) (coinGeckoData, []string) {
	result := coinGeckoData{}
	params := url.Values{}
	params.Set("query", symbol)
	var search coinGeckoSearchResponse
	if err := scanners.GetJSON(ctx, s.client, "https://api.coingecko.com/api/v3/search", params, &search); err != nil {
		return result, []string{fmt.Sprintf("cg_search_failed:%s:%v", symbol, err)}
	}
	coinID := ""
	for _, coin := range search.Coins {
		if strings.EqualFold(coin.Symbol, symbol) {
			coinID = coin.ID
			break
		}
	}
	if coinID == "" {
		return result, nil
	}
	params = url.Values{}
	params.Set("localization", "false")
	params.Set("tickers", "false")
	params.Set("market_data", "true")
	params.Set("community_data", "false")
	params.Set("developer_data", "false")
	var details coinGeckoDetailsResponse
	if err := scanners.GetJSON(ctx, s.client, "https://api.coingecko.com/api/v3/coins/"+coinID, params, &details); err != nil {
		return result, []string{fmt.Sprintf("cg_details_failed:%s:%v", symbol, err)}
	}
	result.Found = true
	result.Price = details.MarketData.CurrentPrice.USD
	result.FDV = details.MarketData.FullyDilutedValuation.USD
	result.MCap = details.MarketData.MarketCap.USD
	result.TotalSupply = details.MarketData.TotalSupply
	result.CirculatingSupply = details.MarketData.CirculatingSupply
	result.Categories = details.Categories
	result.Description = details.Description.EN
	if len(result.Description) > 500 {
		result.Description = result.Description[:500]
	}
	for chain, address := range details.Platforms {
		if address != "" {
			result.Chain = chain
			result.Contract = address
			break
		}
	}
	return result, nil
}

func (s *Scanner) extractNarrative(ctx context.Context, rawText string, symbol string, name string, cg coinGeckoData) (narrativeExtract, string) {
	narrative, desc, vcs, isDarling := InferNarrative(rawText, cg.Categories, cg.Description)
	fallback := narrativeExtract{
		Narrative:     narrative,
		NarrativeDesc: desc,
		VCs:           vcs,
		IsDarling:     isDarling,
	}
	apiKey := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
	if apiKey == "" {
		return fallback, ""
	}

	payload := map[string]any{
		"model":       firstNonEmpty(os.Getenv("ANTHROPIC_MODEL"), "claude-sonnet-4-20250514"),
		"max_tokens":  800,
		"temperature": 0,
		"system":      "You are a crypto research analyst. Return JSON only.",
		"messages": []map[string]string{{
			"role":    "user",
			"content": llmPrompt(rawText, symbol, name, cg),
		}},
	}
	encoded, _ := json.Marshal(payload)
	baseURL := firstNonEmpty(os.Getenv("ANTHROPIC_BASE_URL"), "https://api.anthropic.com")
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/v1/messages", bytes.NewReader(encoded))
	if err != nil {
		return fallback, fmt.Sprintf("llm_request_failed:%s:%v", symbol, err)
	}
	request.Header.Set("x-api-key", apiKey)
	request.Header.Set("anthropic-version", "2023-06-01")
	request.Header.Set("content-type", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		return fallback, fmt.Sprintf("llm_call_failed:%s:%v", symbol, err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fallback, fmt.Sprintf("llm_call_failed:%s:%s", symbol, response.Status)
	}
	var decoded anthropicResponse
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		return fallback, fmt.Sprintf("llm_decode_failed:%s:%v", symbol, err)
	}
	text := ""
	for _, block := range decoded.Content {
		if block.Type == "text" {
			text = strings.TrimSpace(block.Text)
			break
		}
	}
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	var extracted narrativeExtract
	if err := json.Unmarshal([]byte(strings.TrimSpace(text)), &extracted); err != nil {
		return fallback, fmt.Sprintf("llm_json_failed:%s:%v", symbol, err)
	}
	if extracted.Narrative == "" {
		extracted.Narrative = fallback.Narrative
	}
	if extracted.NarrativeDesc == "" {
		extracted.NarrativeDesc = fallback.NarrativeDesc
	}
	if len(extracted.VCs) == 0 {
		extracted.VCs = fallback.VCs
	}
	extracted.IsDarling = extracted.IsDarling || fallback.IsDarling
	return extracted, ""
}

func llmPrompt(rawText string, symbol string, name string, cg coinGeckoData) string {
	return fmt.Sprintf(`Analyze this Binance Alpha/listing project.
Token: %s
Name: %s
Announcement: %s
CoinGecko categories: %s
Description: %s
Market: FDV=%f MCap=%f Price=%f Chain=%s

Return JSON:
{
  "narrative": "defi_perp|ai_agent|ai_defi|defai|zk_proof|infra|defi|rwa|gamefi|meme|social|stablecoin|unknown",
  "narrative_desc": "one short Chinese description",
  "vcs": ["investor names from categories/announcement"],
  "is_darling": true,
  "exclude_reason": null
}

Use exclude_reason only for "already_tge" or "meme_only".`, symbol, name, rawText, strings.Join(cg.Categories, ", "), cg.Description, cg.FDV, cg.MCap, cg.Price, cg.Chain)
}

func rawForProject(project *alphaProject, pushType string, price float64, mcap float64, fdv float64, change float64) map[string]any {
	if price == 0 {
		price = project.OpenPrice
	}
	if mcap == 0 {
		mcap = project.CirculatingMCap
	}
	if fdv == 0 {
		fdv = project.FDV
	}
	return map[string]any{
		"project_id":          project.ID,
		"title":               project.RawText,
		"fdv":                 fdv,
		"mcap":                mcap,
		"price":               price,
		"change_pct":          change,
		"vcs":                 project.VCs,
		"is_darling":          project.IsDarling,
		"tier":                project.Tier,
		"tier_reason":         project.TierReason,
		"narrative":           project.Narrative,
		"narrative_desc":      project.NarrativeDesc,
		"launch_time":         project.LaunchTime,
		"launch_date":         launchDateFromTime(project.LaunchTime),
		"push_type":           pushType,
		"announcement_kind":   DetectAnnouncementKind(project.RawText),
		"total_supply":        project.TotalSupply,
		"circulating_supply":  project.CirculatingSupply,
		"circulating_mcap":    mcap,
		"initial_float_ratio": floatRatio(project.CirculatingSupply, project.TotalSupply),
	}
}

func projectSnapshot(project alphaProject, cg coinGeckoData, pushType string, change float64) scanners.SnapshotPayload {
	raw := rawForProject(&project, pushType, cg.Price, cg.MCap, cg.FDV, change)
	return scanners.SnapshotPayload{
		Source:  "s1",
		Chain:   firstNonEmpty(project.Chain, "binance_alpha"),
		Address: projectAddress(project),
		Symbol:  project.Symbol,
		Name:    firstNonEmpty(project.Name, project.Symbol),
		Price:   optionalFloat(cg.Price),
		MC:      optionalFloat(cg.MCap),
		Raw:     raw,
	}
}

func tagsForProject(project alphaProject, extra string) []string {
	tags := []string{DetectAnnouncementKind(project.RawText), project.Tier, project.Narrative}
	if extra != "" {
		tags = append(tags, extra)
	}
	for _, vc := range firstN(project.VCs, 3) {
		tags = append(tags, strings.ReplaceAll(strings.ToLower(vc), " ", "_"))
	}
	out := []string{}
	seen := map[string]bool{}
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		out = append(out, tag)
	}
	return out
}

func projectAddress(project alphaProject) string {
	if strings.TrimSpace(project.Contract) != "" {
		return strings.ToLower(project.Contract)
	}
	return strings.ToLower(project.Symbol + "_" + launchDateFromTime(project.LaunchTime))
}

func releaseTimes(releaseDate int64) (string, string) {
	if releaseDate <= 0 {
		now := time.Now().UTC()
		return now.Format(time.RFC3339Nano), now.Format("2006-01-02")
	}
	t := time.UnixMilli(releaseDate).UTC()
	return t.Format(time.RFC3339Nano), t.Format("2006-01-02")
}

func ProjectID(symbol string, date string) string {
	sum := md5.Sum([]byte(strings.ToUpper(symbol) + "_" + date))
	return fmt.Sprintf("%x", sum)[:16]
}

func parseProjectTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("empty time")
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported time %q", value)
}

func launchDateFromTime(value string) string {
	parsed, err := parseProjectTime(value)
	if err != nil {
		return time.Now().UTC().Format("2006-01-02")
	}
	return parsed.Format("2006-01-02")
}

func priorityForTier(tier string) string {
	switch tier {
	case "S", "A":
		return "high"
	case "B":
		return "medium"
	default:
		return "low"
	}
}

func priorityForFollowup(tier string, idx int) string {
	if tier == "S" || tier == "A" || idx == 1 {
		return "high"
	}
	return "medium"
}

func countActiveProjects(projects map[string]alphaProject) int {
	count := 0
	for _, project := range projects {
		if !project.Excluded && project.LaunchTime != "" && project.Tier != "" && project.Tier != "PENDING" && project.Tier != "EXCLUDED" && project.Tier != "ERROR" {
			count++
		}
	}
	return count
}

func floatRatio(num float64, den float64) float64 {
	if den <= 0 {
		return 0
	}
	return num / den * 100
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

type announcementResponse struct {
	Data struct {
		Catalogs []struct {
			Articles []article `json:"articles"`
		} `json:"catalogs"`
	} `json:"data"`
}

type coinGeckoSearchResponse struct {
	Coins []struct {
		ID     string `json:"id"`
		Symbol string `json:"symbol"`
	} `json:"coins"`
}

type coinGeckoDetailsResponse struct {
	Categories  []string `json:"categories"`
	Description struct {
		EN string `json:"en"`
	} `json:"description"`
	Platforms  map[string]string `json:"platforms"`
	MarketData struct {
		CurrentPrice struct {
			USD float64 `json:"usd"`
		} `json:"current_price"`
		FullyDilutedValuation struct {
			USD float64 `json:"usd"`
		} `json:"fully_diluted_valuation"`
		MarketCap struct {
			USD float64 `json:"usd"`
		} `json:"market_cap"`
		TotalSupply       float64 `json:"total_supply"`
		CirculatingSupply float64 `json:"circulating_supply"`
	} `json:"market_data"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

func firstN(values []string, count int) []string {
	if len(values) <= count {
		return values
	}
	return values[:count]
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func optionalFloat(value float64) *float64 {
	if value == 0 {
		return nil
	}
	return &value
}
