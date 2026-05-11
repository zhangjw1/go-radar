package s1

import (
	"crypto/md5"
	"fmt"
	"regexp"
	"strings"
)

var triggerKeywords = []string{"will list", "hodler airdrops", "airdrop", "binance alpha"}
var excludeKeywords = []string{
	"delisting", "delist", "deprecate", "maintenance", "launchpool", "megadrop", "buyback",
	"perpetual contract", "futures will launch", "coin-margined", "margin will add",
	"trading bots services", "trading pairs", "will remove", "alpha will remove",
	"trade & win", "win a share", "campaign", "deposit fee discount", "limited time",
	"exclusive:", "wallet campaign",
}
var alphaBoxKeywords = []string{"alpha box", "mystery box"}
var binanceDarlingKeywords = []string{"yzi labs", "binance labs"}
var hotNarratives = map[string]bool{"defi_perp": true, "ai_agent": true, "ai_defi": true, "defai": true, "zk_proof": true}
var tier1VCs = []string{"binance labs", "yzi labs", "coinbase ventures", "a16z", "andreessen horowitz", "paradigm", "polychain", "multicoin", "pantera", "dragonfly"}

var symbolRE = regexp.MustCompile(`\(([A-Z0-9]{2,10})\)`)
var nameRE = regexp.MustCompile(`(?i)(?:List|list|Launch|launch|featured)\s+([A-Za-z0-9 ]+?)\s*[\(]`)

// IsTrigger 判断公告标题是否属于 S1 关心的新币/空投/Alpha 上线事件。
func IsTrigger(title string) bool {
	lowered := strings.ToLower(title)
	if containsAny(lowered, excludeKeywords) || containsAny(lowered, alphaBoxKeywords) {
		return false
	}
	if strings.Contains(lowered, "will list") {
		return true
	}
	if strings.Contains(lowered, "hodler airdrops") || (strings.Contains(lowered, "airdrop") && strings.Contains(lowered, "binance")) {
		return true
	}
	if strings.Contains(lowered, "binance alpha") && containsAny(lowered, []string{"feature", "featured", "list", "launch"}) {
		return true
	}
	return containsAny(lowered, triggerKeywords)
}

// ExtractSymbol 从公告标题括号中提取代币符号。
func ExtractSymbol(title string) string {
	match := symbolRE.FindStringSubmatch(title)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

// ExtractName 从公告标题中提取项目名称。
func ExtractName(title string) string {
	match := nameRE.FindStringSubmatch(title)
	if len(match) > 1 {
		return strings.TrimSpace(match[1])
	}
	return ""
}

// BuildArticleDedupeKey 为公告生成稳定去重键，优先使用 Binance article code。
func BuildArticleDedupeKey(code string, title string, symbol string, launchDate string) string {
	code = strings.TrimSpace(code)
	if code != "" {
		return "s1|article|" + code
	}
	sum := md5.Sum([]byte(title))
	hash := fmt.Sprintf("%x", sum)[:12]
	return fmt.Sprintf("s1|symbol|%s|%s|%s", strings.ToUpper(symbol), launchDate, hash)
}

// DetectAnnouncementKind 将公告标题归类为 listing、airdrop、alpha 或 other。
func DetectAnnouncementKind(title string) string {
	lowered := strings.ToLower(title)
	if strings.Contains(lowered, "hodler airdrops") || strings.Contains(lowered, "airdrop") {
		return "airdrop"
	}
	if strings.Contains(lowered, "binance alpha") {
		return "alpha"
	}
	if strings.Contains(lowered, "will list") {
		return "listing"
	}
	return "other"
}

func InferNarrative(rawText string, categories []string, description string) (string, string, []string, bool) {
	categoryText := strings.ToLower(strings.Join(categories, " "))
	titleText := strings.ToLower(rawText)
	vcs := []string{}
	for _, category := range categories {
		if strings.Contains(strings.ToLower(category), "portfolio") {
			vcs = append(vcs, strings.ReplaceAll(category, " Portfolio", ""))
		}
	}
	isDarling := containsAny(titleText, binanceDarlingKeywords) || containsAny(categoryText, binanceDarlingKeywords)
	narrative := "unknown"
	switch {
	case strings.Contains(categoryText, "defi"):
		narrative = "defi"
	case strings.Contains(categoryText, "ai"):
		narrative = "ai_agent"
	case strings.Contains(categoryText, "gaming") || strings.Contains(categoryText, "gamefi"):
		narrative = "gamefi"
	case strings.Contains(categoryText, "meme"):
		narrative = "meme"
	case strings.Contains(categoryText, "rwa") || strings.Contains(categoryText, "real world"):
		narrative = "rwa"
	}
	narrativeDesc := description
	if len(narrativeDesc) > 100 {
		narrativeDesc = narrativeDesc[:100]
	}
	if len(vcs) > 6 {
		vcs = vcs[:6]
	}
	return narrative, strings.TrimSpace(narrativeDesc), vcs, isDarling
}

func RateProject(circMCap float64, fdv float64, vcs []string, narrative string, isDarling bool) (string, string) {
	vcsLower := make([]string, 0, len(vcs))
	for _, vc := range vcs {
		vcsLower = append(vcsLower, strings.ToLower(vc))
	}
	tier1Count := 0
	for _, tier1 := range tier1VCs {
		for _, vc := range vcsLower {
			if strings.Contains(vc, tier1) {
				tier1Count++
				break
			}
		}
	}
	hot := hotNarratives[narrative]
	if isDarling {
		return "S", "Binance darling"
	}
	if hot && tier1Count >= 1 && fdv < 500_000_000 {
		return "S", "Hot narrative + Tier1 VC"
	}
	if tier1Count >= 2 && circMCap < 50_000_000 && fdv < 300_000_000 {
		return "S", "Multiple Tier1 mid cap"
	}
	if tier1Count >= 1 && circMCap < 10_000_000 && fdv < 100_000_000 {
		return "S", "Tier1 micro cap"
	}
	if hot && circMCap < 10_000_000 && fdv < 50_000_000 {
		return "S", "Hot narrative micro cap"
	}
	if tier1Count >= 1 && circMCap < 20_000_000 && fdv < 200_000_000 {
		return "A", "Tier1 small cap"
	}
	if circMCap < 50_000_000 && fdv < 500_000_000 {
		return "B", "Mid cap"
	}
	return "C", "Large cap / weak signal"
}

func ScoreTier(tier string, fdv float64, isDarling bool) float64 {
	base := map[string]float64{"S": 92, "A": 80, "B": 62, "C": 40}[tier]
	if base == 0 {
		base = 40
	}
	if isDarling {
		base += 6
	}
	if fdv > 0 && fdv < 50_000_000 {
		base += 4
	}
	return base
}

func MapChain(chainName string) string {
	switch strings.ToLower(chainName) {
	case "ethereum":
		return "eth"
	case "binance-smart-chain":
		return "bsc"
	case "base":
		return "base"
	case "solana":
		return "sol"
	default:
		return "binance_alpha"
	}
}

func containsAny(text string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}
