package s5

import (
	"math"
	"regexp"
	"strings"
)

// MomentumRow 是 S5 动量判断使用的一条历史状态。
//
// 业务上它来自 snapshots 表中的历史 S5 快照，用于判断市值是否连续抬升、
// 交易量和买入次数是否同步保持。
type MomentumRow struct {
	MC     float64 // MC 是该轮快照的市值。
	Volume float64 // Volume 是该轮快照的成交额。
	Price  float64 // Price 是该轮快照的价格。
	Buys1H float64 // Buys1H 是该轮快照的 1 小时买入次数。
}

// MomentumResult 是 S5 连续上涨检测的业务结论。
type MomentumResult struct {
	Triggered bool    // Triggered 表示是否满足动量信号条件。
	Reason    string  // Reason 是未触发或特殊状态的机器可读原因。
	PctGain   float64 // PctGain 是连续上涨区间内累计涨幅百分比。
	BuysOK    bool    // BuysOK 表示买入次数没有明显衰减。
}

// EvaluateMomentum 判断当前 token 是否出现连续多轮市值上涨。
func EvaluateMomentum(historicalRows []MomentumRow, currentRow MomentumRow, consecutiveUp int, minGainPct float64) MomentumResult {
	if consecutiveUp <= 1 {
		consecutiveUp = 2
	}
	start := len(historicalRows) - (consecutiveUp - 1)
	if start < 0 {
		start = 0
	}
	recent := append([]MomentumRow{}, historicalRows[start:]...)
	recent = append(recent, currentRow)
	if len(recent) < consecutiveUp {
		return MomentumResult{Triggered: false, Reason: "not_enough_history"}
	}

	if len(recent) >= 2 {
		last := recent[len(recent)-2]
		if last.MC == currentRow.MC && last.Volume == currentRow.Volume && last.Price == currentRow.Price {
			return MomentumResult{Triggered: false, Reason: "no_state_change"}
		}
	}

	buysOK := true
	for i := 1; i < len(recent); i++ {
		prev := recent[i-1]
		curr := recent[i]
		if prev.MC <= 0 || curr.MC <= prev.MC {
			return MomentumResult{Triggered: false, Reason: "not_consecutive_up"}
		}
		if prev.Buys1H > 0 && curr.Buys1H < prev.Buys1H*0.8 {
			buysOK = false
		}
	}

	pctGain := 0.0
	firstMC := recent[0].MC
	lastMC := recent[len(recent)-1].MC
	if firstMC > 0 {
		pctGain = (lastMC - firstMC) / firstMC * 100
	}
	if pctGain < minGainPct {
		return MomentumResult{Triggered: false, Reason: "gain_too_small", PctGain: pctGain}
	}
	return MomentumResult{Triggered: true, PctGain: pctGain, BuysOK: buysOK}
}

// ScoreDiscoverySignal 为 S5 叙事发现类信号评分。
func ScoreDiscoverySignal(stars int, mc float64, liq float64) float64 {
	base := float64(35 + stars*10)
	if mc > 0 && mc < 500_000 {
		base += 10
	}
	if liq > 50_000 {
		base += 5
	}
	return math.Round(base*100) / 100
}

// ScoreMomentumSignal 为 S5 动量类信号评分。
func ScoreMomentumSignal(stars int, pctGain float64, smartMoney int64) float64 {
	base := float64(50+stars*10) + math.Min(pctGain, 30)
	if smartMoney > 0 {
		base += math.Min(float64(smartMoney), 10)
	}
	return math.Round(base*100) / 100
}

// StarsToPriority 将 S5 内部星级映射为通用 high/medium/low 优先级。
func StarsToPriority(stars int) string {
	if stars >= 3 {
		return "high"
	}
	if stars == 2 {
		return "medium"
	}
	return "low"
}

// NormalizeTheme 从 token 名称和符号里提取更干净的叙事主题文本。
func NormalizeTheme(name string, symbol string) string {
	text := strings.ToLower(name + " " + symbol)
	text = regexp.MustCompile(`\d+x?`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`[^a-z\s]`).ReplaceAllString(text, " ")
	noise := map[string]bool{
		"coin": true, "token": true, "meme": true, "pepe": true, "wojak": true,
		"finance": true, "protocol": true, "swap": true, "dao": true, "defi": true,
		"ai": true, "meta": true, "verse": true,
	}
	words := []string{}
	for _, word := range strings.Fields(text) {
		if len(word) > 1 && !noise[word] {
			words = append(words, word)
		}
		if len(words) >= 6 {
			break
		}
	}
	return strings.Join(words, " ")
}

// ClassifyNarrative 根据名称、符号和链识别 S5 关注的早期 meme/名人/生态叙事。
func ClassifyNarrative(name string, symbol string, chain string) (string, []string) {
	text := strings.ToLower(name + " " + symbol)
	for _, pattern := range []string{`airdrop`, `presale`, `1000x`, `100x`, `safe\s*moon`, `baby\s*\w+`, `porn`, `xxx`, `scam`, `rug\s*pull`} {
		if regexp.MustCompile(pattern).MatchString(text) {
			return "spam", nil
		}
	}
	if matched := matchKeywords(text, []string{"musk", "elon", "elonmusk", "spacex", "starship", "tesla", "cybertruck", "neuralink", "xai", "grok", "floki", "shiba", "dogefather", "mars", "trump", "maga", "potus", "melania", "barron", "ivanka", "covfefe"}); len(matched) > 0 {
		if isKnownNarrativeChain(chain) {
			return "musk_trump", matched
		}
	}
	if matched := matchKeywords(text, []string{"cz", "changpeng", "zhao", "czbinance", "heyi", "yi he", "binance", "bnb", "pancake", "pancakeswap", "giggle academy", "bnb chain", "yzi", "fourmeme", "4meme"}); len(matched) > 0 {
		if strings.EqualFold(chain, "bsc") {
			return "binance_cz", matched
		}
		return "binance_cz_wrong_chain", matched
	}
	if matched := matchKeywords(text, []string{"vitalik", "buterin", "sam altman", "satoshi", "blackrock", "coinbase", "justin sun", "larry fink", "mrbeast", "drake", "kanye", "snoop dogg", "etf", "halving"}); len(matched) > 0 {
		return "celebrity_viral", matched
	}
	return "check_novelty", nil
}

// matchKeywords 返回命中的关键词，最多保留前三个用于标签展示。
func matchKeywords(text string, keywords []string) []string {
	matched := []string{}
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			matched = append(matched, keyword)
			if len(matched) >= 3 {
				break
			}
		}
	}
	return matched
}

// isKnownNarrativeChain 判断某类广谱叙事是否出现在可信链上。
func isKnownNarrativeChain(chain string) bool {
	switch strings.ToLower(chain) {
	case "eth", "ethereum", "sol", "solana", "bsc", "base":
		return true
	default:
		return false
	}
}
