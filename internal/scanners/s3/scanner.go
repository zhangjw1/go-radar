package s3

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"go-radar/internal/scanners"

	"gorm.io/gorm"
)

// Scanner 实现 S3 合约热度与异常扫描器。
//
// 业务目标：综合 Binance 合约成交量、资金费率、OI、CoinGecko trending 等信号，
// 找出“热度上升 + 持仓/资金费率异常”的合约标的。
type Scanner struct {
	db     *gorm.DB     // db 当前主要保留给后续历史对比扩展使用。
	client *http.Client // client 是带代理和超时配置的 HTTP 客户端。
}

// marketData 是 S3 对单个合约标的聚合后的市场画像。
//
// 业务上它把多个外部接口的指标压缩成统一输入，供 BuildSignalTypes 和 scoreSignal
// 判断是否生成 heat、oi_anomaly、heat_plus_oi 等信号。
type marketData struct {
	Symbol     string  // Symbol 是 Binance 合约交易对，例如 BTCUSDT。
	Coin       string  // Coin 是去掉 USDT 后的币种符号。
	Price      float64 // Price 是最新价格。
	PxChg      float64 // PxChg 是 24 小时价格涨跌幅百分比。
	Vol        float64 // Vol 是 24 小时报价币成交额。
	FundingPct float64 // FundingPct 是当前资金费率百分比。
	OIUSD      float64 // OIUSD 是当前未平仓合约美元价值。
	OID6H      float64 // OID6H 是 6 小时 OI 变化百分比。
	EstMCap    float64 // EstMCap 是市值或根据成交额/OI 估算的市值。
	Heat       float64 // Heat 是外部热度和成交量异动累加出的热度分。
	InCG       bool    // InCG 表示是否出现在 CoinGecko trending。
	InSquare   bool    // InSquare 预留给 Binance Square 热度源。
	VolSurge   bool    // VolSurge 表示当前成交额相对历史是否放大。
}

// NewScanner 创建 S3 扫描器实例。
func NewScanner(db *gorm.DB) *Scanner {
	return &Scanner{db: db, client: scanners.NewHTTPClient()}
}

// Scan 执行一次 S3 扫描：聚合热度、成交量、资金费率和 OI，生成异常信号。
func (s *Scanner) Scan(ctx context.Context) (scanners.Result, error) {
	result := scanners.Result{ScannerName: "s3", Metadata: map[string]any{}}

	var tickers []ticker24h
	if err := scanners.GetJSON(ctx, s.client, "https://fapi.binance.com/fapi/v1/ticker/24hr", nil, &tickers); err != nil {
		return result, err
	}
	tickerMap := make(map[string]ticker24h)
	for _, item := range tickers {
		if strings.HasSuffix(item.Symbol, "USDT") {
			tickerMap[item.Symbol] = item
		}
	}

	var premiums []premiumIndexItem
	if err := scanners.GetJSON(ctx, s.client, "https://fapi.binance.com/fapi/v1/premiumIndex", nil, &premiums); err != nil {
		return result, err
	}
	fundingMap := make(map[string]float64)
	for _, item := range premiums {
		if strings.HasSuffix(item.Symbol, "USDT") {
			fundingMap[item.Symbol] = scanners.ParseFloat(item.LastFundingRate) * 100
		}
	}

	marketCaps, warnings := s.fetchMarketCaps(ctx)
	result.Warnings = append(result.Warnings, warnings...)
	cgTrending, warnings := s.fetchCGTrending(ctx)
	result.Warnings = append(result.Warnings, warnings...)
	squareTrending := map[string]bool{}
	if scanners.EnvBool("ENABLE_BINANCE_SQUARE", false) {
		result.Warnings = append(result.Warnings, "Binance Square provider is reserved for a future phase.")
	}

	heatMap := make(map[string]float64)
	for symbol := range cgTrending {
		heatMap[symbol] += 30
	}

	symbolsByVolume := make([]ticker24h, 0, len(tickerMap))
	for _, item := range tickerMap {
		symbolsByVolume = append(symbolsByVolume, item)
	}
	sort.Slice(symbolsByVolume, func(i, j int) bool {
		return scanners.ParseFloat(symbolsByVolume[i].QuoteVolume) > scanners.ParseFloat(symbolsByVolume[j].QuoteVolume)
	})

	volumeSurgeCoins := make(map[string]bool)
	volumeLookbackLimit := scanners.EnvInt("S3_VOLUME_LOOKBACK_LIMIT", 80)
	volumeCandidates := []ticker24h{}
	for _, ticker := range symbolsByVolume {
		if scanners.ParseFloat(ticker.QuoteVolume) >= scanners.EnvFloat("S3_MIN_VOL_USD", 20_000_000) {
			volumeCandidates = append(volumeCandidates, ticker)
			if len(volumeCandidates) >= volumeLookbackLimit {
				break
			}
		}
	}
	for _, ticker := range volumeCandidates {
		previousVolumes, err := s.fetchPreviousVolumes(ctx, ticker.Symbol)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("klines_failed:%s:%v", ticker.Symbol, err))
			continue
		}
		surged, ratio := DetectVolumeSurge(scanners.ParseFloat(ticker.QuoteVolume), previousVolumes, scanners.EnvFloat("S3_VOL_SURGE_MULT", 2.5))
		if surged {
			coin := strings.TrimSuffix(ticker.Symbol, "USDT")
			volumeSurgeCoins[coin] = true
			heatMap[coin] += math.Min(ratio*10, 50)
		}
	}

	scanSymbols := make(map[string]bool)
	for coin := range heatMap {
		symbol := coin + "USDT"
		if _, ok := tickerMap[symbol]; ok {
			scanSymbols[symbol] = true
		}
	}
	topLimit := scanners.EnvInt("S3_TOP_VOLUME_LIMIT", 100)
	for i, ticker := range symbolsByVolume {
		if i >= topLimit {
			break
		}
		scanSymbols[ticker.Symbol] = true
	}

	oiMap := make(map[string]map[string]float64)
	for symbol := range scanSymbols {
		oi, err := s.fetchOI(ctx, symbol)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("oi_failed:%s:%v", symbol, err))
			continue
		}
		if oi != nil {
			oiMap[symbol] = oi
		}
	}

	coinData := make(map[string]marketData)
	for symbol, ticker := range tickerMap {
		coin := strings.TrimSuffix(symbol, "USDT")
		oi := oiMap[symbol]
		oiUSD := oi["oi_usd"]
		estMCap, ok := marketCaps[coin]
		if !ok {
			vol := scanners.ParseFloat(ticker.QuoteVolume)
			if oiUSD > 0 {
				estMCap = math.Max(vol*0.3, oiUSD*2)
			} else {
				estMCap = vol * 0.3
			}
		}
		coinData[symbol] = marketData{
			Symbol:     symbol,
			Coin:       coin,
			Price:      scanners.ParseFloat(ticker.LastPrice),
			PxChg:      scanners.ParseFloat(ticker.PriceChangePercent),
			Vol:        scanners.ParseFloat(ticker.QuoteVolume),
			FundingPct: fundingMap[symbol],
			OIUSD:      oiUSD,
			OID6H:      oi["d6h"],
			EstMCap:    estMCap,
			Heat:       heatMap[coin],
			InCG:       cgTrending[coin],
			InSquare:   squareTrending[coin],
			VolSurge:   volumeSurgeCoins[coin],
		}
	}

	minOIDelta := scanners.EnvFloat("S3_MIN_OI_DELTA_PCT", 3)
	interestingCount := 0
	for symbol, data := range coinData {
		if !(data.Heat > 0 || math.Abs(data.OID6H) >= minOIDelta || data.FundingPct < -0.01) {
			continue
		}
		interestingCount++
		raw := map[string]any{
			"symbol":      symbol,
			"coin":        data.Coin,
			"price":       data.Price,
			"px_chg":      data.PxChg,
			"vol":         data.Vol,
			"funding_pct": data.FundingPct,
			"oi_usd":      data.OIUSD,
			"oi_d6h":      data.OID6H,
			"est_mcap":    data.EstMCap,
			"heat":        data.Heat,
			"in_cg":       data.InCG,
			"in_square":   data.InSquare,
			"vol_surge":   data.VolSurge,
		}
		result.Snapshots = append(result.Snapshots, scanners.SnapshotPayload{
			Source:     "s3",
			Chain:      "binance_perp",
			Address:    strings.ToLower(symbol),
			Symbol:     data.Coin,
			Name:       data.Coin,
			Price:      &data.Price,
			MC:         &data.EstMCap,
			Volume:     &data.Vol,
			FundingPct: &data.FundingPct,
			OIUSD:      &data.OIUSD,
			OID6H:      &data.OID6H,
			Raw:        raw,
		})
		tags := []string{}
		if data.InCG {
			tags = append(tags, "cg_trending")
		}
		if data.InSquare {
			tags = append(tags, "binance_square")
		}
		if data.VolSurge {
			tags = append(tags, "vol_surge")
		}
		for _, signalType := range BuildSignalTypes(data.Heat, data.OID6H, data.FundingPct, minOIDelta) {
			priority, score, reason := scoreSignal(signalType, data)
			result.Signals = append(result.Signals, scanners.SignalPayload{
				Source:     "s3",
				Chain:      "binance_perp",
				Address:    strings.ToLower(symbol),
				Symbol:     data.Coin,
				Name:       data.Coin,
				SignalType: signalType,
				Priority:   priority,
				Score:      math.Round(score*100) / 100,
				Reason:     reason,
				Tags:       tags,
				Raw:        raw,
				Token: &scanners.TokenPayload{
					Chain:          "binance_perp",
					Address:        strings.ToLower(symbol),
					Symbol:         data.Coin,
					Name:           data.Coin,
					NarrativeTheme: strings.ToLower(data.Coin),
					NarrativeTags:  tags,
				},
			})
		}
	}

	result.Metadata = map[string]any{
		"tracked_symbols":      len(coinData),
		"interesting_symbols":  interestingCount,
		"cg_trending_count":    len(cgTrending),
		"volume_surge_count":   len(volumeSurgeCoins),
		"warnings":             result.Warnings,
	}
	return result, nil
}

// scoreSignal 根据 S3 信号类型和市场画像计算优先级、分数与可读原因。
func scoreSignal(signalType string, data marketData) (string, float64, string) {
	priority := "low"
	score := data.Heat
	reason := "Heat signal"
	switch signalType {
	case "heat_plus_oi":
		priority = "high"
		score += data.OID6H * 2
		reason = fmt.Sprintf("Heat + OI rising %+0.1f%%", data.OID6H)
	case "heat_plus_negative_funding":
		priority = "medium"
		score += math.Abs(data.FundingPct) * 600
		reason = fmt.Sprintf("Heat + negative funding %+0.3f%%", data.FundingPct)
	case "oi_anomaly":
		priority = "medium"
		score = math.Abs(data.OID6H) * 8
		reason = fmt.Sprintf("OI anomaly %+0.1f%%", data.OID6H)
	case "heat":
		if data.VolSurge {
			priority = "medium"
			score += 10
		}
		reason = "CoinGecko trending / volume signal"
	}
	return priority, score, reason
}

// fetchMarketCaps 从 Binance 补充市值，缺失时 S3 会用成交额/OI 做估算。
func (s *Scanner) fetchMarketCaps(ctx context.Context) (map[string]float64, []string) {
	var response marketCapsResponse
	if err := scanners.GetJSON(ctx, s.client, "https://www.binance.com/bapi/composite/v1/public/marketing/symbol/list", nil, &response); err != nil {
		return map[string]float64{}, []string{fmt.Sprintf("market_caps_failed:%v", err)}
	}
	marketCaps := make(map[string]float64)
	for _, item := range response.Data {
		if item.Name != "" && item.MarketCap != nil {
			marketCaps[item.Name] = *item.MarketCap
		}
	}
	return marketCaps, nil
}

// fetchCGTrending 拉取 CoinGecko trending，作为热度来源之一。
func (s *Scanner) fetchCGTrending(ctx context.Context) (map[string]bool, []string) {
	var response cgTrendingResponse
	if err := scanners.GetJSON(ctx, s.client, "https://api.coingecko.com/api/v3/search/trending", nil, &response); err != nil {
		return map[string]bool{}, []string{fmt.Sprintf("cg_trending_failed:%v", err)}
	}
	symbols := make(map[string]bool)
	for _, coin := range response.Coins {
		symbol := strings.ToUpper(coin.Item.Symbol)
		if symbol != "" {
			symbols[symbol] = true
		}
	}
	return symbols, nil
}

// fetchPreviousVolumes 拉取日 K 历史成交额，用于判断成交量是否显著放大。
func (s *Scanner) fetchPreviousVolumes(ctx context.Context, symbol string) ([]float64, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("interval", "1d")
	params.Set("limit", "8")
	var rows [][]any
	if err := scanners.GetJSON(ctx, s.client, "https://fapi.binance.com/fapi/v1/klines", params, &rows); err != nil {
		return nil, err
	}
	previous := []float64{}
	for i, row := range rows {
		if i == len(rows)-1 || len(row) <= 7 {
			continue
		}
		previous = append(previous, anyToFloat(row[7]))
	}
	return previous, nil
}

// fetchOI 拉取最近 OI 序列，并计算 1 小时和 6 小时变化。
func (s *Scanner) fetchOI(ctx context.Context, symbol string) (map[string]float64, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("period", "1h")
	params.Set("limit", "6")
	var rows []oiHistoryItem
	if err := scanners.GetJSON(ctx, s.client, "https://fapi.binance.com/futures/data/openInterestHist", params, &rows); err != nil {
		return nil, err
	}
	if len(rows) < 2 {
		return nil, nil
	}
	current := scanners.ParseFloat(rows[len(rows)-1].SumOpenInterestValue)
	prev1h := scanners.ParseFloat(rows[len(rows)-2].SumOpenInterestValue)
	prev6h := scanners.ParseFloat(rows[0].SumOpenInterestValue)
	d1h := 0.0
	d6h := 0.0
	if prev1h > 0 {
		d1h = (current - prev1h) / prev1h * 100
	}
	if prev6h > 0 {
		d6h = (current - prev6h) / prev6h * 100
	}
	return map[string]float64{"oi_usd": current, "d1h": d1h, "d6h": d6h}, nil
}

// anyToFloat 将 Binance kline 响应中的混合类型数值转为 float64。
func anyToFloat(value any) float64 {
	switch typed := value.(type) {
	case string:
		return scanners.ParseFloat(typed)
	case float64:
		return typed
	default:
		return 0
	}
}

// ticker24h 是 Binance 24 小时行情结构，是 S3 成交额和价格的来源。
type ticker24h struct {
	Symbol             string `json:"symbol"`             // Symbol 是合约交易对。
	QuoteVolume        string `json:"quoteVolume"`        // QuoteVolume 是 24 小时报价币成交额。
	LastPrice          string `json:"lastPrice"`          // LastPrice 是最新价格。
	PriceChangePercent string `json:"priceChangePercent"` // PriceChangePercent 是 24 小时价格涨跌幅。
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

// cgTrendingResponse 是 CoinGecko trending 接口的最小响应结构。
type cgTrendingResponse struct {
	Coins []struct { // Coins 是当前 trending 币种列表。
		Item struct {
			Symbol string `json:"symbol"`
		} `json:"item"`
	} `json:"coins"`
}

// oiHistoryItem 是 Binance OI 历史接口的最小响应结构。
type oiHistoryItem struct {
	SumOpenInterestValue string `json:"sumOpenInterestValue"` // SumOpenInterestValue 是某个时间点的未平仓合约美元价值。
}
