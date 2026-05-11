package s7

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"

	"go-radar/internal/model"
	"go-radar/internal/scanners"

	"gorm.io/gorm"
)

const defaultRPCURL = "https://eth.drpc.org"
const dexScreenerTokenURL = "https://api.dexscreener.com/latest/dex/tokens/"

// Scanner 实现 S7 Vitalik 钱包出站转账扫描器。
//
// 业务目标：监听 Vitalik 地址发出的 ERC20 Transfer 日志，识别转入 CEX、DEX、
// 流动性池等路径的潜在卖出/处置信号，并根据美元价值和接收方类型决定优先级。
type Scanner struct {
	db         *gorm.DB             // db 用于读取上一轮成功扫描到的区块位置。
	client     *http.Client         // client 是带代理和超时配置的 HTTP 客户端。
	tokenCache map[string]tokenInfo // tokenCache 缓存 ERC20 symbol/name/decimals，避免同轮扫描重复 eth_call。
}

// tokenInfo 是链上 ERC20 元数据的本地缓存对象。
//
// 业务上用于把原始 Transfer amount 转成真实 token 数量，并生成用户可读的信号文案。
type tokenInfo struct {
	Address  string // Address 是 token 合约地址。
	Symbol   string // Symbol 是 ERC20 symbol，读取失败时退化为短地址。
	Name     string // Name 是 ERC20 name，读取失败时退化为短地址。
	Decimals int    // Decimals 是 ERC20 decimals，读取失败时默认 18。
}

// ethLog 是 eth_getLogs 返回的 ERC20 Transfer 日志。
//
// 业务上每条日志代表 Vitalik 地址向某个 recipient 转出某个 token 的一次事件。
type ethLog struct {
	Address         string   `json:"address"`         // Address 是触发日志的 token 合约地址。
	Topics          []string `json:"topics"`          // Topics 保存 Transfer topic、from、to 等索引字段。
	Data            string   `json:"data"`            // Data 保存转账数量的 ABI 编码值。
	TransactionHash string   `json:"transactionHash"` // TransactionHash 是交易哈希，用于构建去重键和跳转链接。
	BlockNumber     string   `json:"blockNumber"`     // BlockNumber 是十六进制区块号。
	LogIndex        string   `json:"logIndex"`        // LogIndex 是同一交易内的日志序号。
}

// rpcResponse 是以太坊 JSON-RPC 的通用响应包装。
type rpcResponse struct {
	Result json.RawMessage `json:"result"` // Result 是不同 RPC 方法返回的原始 JSON 结果。
	Error  *struct { // Error 是 RPC 层错误，非空时本次调用失败。
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// dexScreenerResponse 是 DexScreener token 查询接口的最小响应结构。
//
// 业务上只需要价格和流动性，用流动性最高的交易对估算转账美元价值。
type dexScreenerResponse struct {
	Pairs []struct { // Pairs 是 DexScreener 返回的交易对列表。
		PriceUSD  string `json:"priceUsd"` // PriceUSD 是该交易对上的美元价格字符串。
		Liquidity struct { // Liquidity 用于选择最可靠的交易对。
			USD float64 `json:"usd"`
		} `json:"liquidity"`
	} `json:"pairs"`
}

// NewScanner 创建 S7 扫描器实例。
func NewScanner(db *gorm.DB) *Scanner {
	return &Scanner{
		db:         db,
		client:     scanners.NewHTTPClient(),
		tokenCache: map[string]tokenInfo{},
	}
}

// Scan 执行一次 S7 扫描：确定区块窗口、拉取 Transfer 日志并转换为卖出相关信号。
func (s *Scanner) Scan(ctx context.Context) (scanners.Result, error) {
	result := scanners.Result{ScannerName: "s7", Metadata: map[string]any{}}

	latestBlock, err := s.latestBlock(ctx)
	if err != nil {
		return result, err
	}
	lastScannedBlock := s.lastScannedBlock()
	fromBlock, toBlock := ComputeBlockWindow(latestBlock, lastScannedBlock, InitialLookbackBlocks, CheckpointOverlapBlocks)

	logs, err := s.fetchOutboundTransfers(ctx, fromBlock, toBlock)
	if err != nil {
		return result, err
	}
	sort.Slice(logs, func(i, j int) bool {
		leftBlock := HexToInt(logs[i].BlockNumber)
		rightBlock := HexToInt(logs[j].BlockNumber)
		if leftBlock == rightBlock {
			return HexToInt(logs[i].LogIndex) < HexToInt(logs[j].LogIndex)
		}
		return leftBlock < rightBlock
	})

	minUSD := scanners.EnvFloat("S7_MIN_NOTIFY_USD", 0)
	ignoredCount := 0
	for _, logItem := range logs {
		signal, ignored, warning := s.buildSignal(ctx, logItem, minUSD)
		if warning != "" {
			result.Warnings = append(result.Warnings, warning)
		}
		if ignored {
			ignoredCount++
			continue
		}
		if signal != nil {
			result.Signals = append(result.Signals, *signal)
		}
	}

	result.Metadata = map[string]any{
		"from_block":       fromBlock,
		"to_block":         toBlock,
		"latest_block":     latestBlock,
		"log_count":        len(logs),
		"sell_event_count": len(result.Signals),
		"ignored_count":    ignoredCount,
		"warnings":         result.Warnings,
	}
	return result, nil
}

// buildSignal 将单条 Transfer 日志转换为 S7 信号；不满足条件时返回 ignored。
func (s *Scanner) buildSignal(ctx context.Context, logItem ethLog, minUSD float64) (*scanners.SignalPayload, bool, string) {
	if len(logItem.Topics) < 3 {
		return nil, true, "log_missing_topics"
	}
	tokenAddress := strings.ToLower(logItem.Address)
	recipient := TopicToAddress(logItem.Topics[2])
	if recipient == "" {
		return nil, true, "log_missing_recipient"
	}

	recipientType, recipientName := ClassifyRecipient(recipient)
	if recipientType == "unknown" && s.isPool(ctx, recipient) {
		recipientType = "pool"
		recipientName = "Liquidity Pool"
	}

	info, warning := s.fetchTokenInfo(ctx, tokenAddress)
	amount := DecodeTransferValue(logItem.Data, info.Decimals)
	priceUSD, priceWarning := s.fetchTokenPriceUSD(ctx, tokenAddress)
	if priceWarning != "" {
		warning = appendWarning(warning, priceWarning)
	}

	var usdValue *float64
	if priceUSD != nil {
		value := amount * *priceUSD
		usdValue = &value
	}
	if minUSD > 0 && (usdValue == nil || *usdValue < minUSD) {
		return nil, true, warning
	}

	tags := []string{"vitalik", recipientType}
	if recipientName != "" {
		tags = append(tags, SlugTag(recipientName))
	}
	socialLinksJSON, _ := json.Marshal(map[string]string{
		"etherscan":    fmt.Sprintf("https://etherscan.io/token/%s", tokenAddress),
		"dexscreener": fmt.Sprintf("https://dexscreener.com/ethereum/%s", tokenAddress),
	})
	raw := map[string]any{
		"token":            tokenAddress,
		"recipient":        recipient,
		"recipient_type":   recipientType,
		"recipient_name":   recipientName,
		"amount":           amount,
		"price_usd":        priceUSD,
		"usd_value":        usdValue,
		"tx_hash":          logItem.TransactionHash,
		"log_index":        HexToInt(logItem.LogIndex),
		"block_number":     HexToInt(logItem.BlockNumber),
		"from":             VitalikAddress,
		"min_notify_usd":   minUSD,
		"transfer_topic":   TransferTopic,
		"scanner_language": "go",
	}
	signal := scanners.SignalPayload{
		Source:     "s7",
		Chain:      "ethereum",
		Address:    tokenAddress,
		Symbol:     info.Symbol,
		Name:       info.Name,
		SignalType: SignalType,
		Priority:   ResolvePriority(recipientType, usdValue),
		Score:      ScoreSellSignal(recipientType, usdValue),
		Reason:     buildReason(info.Symbol, amount, recipientType, recipientName, usdValue),
		Tags:       tags,
		Raw:        raw,
		DedupeKey:  fmt.Sprintf("s7|%s|%d", strings.ToLower(logItem.TransactionHash), HexToInt(logItem.LogIndex)),
		Token: &scanners.TokenPayload{
			Chain:           "ethereum",
			Address:         tokenAddress,
			Symbol:          info.Symbol,
			Name:            info.Name,
			NarrativeTheme:  "vitalik",
			NarrativeTags:   tags,
			Description:     "Token observed in an outbound Vitalik wallet transfer.",
			SocialLinksJSON: string(socialLinksJSON),
		},
	}
	return &signal, false, warning
}

// latestBlock 读取当前以太坊最新区块号。
func (s *Scanner) latestBlock(ctx context.Context) (int64, error) {
	var result string
	if err := s.rpcCall(ctx, "eth_blockNumber", []any{}, &result); err != nil {
		return 0, err
	}
	return HexToInt(result), nil
}

// fetchOutboundTransfers 获取 Vitalik 地址在指定区块窗口内的 ERC20 转出日志。
func (s *Scanner) fetchOutboundTransfers(ctx context.Context, fromBlock int64, toBlock int64) ([]ethLog, error) {
	filter := map[string]any{
		"fromBlock": fmt.Sprintf("0x%x", fromBlock),
		"toBlock":   fmt.Sprintf("0x%x", toBlock),
		"topics":    []any{TransferTopic, VitalikPadded},
	}
	var logs []ethLog
	if err := s.rpcCall(ctx, "eth_getLogs", []any{filter}, &logs); err != nil {
		return nil, err
	}
	return logs, nil
}

// fetchTokenInfo 读取并缓存 token symbol、name、decimals。
func (s *Scanner) fetchTokenInfo(ctx context.Context, address string) (tokenInfo, string) {
	address = strings.ToLower(address)
	if info, ok := s.tokenCache[address]; ok {
		return info, ""
	}
	info := tokenInfo{Address: address, Symbol: ShortenAddr(address), Name: ShortenAddr(address), Decimals: 18}
	warnings := []string{}

	if symbol, err := s.ethCallString(ctx, address, "0x95d89b41"); err == nil && symbol != "" {
		info.Symbol = strings.ToUpper(symbol)
	} else if err != nil {
		warnings = append(warnings, fmt.Sprintf("symbol_failed:%s:%v", address, err))
	}
	if name, err := s.ethCallString(ctx, address, "0x06fdde03"); err == nil && name != "" {
		info.Name = name
	} else if err != nil {
		warnings = append(warnings, fmt.Sprintf("name_failed:%s:%v", address, err))
	}
	if decimals, err := s.ethCallHex(ctx, address, "0x313ce567"); err == nil && decimals >= 0 && decimals <= 36 {
		info.Decimals = int(decimals)
	} else if err != nil {
		warnings = append(warnings, fmt.Sprintf("decimals_failed:%s:%v", address, err))
	}

	s.tokenCache[address] = info
	return info, strings.Join(warnings, ";")
}

// ethCallString 执行返回 string 的 ERC20 eth_call，例如 symbol/name。
func (s *Scanner) ethCallString(ctx context.Context, address string, data string) (string, error) {
	var result string
	err := s.rpcCall(ctx, "eth_call", []any{map[string]any{"to": address, "data": data}, "latest"}, &result)
	if err != nil {
		return "", err
	}
	return DecodeRPCString(result), nil
}

// ethCallHex 执行返回 uint/int 十六进制值的 ERC20 eth_call，例如 decimals。
func (s *Scanner) ethCallHex(ctx context.Context, address string, data string) (int64, error) {
	var result string
	err := s.rpcCall(ctx, "eth_call", []any{map[string]any{"to": address, "data": data}, "latest"}, &result)
	if err != nil {
		return 0, err
	}
	return HexToInt(result), nil
}

// isPool 用 UniswapV2 pair 的 token0() 方法粗略判断地址是否为流动性池。
func (s *Scanner) isPool(ctx context.Context, address string) bool {
	var result string
	if err := s.rpcCall(ctx, "eth_call", []any{map[string]any{"to": address, "data": "0x0dfe1681"}, "latest"}, &result); err != nil {
		return false
	}
	return len(strings.TrimSpace(result)) == 66
}

// fetchTokenPriceUSD 从 DexScreener 选择流动性最高的交易对估算 token 美元价格。
func (s *Scanner) fetchTokenPriceUSD(ctx context.Context, address string) (*float64, string) {
	var response dexScreenerResponse
	if err := scanners.GetJSON(ctx, s.client, dexScreenerTokenURL+address, nil, &response); err != nil {
		return nil, fmt.Sprintf("dexscreener_failed:%s:%v", address, err)
	}
	if len(response.Pairs) == 0 {
		return nil, ""
	}
	bestIndex := 0
	bestLiq := response.Pairs[0].Liquidity.USD
	for i, pair := range response.Pairs {
		if pair.Liquidity.USD > bestLiq {
			bestIndex = i
			bestLiq = pair.Liquidity.USD
		}
	}
	price := scanners.ParseFloat(response.Pairs[bestIndex].PriceUSD)
	if price <= 0 || math.IsNaN(price) || math.IsInf(price, 0) {
		return nil, ""
	}
	return &price, ""
}

// lastScannedBlock 从上一条成功的 s7 scanner_runs metadata 中恢复扫描检查点。
func (s *Scanner) lastScannedBlock() *int64 {
	var run model.ScannerRun
	err := s.db.Where("scanner = ? AND status = ?", "s7", "ok").Order("started_at desc").First(&run).Error
	if err != nil {
		return nil
	}
	var metadata map[string]any
	if json.Unmarshal([]byte(run.MetadataJSON), &metadata) != nil {
		return nil
	}
	value, ok := metadata["to_block"]
	if !ok {
		return nil
	}
	switch typed := value.(type) {
	case float64:
		block := int64(typed)
		return &block
	case string:
		block := HexToInt(typed)
		if block == 0 {
			return nil
		}
		return &block
	default:
		return nil
	}
}

// rpcCall 执行通用 JSON-RPC 请求，并把 result 反序列化到 target。
func (s *Scanner) rpcCall(ctx context.Context, method string, params any, target any) error {
	rpcURL := strings.TrimSpace(os.Getenv("S7_ETH_RPC_URL"))
	if rpcURL == "" {
		rpcURL = defaultRPCURL
	}
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "go-radar/0.1")

	response, err := s.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("RPC %s returned %s", method, response.Status)
	}

	var rpc rpcResponse
	if err := json.NewDecoder(response.Body).Decode(&rpc); err != nil {
		return err
	}
	if rpc.Error != nil {
		return fmt.Errorf("RPC %s error %d: %s", method, rpc.Error.Code, rpc.Error.Message)
	}
	if len(rpc.Result) == 0 || string(rpc.Result) == "null" {
		return nil
	}
	return json.Unmarshal(rpc.Result, target)
}

// buildReason 根据 token、数量、接收方和估值生成用户可读的信号原因。
func buildReason(symbol string, amount float64, recipientType string, recipientName string, usdValue *float64) string {
	if symbol == "" {
		symbol = "token"
	}
	route := "outbound transfer"
	switch recipientType {
	case "dex":
		route = "DEX route"
	case "cex":
		route = "CEX deposit route"
	case "pool":
		route = "liquidity pool route"
	}
	if recipientName != "" {
		route += " via " + recipientName
	}
	if usdValue != nil {
		return fmt.Sprintf("Vitalik transferred %.4g %s through %s, approx $%.0f.", amount, strings.ToUpper(symbol), route, *usdValue)
	}
	return fmt.Sprintf("Vitalik transferred %.4g %s through %s.", amount, strings.ToUpper(symbol), route)
}

// appendWarning 合并同一日志解析过程中的多个非致命告警。
func appendWarning(existing string, next string) string {
	if existing == "" {
		return next
	}
	if next == "" {
		return existing
	}
	return existing + ";" + next
}
