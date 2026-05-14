package s3

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"go-radar/internal/scanners"

	"gorm.io/gorm"
)

var bjLocationS3 = time.FixedZone("Asia/Shanghai", 8*60*60)

// Scanner 实现 S3 合约热度与异常扫描器。
//
// 业务目标：综合 Binance 合约成交量、资金费率、OI、CoinGecko trending 等信号，
// 找出“热度上升 + 持仓/资金费率异常”的合约标的，并生成 Python 版同款整轮热度报告。
type Scanner struct {
	db     *gorm.DB     // db 用于保存热度历史、快照和信号。
	client *http.Client // client 是带代理和超时配置的 HTTP 客户端。
}

// marketData 是 S3 对单个合约标的聚合后的市场画像。
//
// 业务上它把多个外部接口的指标压缩成统一输入，供逐币信号和整轮报告共同使用。
type marketData struct {
	Symbol     string  // Symbol 是 Binance 合约交易对，例如 BTCUSDT。
	Coin       string  // Coin 是去掉 USDT 后的币种符号。
	Price      float64 // Price 是最新价格。
	PxChg      float64 // PxChg 是 24 小时价格涨跌幅百分比。
	Vol        float64 // Vol 是 24 小时报价币成交额。
	FundingPct float64 // FundingPct 是当前资金费率百分比。
	OIUSD      float64 // OIUSD 是当前未平仓合约美元价值。
	OID1H      float64 // OID1H 是 1 小时 OI 变化百分比。
	OID6H      float64 // OID6H 是 6 小时 OI 变化百分比。
	EstMCap    float64 // EstMCap 是市值或根据成交额/OI 估算出的市值。
	Heat       float64 // Heat 是外部热度和成交量异动累加出的热度分。
	InCG       bool    // InCG 表示是否出现在 CoinGecko trending。
	InSquare   bool    // InSquare 表示是否出现在 Binance Square 热榜。
	VolSurge   bool    // VolSurge 表示当前成交额相对历史是否放大。
}

// heatHistoryEntry 记录某个币第一次进入热度榜的时间。
type heatHistoryEntry struct {
	FirstSeen string  `json:"first_seen"`
	Price     float64 `json:"price"`
}

// chaseEntry 是 Python 版“追多”板块里的单行数据。
type chaseEntry struct {
	Data    marketData `json:"data"`
	FRDelta float64    `json:"fr_delta"`
	Trend   string     `json:"trend"`
	Rates   []float64  `json:"rates"`
}

// NewScanner 创建 S3 扫描器实例。
func NewScanner(db *gorm.DB) *Scanner {
	return &Scanner{db: db, client: scanners.NewHTTPClient()}
}

// Scan 执行一次 S3 扫描。
//
// 核心步骤与 Python 版保持一致：
// 1. 拉 Binance 合约行情和资金费率；
// 2. 补充真实市值、CoinGecko trending、成交量放大；
// 3. 扫描热度币 + Top 成交量标的的 OI；
// 4. 更新热度历史，生成首次上榜、热度榜、追多、OI 异动和 highlights 报告；
// 5. 保留 Go 版逐币信号，用于入库、页面展示和跨源共振。
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
	cgHeat, warnings := s.fetchCGTrending(ctx)
	result.Warnings = append(result.Warnings, warnings...)
	squareTrending := map[string]bool{}
	if scanners.EnvBool("ENABLE_BINANCE_SQUARE", false) {
		result.Warnings = append(result.Warnings, "square_provider_missing: docs/square_heat.py is not present in this project")
	}

	heatMap := make(map[string]float64)
	cgTrending := make(map[string]bool)
	for coin, score := range cgHeat {
		cgTrending[coin] = true
		heatMap[coin] += score
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

	for coin := range heatMap {
		if cgTrending[coin] && volumeSurgeCoins[coin] {
			heatMap[coin] += 20
		}
		if squareTrending[coin] && volumeSurgeCoins[coin] {
			heatMap[coin] += 20
		}
		if cgTrending[coin] && squareTrending[coin] && volumeSurgeCoins[coin] {
			heatMap[coin] += 30
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
			OID1H:      oi["d1h"],
			OID6H:      oi["d6h"],
			EstMCap:    estMCap,
			Heat:       heatMap[coin],
			InCG:       cgTrending[coin],
			InSquare:   squareTrending[coin],
			VolSurge:   volumeSurgeCoins[coin],
		}
	}

	reportSignal, reportMeta, reportWarnings := s.buildHeatReport(ctx, coinData)
	result.Warnings = append(result.Warnings, reportWarnings...)
	if reportSignal != nil {
		result.Signals = append(result.Signals, *reportSignal)
	}

	minOIDelta := scanners.EnvFloat("S3_MIN_OI_DELTA_PCT", 3)
	interestingCount := 0
	for symbol, data := range coinData {
		if !(data.Heat > 0 || math.Abs(data.OID6H) >= minOIDelta || data.FundingPct < -0.01) {
			continue
		}
		interestingCount++
		raw := rawForMarketData(symbol, data)
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
		tags := tagsForMarketData(data)
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
		"tracked_symbols":     len(coinData),
		"interesting_symbols": interestingCount,
		"cg_trending_count":   len(cgTrending),
		"volume_surge_count":  len(volumeSurgeCoins),
		"report":              reportMeta,
		"warnings":            result.Warnings,
	}
	return result, nil
}

func (s *Scanner) buildHeatReport(ctx context.Context, coinData map[string]marketData) (*scanners.SignalPayload, map[string]any, []string) {
	meta := map[string]any{}
	warnings := []string{}

	hotCoins := make([]marketData, 0)
	for _, data := range coinData {
		if data.Heat > 0 {
			hotCoins = append(hotCoins, data)
		}
	}
	sort.Slice(hotCoins, func(i, j int) bool {
		if hotCoins[i].Heat == hotCoins[j].Heat {
			return hotCoins[i].Vol > hotCoins[j].Vol
		}
		return hotCoins[i].Heat > hotCoins[j].Heat
	})

	history := map[string]heatHistoryEntry{}
	if _, err := scanners.LoadState(s.db, "s3", "heat_history", &history); err != nil {
		warnings = append(warnings, fmt.Sprintf("heat_history_load_failed:%v", err))
		history = map[string]heatHistoryEntry{}
	}
	nowBJ := time.Now().In(bjLocationS3)
	nowText := nowBJ.Format("2006-01-02 15:04")
	newEntries := []marketData{}
	for _, data := range hotCoins {
		if _, ok := history[data.Coin]; !ok {
			history[data.Coin] = heatHistoryEntry{FirstSeen: nowText, Price: data.PxChg}
			newEntries = append(newEntries, data)
		}
	}
	cutoff := nowBJ.AddDate(0, 0, -7).Format("2006-01-02")
	for coin, entry := range history {
		if entry.FirstSeen != "" && entry.FirstSeen < cutoff {
			delete(history, coin)
		}
	}
	if err := scanners.SaveState(s.db, "s3", "heat_history", history); err != nil {
		warnings = append(warnings, fmt.Sprintf("heat_history_save_failed:%v", err))
	}

	chase := []chaseEntry{}
	for symbol, data := range coinData {
		if data.PxChg <= 3 || data.FundingPct >= -0.005 || data.Vol <= 1_000_000 {
			continue
		}
		rates, err := s.fetchFundingHistory(ctx, symbol)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("funding_history_failed:%s:%v", symbol, err))
			rates = []float64{data.FundingPct}
		}
		previous := data.FundingPct
		if len(rates) >= 2 {
			previous = rates[len(rates)-2]
		}
		delta := data.FundingPct - previous
		chase = append(chase, chaseEntry{
			Data:    data,
			FRDelta: delta,
			Trend:   fundingTrend(delta),
			Rates:   lastFundingRates(rates, data.FundingPct),
		})
	}
	sort.Slice(chase, func(i, j int) bool {
		return chase[i].Data.FundingPct < chase[j].Data.FundingPct
	})

	oiAlerts := []marketData{}
	for _, data := range coinData {
		if math.Abs(data.OID6H) >= 8 && data.Heat == 0 {
			oiAlerts = append(oiAlerts, data)
		}
	}
	sort.Slice(oiAlerts, func(i, j int) bool {
		return math.Abs(oiAlerts[i].OID6H) > math.Abs(oiAlerts[j].OID6H)
	})

	highlights := buildHighlights(coinData, chase)
	reportText := formatHeatReport(nowBJ, newEntries, hotCoins, chase, oiAlerts, highlights)
	meta["hot_count"] = len(hotCoins)
	meta["new_count"] = len(newEntries)
	meta["chase_count"] = len(chase)
	meta["oi_alert_count"] = len(oiAlerts)
	meta["highlight_count"] = len(highlights)

	if !scanners.EnvBool("S3_ALWAYS_REPORT", true) && len(hotCoins) == 0 && len(chase) == 0 && len(oiAlerts) == 0 {
		return nil, meta, warnings
	}

	raw := map[string]any{
		"report_text": reportText,
		"hot_coins":   marketRows(hotCoins, 10),
		"new_entries": marketRows(newEntries, 20),
		"chase":       chaseRows(chase, 8),
		"oi_alerts":   marketRows(oiAlerts, 6),
		"highlights":  highlights,
	}
	priority := "medium"
	if len(newEntries) > 0 || len(chase) > 0 || len(oiAlerts) > 0 || len(highlights) > 0 {
		priority = "high"
	}
	score := float64(len(newEntries)*12 + len(chase)*8 + len(oiAlerts)*6 + len(highlights)*5)
	return &scanners.SignalPayload{
		Source:     "s3",
		Chain:      "binance_perp",
		Address:    "s3_heat_report",
		Symbol:     "S3",
		Name:       "S3 Heat Report",
		SignalType: "heat_report",
		Priority:   priority,
		Score:      score,
		Reason:     fmt.Sprintf("S3 heat report: hot=%d new=%d chase=%d oi=%d", len(hotCoins), len(newEntries), len(chase), len(oiAlerts)),
		Tags:       []string{"heat_report", "binance_futures"},
		Raw:        raw,
		ForcePush:  true,
		DedupeKey:  "s3|heat_report|" + time.Now().UTC().Format("200601021504"),
	}, meta, warnings
}

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

func (s *Scanner) fetchMarketCaps(ctx context.Context) (map[string]float64, []string) {
	var response marketCapsResponse
	if err := scanners.GetJSON(ctx, s.client, "https://www.binance.com/bapi/composite/v1/public/marketing/symbol/list", nil, &response); err != nil {
		return map[string]float64{}, []string{fmt.Sprintf("market_caps_failed:%v", err)}
	}
	marketCaps := make(map[string]float64)
	for _, item := range response.Data {
		if item.Name != "" && item.MarketCap != nil {
			marketCaps[strings.ToUpper(item.Name)] = *item.MarketCap
		}
	}
	return marketCaps, nil
}

func (s *Scanner) fetchCGTrending(ctx context.Context) (map[string]float64, []string) {
	var response cgTrendingResponse
	if err := scanners.GetJSON(ctx, s.client, "https://api.coingecko.com/api/v3/search/trending", nil, &response); err != nil {
		return map[string]float64{}, []string{fmt.Sprintf("cg_trending_failed:%v", err)}
	}
	symbols := make(map[string]float64)
	for _, coin := range response.Coins {
		symbol := strings.ToUpper(coin.Item.Symbol)
		if symbol == "" {
			continue
		}
		rank := coin.Item.Score
		score := math.Max(50-float64(rank)*3, 10)
		symbols[symbol] += score
	}
	return symbols, nil
}

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

func (s *Scanner) fetchFundingHistory(ctx context.Context, symbol string) ([]float64, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("limit", "5")
	var rows []fundingRateItem
	if err := scanners.GetJSON(ctx, s.client, "https://fapi.binance.com/fapi/v1/fundingRate", params, &rows); err != nil {
		return nil, err
	}
	rates := make([]float64, 0, len(rows))
	for _, row := range rows {
		rates = append(rates, scanners.ParseFloat(row.FundingRate)*100)
	}
	return rates, nil
}

func rawForMarketData(symbol string, data marketData) map[string]any {
	return map[string]any{
		"symbol":      symbol,
		"coin":        data.Coin,
		"price":       data.Price,
		"px_chg":      data.PxChg,
		"vol":         data.Vol,
		"funding_pct": data.FundingPct,
		"oi_usd":      data.OIUSD,
		"oi_d1h":      data.OID1H,
		"oi_d6h":      data.OID6H,
		"est_mcap":    data.EstMCap,
		"heat":        data.Heat,
		"in_cg":       data.InCG,
		"in_square":   data.InSquare,
		"vol_surge":   data.VolSurge,
	}
}

func tagsForMarketData(data marketData) []string {
	tags := []string{}
	if data.InSquare {
		tags = append(tags, "binance_square")
	}
	if data.InCG {
		tags = append(tags, "cg_trending")
	}
	if data.VolSurge {
		tags = append(tags, "vol_surge")
	}
	return tags
}

func sourcesForMarketData(data marketData) []string {
	sources := []string{}
	if data.InSquare {
		sources = append(sources, "Square")
	}
	if data.InCG {
		sources = append(sources, "CG")
	}
	if data.VolSurge {
		sources = append(sources, "Vol")
	}
	return sources
}

func marketRows(values []marketData, limit int) []map[string]any {
	if limit > 0 && len(values) > limit {
		values = values[:limit]
	}
	rows := make([]map[string]any, 0, len(values))
	for _, data := range values {
		rows = append(rows, map[string]any{
			"coin":        data.Coin,
			"mcap":        data.EstMCap,
			"px_chg":      data.PxChg,
			"funding_pct": data.FundingPct,
			"oi_d6h":      data.OID6H,
			"heat":        data.Heat,
			"sources":     sourcesForMarketData(data),
		})
	}
	return rows
}

func chaseRows(values []chaseEntry, limit int) []map[string]any {
	if limit > 0 && len(values) > limit {
		values = values[:limit]
	}
	rows := make([]map[string]any, 0, len(values))
	for _, item := range values {
		rows = append(rows, map[string]any{
			"coin":        item.Data.Coin,
			"mcap":        item.Data.EstMCap,
			"px_chg":      item.Data.PxChg,
			"funding_pct": item.Data.FundingPct,
			"fr_delta":    item.FRDelta,
			"trend":       item.Trend,
			"rates":       item.Rates,
		})
	}
	return rows
}

func buildHighlights(coinData map[string]marketData, chase []chaseEntry) []string {
	highlights := []string{}
	hotOI := []marketData{}
	for _, data := range coinData {
		if data.Heat > 0 && data.OID6H > 5 {
			hotOI = append(hotOI, data)
		}
	}
	sort.Slice(hotOI, func(i, j int) bool { return hotOI[i].OID6H > hotOI[j].OID6H })
	for _, data := range firstMarketData(hotOI, 3) {
		highlights = append(highlights, fmt.Sprintf("%s - heat plus OI %+0.0f%%", data.Coin, data.OID6H))
	}

	hotFuel := []marketData{}
	for _, data := range coinData {
		if data.Heat > 0 && data.FundingPct < -0.03 {
			hotFuel = append(hotFuel, data)
		}
	}
	sort.Slice(hotFuel, func(i, j int) bool { return hotFuel[i].FundingPct < hotFuel[j].FundingPct })
	for _, data := range firstMarketData(hotFuel, 2) {
		if !highlightContains(highlights, data.Coin) {
			highlights = append(highlights, fmt.Sprintf("%s - heat plus negative funding %.3f%%", data.Coin, data.FundingPct))
		}
	}

	for _, item := range firstChaseEntries(chase, 5) {
		if item.Trend != "accelerating_negative" || highlightContains(highlights, item.Data.Coin) {
			continue
		}
		highlights = append(highlights, fmt.Sprintf("%s - funding keeps worsening %.3f%%", item.Data.Coin, item.Data.FundingPct))
		if len(highlights) >= 5 {
			break
		}
	}
	if len(highlights) > 5 {
		return highlights[:5]
	}
	return highlights
}

func firstMarketData(values []marketData, limit int) []marketData {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}

func firstChaseEntries(values []chaseEntry, limit int) []chaseEntry {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}

func highlightContains(highlights []string, coin string) bool {
	for _, line := range highlights {
		if strings.Contains(line, coin+" ") || strings.HasPrefix(line, coin+" -") {
			return true
		}
	}
	return false
}

func formatHeatReport(now time.Time, newEntries []marketData, hotCoins []marketData, chase []chaseEntry, oiAlerts []marketData, highlights []string) string {
	lines := []string{
		"S3 Heat Long Radar",
		now.Format("2006-01-02 15:04") + " CST",
	}

	if len(newEntries) > 0 {
		lines = append(lines, "", "[New Heat Entries]", formatMarketTable(newEntries, 20, true))
	}

	lines = append(lines, "", "[Heat Rank]")
	if len(hotCoins) == 0 {
		lines = append(lines, "No active heat coins.")
	} else {
		lines = append(lines, formatMarketTable(hotCoins, 10, true))
	}

	lines = append(lines, "", "[Chase Long] Price up + negative funding")
	if len(chase) == 0 {
		lines = append(lines, "No matching targets.")
	} else {
		lines = append(lines, formatChaseTable(chase, 8))
	}

	if len(oiAlerts) > 0 {
		lines = append(lines, "", "[OI Anomaly] 6h OI change >= 8% without heat", formatMarketTable(oiAlerts, 6, false))
	}

	if len(highlights) > 0 {
		lines = append(lines, "", "[Worth Watching]")
		for _, line := range highlights {
			lines = append(lines, "  - "+line)
		}
	}

	lines = append(lines, "", "Square=Binance Square, CG=CoinGecko, Vol=volume surge.")
	lines = append(lines, "Negative funding often means crowded shorts and possible squeeze fuel.")
	return strings.Join(lines, "\n")
}

func formatMarketTable(values []marketData, limit int, withSources bool) string {
	if len(values) > limit {
		values = values[:limit]
	}
	lines := []string{"```", fmt.Sprintf("%-10s %8s %7s %s", "Coin", "MCap", "24h", "Source")}
	lines = append(lines, fmt.Sprintf("%-10s %8s %7s %s", "----------", "--------", "-------", "--------------------"))
	for _, data := range values {
		sourceText := "-"
		if withSources {
			sourceText = strings.Join(sourcesForMarketData(data), "/")
		} else if data.OID6H > 0 {
			sourceText = "OI+"
		} else {
			sourceText = "OI-"
		}
		extra := []string{}
		if math.Abs(data.OID6H) >= 3 {
			extra = append(extra, fmt.Sprintf("OI%+.0f%%", data.OID6H))
		}
		if data.FundingPct < -0.03 {
			extra = append(extra, fmt.Sprintf("FR%.2f%%", data.FundingPct))
		}
		if len(extra) > 0 {
			sourceText = strings.TrimSpace(sourceText + " " + strings.Join(extra, " "))
		}
		lines = append(lines, fmt.Sprintf("%-10s %8s %+6.0f%%  %s", data.Coin, compactUSD(data.EstMCap), data.PxChg, sourceText))
	}
	lines = append(lines, "```")
	return strings.Join(lines, "\n")
}

func formatChaseTable(values []chaseEntry, limit int) string {
	if len(values) > limit {
		values = values[:limit]
	}
	lines := []string{"```", fmt.Sprintf("%-10s %10s %20s %7s %8s", "Coin", "Funding", "Trend", "24h", "MCap")}
	lines = append(lines, fmt.Sprintf("%-10s %10s %20s %7s %8s", "----------", "----------", "--------------------", "-------", "--------"))
	for _, item := range values {
		data := item.Data
		lines = append(lines, fmt.Sprintf("%-10s %+9.3f%% %20s %+6.0f%% %8s", data.Coin, data.FundingPct, item.Trend, data.PxChg, compactUSD(data.EstMCap)))
	}
	lines = append(lines, "```")
	return strings.Join(lines, "\n")
}

func fundingTrend(delta float64) string {
	switch {
	case delta < -0.05:
		return "accelerating_negative"
	case delta < -0.01:
		return "turning_negative"
	case math.Abs(delta) < 0.01:
		return "flat"
	default:
		return "recovering"
	}
}

func lastFundingRates(rates []float64, fallback float64) []float64 {
	if len(rates) == 0 {
		return []float64{fallback}
	}
	if len(rates) > 3 {
		rates = rates[len(rates)-3:]
	}
	out := make([]float64, len(rates))
	copy(out, rates)
	return out
}

func compactUSD(value float64) string {
	abs := math.Abs(value)
	switch {
	case abs >= 1_000_000_000:
		return fmt.Sprintf("$%.1fB", value/1_000_000_000)
	case abs >= 1_000_000:
		return fmt.Sprintf("$%.0fM", value/1_000_000)
	case abs >= 1_000:
		return fmt.Sprintf("$%.0fK", value/1_000)
	default:
		return fmt.Sprintf("$%.0f", value)
	}
}

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

type ticker24h struct {
	Symbol             string `json:"symbol"`
	QuoteVolume        string `json:"quoteVolume"`
	LastPrice          string `json:"lastPrice"`
	PriceChangePercent string `json:"priceChangePercent"`
}

type premiumIndexItem struct {
	Symbol          string `json:"symbol"`
	LastFundingRate string `json:"lastFundingRate"`
}

type marketCapsResponse struct {
	Data []struct {
		Name      string   `json:"name"`
		MarketCap *float64 `json:"marketCap"`
	} `json:"data"`
}

type cgTrendingResponse struct {
	Coins []struct {
		Item struct {
			Symbol string `json:"symbol"`
			Score  int    `json:"score"`
		} `json:"item"`
	} `json:"coins"`
}

type oiHistoryItem struct {
	SumOpenInterestValue string `json:"sumOpenInterestValue"`
}

type fundingRateItem struct {
	FundingRate string `json:"fundingRate"`
}
