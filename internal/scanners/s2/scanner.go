package s2

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"go-radar/internal/scanners"

	"gorm.io/gorm"
)

// Scanner 实现 S2 合约资金费率反转扫描器。
//
// 业务目标：跟踪 Binance USDT 永续合约，当资金费率由正转负且 OI 同步上升时，
// 识别“空头拥挤但资金继续进场”的潜在反身性机会。
type Scanner struct {
	db     *gorm.DB     // db 用于读取上一轮 funding 快照，判断是否发生资金费率翻转。
	client *http.Client // client 是带代理和超时配置的 HTTP 客户端。
}

// NewScanner 创建 S2 扫描器实例。
func NewScanner(db *gorm.DB) *Scanner {
	return &Scanner{
		db:     db,
		client: scanners.NewHTTPClient(),
	}
}

/*
  - 热度做多雷达 v2 — 热度+费率+OI 三维扫描

核心逻辑（拉哪模式）：
1. 热度先行 → CG热搜+放量=资金涌入信号
2. 负费率=空头燃料，庄家拉盘爆空单
3. OI暴涨=大资金建仓=即将拉盘

单策略：发现热度→小仓做多→严格止损→拿住赢家
数据源：币安合约API + CoinGecko Trending（零成本）
*/
func (s *Scanner) Scan(ctx context.Context) (scanners.Result, error) {
	result := scanners.Result{ScannerName: "s2", Metadata: map[string]any{}}

	var exchangeInfo exchangeInfoResponse
	if err := s.getJSON(ctx, "https://fapi.binance.com/fapi/v1/exchangeInfo", nil, &exchangeInfo); err != nil {
		return result, err
	}
	symbols := make([]string, 0, len(exchangeInfo.Symbols))
	for _, item := range exchangeInfo.Symbols {
		if item.ContractType == "PERPETUAL" && item.QuoteAsset == "USDT" && item.Status == "TRADING" {
			symbols = append(symbols, item.Symbol)
		}
	}

	var tickers []ticker24h
	if err := s.getJSON(ctx, "https://fapi.binance.com/fapi/v1/ticker/24hr", nil, &tickers); err != nil {
		return result, err
	}
	tickerMap := make(map[string]ticker24h, len(tickers))
	for _, item := range tickers {
		if strings.HasSuffix(item.Symbol, "USDT") {
			tickerMap[item.Symbol] = item
		}
	}

	var premiumIndex []premiumIndexItem
	if err := s.getJSON(ctx, "https://fapi.binance.com/fapi/v1/premiumIndex", nil, &premiumIndex); err != nil {
		return result, err
	}
	currentFunding := make(map[string]float64, len(premiumIndex))
	for _, item := range premiumIndex {
		if strings.HasSuffix(item.Symbol, "USDT") {
			currentFunding[item.Symbol] = scanners.ParseFloat(item.LastFundingRate)
		}
	}

	marketCaps, warnings := s.fetchMarketCaps(ctx)
	result.Warnings = append(result.Warnings, warnings...)
	spotSymbols, warnings := s.fetchSpotSymbols(ctx)
	result.Warnings = append(result.Warnings, warnings...)

	activeSymbols := symbols
	previousFunding := make(map[string]*float64, len(activeSymbols))
	for _, symbol := range activeSymbols {
		snapshot, err := scanners.RecentFundingSnapshot(s.db, "binance_perp", strings.ToLower(symbol), "s2")
		if err != nil {
			return result, err
		}
		if snapshot != nil && snapshot.FundingPct != nil {
			value := *snapshot.FundingPct / 100
			previousFunding[symbol] = &value
		}
	}

	turnedNegative := make(map[string]bool)
	for _, symbol := range activeSymbols {
		current, ok := currentFunding[symbol]
		if !ok {
			continue
		}
		if IsFundingFlip(previousFunding[symbol], &current) {
			turnedNegative[symbol] = true
		}
	}

	for _, symbol := range activeSymbols {
		ticker := tickerMap[symbol]
		baseSymbol := strings.TrimSuffix(symbol, "USDT")
		volumeUSD := scanners.ParseFloat(ticker.QuoteVolume)
		price := scanners.ParseFloat(ticker.LastPrice)
		priceChange24h := scanners.ParseFloat(ticker.PriceChangePercent)
		currentFRPct := currentFunding[symbol] * 100
		var previousFRPct *float64
		if previousFunding[symbol] != nil {
			value := *previousFunding[symbol] * 100
			previousFRPct = &value
		}
		hasSpot := spotSymbols[baseSymbol]
		squarePosts, squareViews, squareWarning := s.getSquareDiscussion(ctx, baseSymbol)
		if squareWarning != "" {
			result.Warnings = append(result.Warnings, squareWarning)
		}
		var currentOIUSD *float64
		var oiChange *float64
		segments := []float64{}
		oiRising := false

		if turnedNegative[symbol] {
			oiValues, err := s.fetchOIHistory(ctx, symbol)
			if err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("oi_history_failed:%s:%v", symbol, err))
			} else {
				computedSegments, oiChangeValue, rising := ComputeOISegments(oiValues)
				segments = computedSegments
				oiRising = rising
				oiChange = &oiChangeValue
				if len(oiValues) > 0 {
					last := oiValues[len(oiValues)-1]
					currentOIUSD = &last
				}
			}
		}

		marketCap := marketCaps[baseSymbol]
		result.Snapshots = append(result.Snapshots, scanners.SnapshotPayload{
			Source:     "s2",
			Chain:      "binance_perp",
			Address:    strings.ToLower(symbol),
			Symbol:     baseSymbol,
			Name:       baseSymbol,
			Price:      &price,
			MC:         marketCap,
			Volume:     &volumeUSD,
			FundingPct: &currentFRPct,
			OIUSD:      currentOIUSD,
			OID6H:      oiChange,
			Raw: map[string]any{
				"symbol":               symbol,
				"price_change_24h":     priceChange24h,
				"oi_segments":          segments,
				"previous_funding_pct": previousFRPct,
				"current_funding_pct":  currentFRPct,
				"has_spot":             hasSpot,
				"square_posts":         squarePosts,
				"square_views":         squareViews,
			},
		})

		if !turnedNegative[symbol] || oiChange == nil || !oiRising {
			continue
		}
		priority := "medium"
		if *oiChange >= 8 {
			priority = "high"
		}
		tags := []string{"funding_flip", "oi_rising"}
		if hasSpot {
			tags = append(tags, "has_spot")
		}
		raw := map[string]any{
			"symbol":               symbol,
			"price_change_24h":     priceChange24h,
			"oi_segments":          segments,
			"previous_funding_pct": previousFRPct,
			"current_funding_pct":  currentFRPct,
			"oi_change_pct":        *oiChange,
			"volume_usd":           volumeUSD,
			"has_spot":             hasSpot,
			"square_posts":         squarePosts,
			"square_views":         squareViews,
		}
		result.Signals = append(result.Signals, scanners.SignalPayload{
			Source:     "s2",
			Chain:      "binance_perp",
			Address:    strings.ToLower(symbol),
			Symbol:     baseSymbol,
			Name:       baseSymbol,
			SignalType: "funding_flip_oi_rising",
			Priority:   priority,
			Score:      ScoreFundingSignal(currentFRPct, *oiChange, hasSpot, volumeUSD),
			Reason:     fmt.Sprintf("Funding flipped from %s%% to %+0.3f%%, OI %+0.1f%%", formatOptional(previousFRPct), currentFRPct, *oiChange),
			Tags:       tags,
			ForcePush:  true,
			Raw:        raw,
			Token: &scanners.TokenPayload{
				Chain:          "binance_perp",
				Address:        strings.ToLower(symbol),
				Symbol:         baseSymbol,
				Name:           baseSymbol,
				NarrativeTheme: strings.ToLower(baseSymbol),
				NarrativeTags:  tags,
			},
		})
	}

	result.Metadata = map[string]any{
		"active_symbols":        len(activeSymbols),
		"turned_negative_count": len(turnedNegative),
		"warnings":              result.Warnings,
	}
	return result, nil
}

// fetchMarketCaps 从 Binance 营销接口补充币种市值，用于 S2 页面和评分参考。
func (s *Scanner) fetchMarketCaps(ctx context.Context) (map[string]*float64, []string) {
	var response marketCapsResponse
	if err := s.getJSON(ctx, "https://www.binance.com/bapi/composite/v1/public/marketing/symbol/list", nil, &response); err != nil {
		return map[string]*float64{}, []string{fmt.Sprintf("market_caps_failed:%v", err)}
	}
	marketCaps := make(map[string]*float64)
	for _, item := range response.Data {
		if item.Name == "" || item.MarketCap == nil {
			continue
		}
		value := *item.MarketCap
		marketCaps[item.Name] = &value
	}
	return marketCaps, nil
}

// fetchSpotSymbols 读取 Binance 现货列表，用于判断合约标的是否也有现货市场。
func (s *Scanner) fetchSpotSymbols(ctx context.Context) (map[string]bool, []string) {
	var response spotExchangeInfoResponse
	if err := s.getJSON(ctx, "https://api.binance.com/api/v3/exchangeInfo", nil, &response); err != nil {
		return map[string]bool{}, []string{fmt.Sprintf("spot_symbols_failed:%v", err)}
	}
	symbols := make(map[string]bool)
	for _, item := range response.Symbols {
		if item.QuoteAsset == "USDT" && item.Status == "TRADING" {
			symbols[item.BaseAsset] = true
		}
	}
	return symbols, nil
}

// fetchOIHistory 拉取某个合约最近 OI 序列，用于确认资金费率翻转后是否有持仓增长。
func (s *Scanner) fetchOIHistory(ctx context.Context, symbol string) ([]float64, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("period", "1h")
	params.Set("limit", "48")
	var response []oiHistoryItem
	if err := s.getJSON(ctx, "https://fapi.binance.com/futures/data/openInterestHist", params, &response); err != nil {
		return nil, err
	}
	values := make([]float64, 0, len(response))
	for _, item := range response {
		values = append(values, scanners.ParseFloat(item.SumOpenInterestValue))
	}
	return values, nil
}

// getJSON 通过扫描器统一 HTTP 客户端发起 JSON GET 请求。
func (s *Scanner) getJSON(ctx context.Context, rawURL string, params url.Values, target any) error {
	return scanners.GetJSON(ctx, s.client, rawURL, params, target)
}

// exchangeInfoResponse 是 Binance 合约交易规则接口的最小响应结构。
//
// 业务上用于筛出正在交易的 USDT 永续合约。
type exchangeInfoResponse struct {
	Symbols []struct { // Symbols 是合约市场列表。
		Symbol       string `json:"symbol"`
		ContractType string `json:"contractType"`
		QuoteAsset   string `json:"quoteAsset"`
		Status       string `json:"status"`
	} `json:"symbols"`
}

// ticker24h 是 Binance 24 小时行情结构。
//
// 业务上提供价格、涨跌幅和成交额，是 S2 快照与评分输入。
type ticker24h struct {
	Symbol             string `json:"symbol"`             // Symbol 是合约交易对，例如 BTCUSDT。
	QuoteVolume        string `json:"quoteVolume"`        // QuoteVolume 是 24 小时报价币成交额。
	LastPrice          string `json:"lastPrice"`          // LastPrice 是最新成交价。
	PriceChangePercent string `json:"priceChangePercent"` // PriceChangePercent 是 24 小时涨跌幅百分比。
}

// premiumIndexItem 是 Binance premiumIndex 返回的资金费率结构。
type premiumIndexItem struct {
	Symbol          string `json:"symbol"`          // Symbol 是合约交易对。
	LastFundingRate string `json:"lastFundingRate"` // LastFundingRate 是当前资金费率的小数形式。
}

// marketCapsResponse 是 Binance 市值补充接口的最小响应结构。
type marketCapsResponse struct {
	Data []struct { // Data 是币种市值列表。
		Name      string   `json:"name"`
		MarketCap *float64 `json:"marketCap"`
	} `json:"data"`
}

// spotExchangeInfoResponse 是 Binance 现货交易规则接口的最小响应结构。
type spotExchangeInfoResponse struct {
	Symbols []struct { // Symbols 是现货交易对列表。
		BaseAsset  string `json:"baseAsset"`
		QuoteAsset string `json:"quoteAsset"`
		Status     string `json:"status"`
	} `json:"symbols"`
}

// oiHistoryItem 是 Binance OI 历史接口的最小响应结构。
type oiHistoryItem struct {
	SumOpenInterestValue string `json:"sumOpenInterestValue"` // SumOpenInterestValue 是某个时间点的未平仓合约美元价值。
}

// formatOptional 将可空浮点数格式化为信号 reason 可读文本。
func formatOptional(value *float64) string {
	if value == nil {
		return "-"
	}
	return fmt.Sprintf("%+0.3f", *value)
}

func (s *Scanner) getSquareDiscussion(ctx context.Context, coin string) (int64, int64, string) {
	params := url.Values{}
	params.Set("hashtag", "#"+strings.ToLower(coin))
	params.Set("pageIndex", "1")
	params.Set("pageSize", "1")
	params.Set("orderBy", "HOT")
	var response struct {
		Data struct {
			Hashtag struct {
				ContentCount int64 `json:"contentCount"`
				ViewCount    int64 `json:"viewCount"`
			} `json:"hashtag"`
		} `json:"data"`
	}
	if err := s.getJSON(ctx, "https://www.binance.com/bapi/composite/v4/friendly/pgc/content/queryByHashtag", params, &response); err != nil {
		return 0, 0, fmt.Sprintf("square_discussion_failed:%s:%v", coin, err)
	}
	return response.Data.Hashtag.ContentCount, response.Data.Hashtag.ViewCount, ""
}
