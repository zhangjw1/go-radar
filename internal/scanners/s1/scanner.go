package s1

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go-radar/internal/scanners"

	"gorm.io/gorm"
)

const binanceAnnouncementAPI = "https://www.binance.com/bapi/composite/v1/public/cms/article/list/query"

// Scanner 实现 S1 交易所公告扫描器。
//
// 业务目标：从 Binance 新币/上币/Launchpool 等公告里发现早期项目，
// 再用 CoinGecko 补充市值、FDV、分类和合约信息，最后生成 alpha_discovery 信号。
type Scanner struct {
	db     *gorm.DB     // db 用于查询 dedupe_key，避免同一公告重复生成信号。
	client *http.Client // client 是带代理和超时配置的 HTTP 客户端。
}

// article 是 Binance 公告接口返回的单篇公告。
//
// 业务上它代表一个“可能触发 S1 信号的原始新闻源”，后续会从标题中提取 symbol、
// 公告类型和上线日期。
type article struct {
	Code        string `json:"code"`        // Code 是 Binance 文章唯一编号，用于构建公告级去重键。
	Title       string `json:"title"`       // Title 是公告标题，是 S1 规则判断和 symbol 提取的主要输入。
	ReleaseDate int64  `json:"releaseDate"` // ReleaseDate 是公告发布时间毫秒时间戳。
	CatalogID   int    // CatalogID 标记公告来自哪个 Binance 栏目，便于后续排查来源。
}

// coinGeckoData 是 S1 对公告代币做外部补全后的市场画像。
//
// 业务上它用于判断项目分层、叙事主题和信号优先级；CoinGecko 查不到时，
// S1 仍会用公告本身生成弱信号。
type coinGeckoData struct {
	Found       bool     // Found 表示是否在 CoinGecko 找到匹配币种。
	Price       float64  // Price 是当前美元价格。
	FDV         float64  // FDV 是完全稀释估值，用于项目分层。
	MCap        float64  // MCap 是流通市值，用于项目分层。
	Chain       string   // Chain 是 CoinGecko platform 名称，会映射到本系统链标识。
	Contract    string   // Contract 是 CoinGecko 给出的合约地址。
	Categories  []string // Categories 是 CoinGecko 分类，用于叙事识别。
	Description string   // Description 是 CoinGecko 项目简介，用于叙事识别和页面展示。
}

// NewScanner 创建 S1 扫描器实例。
func NewScanner(db *gorm.DB) *Scanner {
	return &Scanner{db: db, client: scanners.NewHTTPClient()}
}

// Scan 执行一次 S1 扫描：拉公告、过滤触发标题、补全市场信息并生成快照/信号。
func (s *Scanner) Scan(ctx context.Context) (scanners.Result, error) {
	result := scanners.Result{ScannerName: "s1", Metadata: map[string]any{}}
	announcements, err := s.fetchAnnouncements(ctx, 20)
	if err != nil {
		return result, err
	}
	newCandidates := 0
	for _, item := range announcements {
		if item.Title == "" || !IsTrigger(item.Title) {
			continue
		}
		symbol := ExtractSymbol(item.Title)
		if symbol == "" {
			continue
		}
		launchDate := time.Now().UTC().Format("2006-01-02")
		if item.ReleaseDate > 0 {
			launchDate = time.UnixMilli(item.ReleaseDate).UTC().Format("2006-01-02")
		}
		dedupeKey := BuildArticleDedupeKey(item.Code, item.Title, symbol, launchDate)
		exists, err := scanners.DedupeExists(s.db, dedupeKey)
		if err != nil {
			return result, err
		}
		if exists {
			continue
		}
		newCandidates++
		name := ExtractName(item.Title)
		cg, warnings := s.fetchCoinGecko(ctx, symbol)
		result.Warnings = append(result.Warnings, warnings...)
		narrative, narrativeDesc, vcs, isDarling := InferNarrative(item.Title, cg.Categories, cg.Description)
		tier, tierReason := RateProject(cg.MCap, cg.FDV, vcs, narrative, isDarling)
		priority := "low"
		if tier == "S" || tier == "A" {
			priority = "high"
		} else if tier == "B" {
			priority = "medium"
		}
		chain := MapChain(cg.Chain)
		address := strings.ToLower(cg.Contract)
		if address == "" {
			address = strings.ToLower(symbol + "_" + launchDate)
		}
		announcementKind := DetectAnnouncementKind(item.Title)
		description := narrativeDesc
		if description == "" && cg.Description != "" {
			description = cg.Description
			if len(description) > 120 {
				description = description[:120]
			}
		}
		tags := append([]string{announcementKind, tier, narrative}, firstN(vcs, 3)...)
		raw := map[string]any{
			"title":             item.Title,
			"fdv":               cg.FDV,
			"mcap":              cg.MCap,
			"price":             cg.Price,
			"vcs":               vcs,
			"is_darling":        isDarling,
			"tier":              tier,
			"tier_reason":       tierReason,
			"launch_date":       launchDate,
			"announcement_kind": announcementKind,
			"article_code":      item.Code,
		}
		price := cg.Price
		mcap := cg.MCap
		result.Snapshots = append(result.Snapshots, scanners.SnapshotPayload{
			Source:  "s1",
			Chain:   chain,
			Address: address,
			Symbol:  symbol,
			Name:    firstNonEmpty(name, symbol),
			Price:   optionalFloat(price),
			MC:      optionalFloat(mcap),
			Raw:     raw,
		})
		result.Signals = append(result.Signals, scanners.SignalPayload{
			Source:     "s1",
			Chain:      chain,
			Address:    address,
			Symbol:     symbol,
			Name:       firstNonEmpty(name, symbol),
			SignalType: "alpha_discovery",
			Priority:   priority,
			Score:      ScoreTier(tier, cg.FDV, isDarling),
			Reason:     fmt.Sprintf("%s tier - %s", tier, tierReason),
			Tags:       tags,
			Raw:        raw,
			Token: &scanners.TokenPayload{
				Chain:          chain,
				Address:        address,
				Symbol:         symbol,
				Name:           firstNonEmpty(name, symbol),
				NarrativeTheme: narrative,
				NarrativeTags:  tags,
				Description:    description,
			},
			DedupeKey: dedupeKey,
		})
	}
	result.Metadata = map[string]any{"announcement_count": len(announcements), "new_candidates": newCandidates, "warnings": result.Warnings}
	return result, nil
}

// fetchAnnouncements 拉取多个 Binance 公告栏目，并按文章 code 去重。
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

// fetchCoinGecko 根据 symbol 查询 CoinGecko，并抽取 S1 需要的市场和叙事字段。
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

// announcementResponse 是 Binance 公告列表接口的最小响应结构。
type announcementResponse struct {
	Data struct { // Data.Catalogs 保存公告栏目和栏目下文章列表。
		Catalogs []struct {
			Articles []article `json:"articles"`
		} `json:"catalogs"`
	} `json:"data"`
}

// coinGeckoSearchResponse 是 CoinGecko 搜索接口的最小响应结构。
type coinGeckoSearchResponse struct {
	Coins []struct { // Coins 是搜索命中的候选币种，S1 会优先匹配 symbol 相等的项。
		ID     string `json:"id"`
		Symbol string `json:"symbol"`
	} `json:"coins"`
}

// coinGeckoDetailsResponse 是 CoinGecko 币种详情接口的最小响应结构。
type coinGeckoDetailsResponse struct {
	Categories  []string `json:"categories"` // Categories 是 CoinGecko 分类，参与叙事识别。
	Description struct { // Description 保存多语言简介，这里只读取英文简介。
		EN string `json:"en"`
	} `json:"description"`
	Platforms  map[string]string `json:"platforms"` // Platforms 是链名到合约地址的映射。
	MarketData struct { // MarketData 保存价格、市值和 FDV。
		CurrentPrice struct {
			USD float64 `json:"usd"`
		} `json:"current_price"`
		FullyDilutedValuation struct {
			USD float64 `json:"usd"`
		} `json:"fully_diluted_valuation"`
		MarketCap struct {
			USD float64 `json:"usd"`
		} `json:"market_cap"`
	} `json:"market_data"`
}

// firstN 返回最多 count 个字符串，用于控制标签数量。
func firstN(values []string, count int) []string {
	if len(values) <= count {
		return values
	}
	return values[:count]
}

// firstNonEmpty 返回第一项非空字符串。
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// optionalFloat 将 0 视作缺失值，避免把未知市场数据写成有效 0。
func optionalFloat(value float64) *float64 {
	if value == 0 {
		return nil
	}
	return &value
}
