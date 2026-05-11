package s7

import (
	"encoding/hex"
	"math"
	"math/big"
	"strings"
	"unicode"
)

const VitalikAddress = "0xd8da6bf26964af9d7eed9e03e53415d37aa96045"
const TransferTopic = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
const SignalType = "vitalik_sell"
const InitialLookbackBlocks = 40
const CheckpointOverlapBlocks = 6

// VitalikPadded 是 Transfer topic 中 indexed address 的 32 字节形式。
var VitalikPadded = "0x" + strings.Repeat("0", 24) + strings.TrimPrefix(VitalikAddress, "0x")

// KnownDEXRouters 是 S7 用来识别 DEX 路由转账目的地的地址表。
var KnownDEXRouters = map[string]string{
	"0x7a250d5630b4cf539739df2c5dacb4c659f2488d": "Uniswap V2 Router",
	"0xe592427a0aece92de3edee1f18e0157c05861564": "Uniswap V3 Router",
	"0x68b3465833fb72a70ecdf485e0e4c7bd8665fc45": "Uniswap V3 Router2",
	"0x3fc91a3afd70395cd496c647d5a6cc9d4b2b7fad": "Uniswap Universal Router",
	"0xef1c6e67703c7bd7107eed8303fbe6ec2554bf6b": "Uniswap Universal Router (old)",
	"0x1111111254eeb25477b68fb85ed929f73a960582": "1inch V5",
	"0x111111125421ca6dc452d289314280a0f8842a65": "1inch V6",
	"0xd9e1ce17f2641f24ae83637ab66a2cca9c378b9f": "SushiSwap Router",
	"0x9008d19f58aabd9ed0d60971565aa8510560ab41": "CoW Settlement",
	"0xdef1c0ded9bec7f1a1670819833240f027b25eff": "0x Exchange Proxy",
	"0x99a58482bd75cbab83b27ec03ca68ff489b5788f": "Curve Router",
}

// KnownCEX 是 S7 用来识别中心化交易所充值地址的地址表。
var KnownCEX = map[string]string{
	"0x28c6c06298d514db089934071355e5743bf21d60": "Binance Hot Wallet",
	"0x21a31ee1afc51d94c2efccaa2092ad1028285549": "Binance Hot Wallet 2",
	"0xdfd5293d8e347dfe59e90efd55b2956a1343963d": "Binance Hot Wallet 3",
	"0x56eddb7aa87536c09ccc2793473599fd21a8b17f": "Binance Hot Wallet 4",
	"0x71660c4005ba85c37ccec55d0c4493e66fe775d3": "Coinbase",
	"0xa9d1e08c7793af67e9d92fe308d5697fb81d3e43": "Coinbase 10",
	"0x503828976d22510aad0201ac7ec88293211d23da": "Coinbase 2",
	"0x2faf487a4414fe77e2327f0bf4ae2a264a776ad2": "FTX",
	"0x267be1c1d684f78cb4f6a176c4911b741e4ffdc0": "Kraken",
	"0xae2d4617c862309a3d75a0ffb358c7a5009c673f": "Kraken 10",
}

// ShortenAddr 把地址缩短为页面/信号文案中可读的形式。
func ShortenAddr(address string) string {
	if len(address) < 12 {
		return address
	}
	return address[:6] + "..." + address[len(address)-4:]
}

// TopicToAddress 从 32 字节 topic 中还原 EVM 地址。
func TopicToAddress(topic string) string {
	if len(topic) < 40 {
		return ""
	}
	return "0x" + strings.ToLower(topic[len(topic)-40:])
}

// HexToInt 将 JSON-RPC 十六进制数字转为 int64。
func HexToInt(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	parsed, ok := new(big.Int).SetString(strings.TrimPrefix(value, "0x"), 16)
	if !ok {
		return 0
	}
	return parsed.Int64()
}

// DecodeTransferValue 按 decimals 把 ERC20 Transfer data 解码为真实 token 数量。
func DecodeTransferValue(dataHex string, decimals int) float64 {
	parsed, ok := new(big.Int).SetString(strings.TrimPrefix(dataHex, "0x"), 16)
	if !ok {
		return 0
	}
	if decimals < 0 || decimals > 36 {
		decimals = 18
	}
	denom := new(big.Float).SetFloat64(math.Pow10(decimals))
	value := new(big.Float).Quo(new(big.Float).SetInt(parsed), denom)
	out, _ := value.Float64()
	return out
}

// DecodeRPCString 解析 ERC20 symbol/name 这类 eth_call 返回的 ABI 字符串。
func DecodeRPCString(result string) string {
	if result == "" || result == "0x" {
		return ""
	}
	body := strings.TrimPrefix(result, "0x")
	if len(body) == 64 {
		return decodeHexString(body)
	}
	if len(body) >= 128 {
		offset, ok := new(big.Int).SetString(body[:64], 16)
		if !ok {
			return ""
		}
		offsetChars := int(offset.Int64()) * 2
		if offsetChars+64 > len(body) {
			return ""
		}
		length, ok := new(big.Int).SetString(body[offsetChars:offsetChars+64], 16)
		if !ok {
			return ""
		}
		start := offsetChars + 64
		end := start + int(length.Int64())*2
		if end > len(body) {
			return ""
		}
		return decodeHexString(body[start:end])
	}
	return ""
}

// ClassifyRecipient 将接收地址归类为 dex、cex 或 unknown。
func ClassifyRecipient(address string) (string, string) {
	lowered := strings.ToLower(address)
	if name, ok := KnownDEXRouters[lowered]; ok {
		return "dex", name
	}
	if name, ok := KnownCEX[lowered]; ok {
		return "cex", name
	}
	return "unknown", ""
}

// ComputeBlockWindow 根据上次扫描检查点计算本轮区块窗口，并保留少量重叠防漏扫。
func ComputeBlockWindow(latestBlock int64, lastScannedBlock *int64, initialLookback int64, overlapBlocks int64) (int64, int64) {
	var fromBlock int64
	if lastScannedBlock == nil {
		fromBlock = latestBlock - initialLookback + 1
	} else {
		fromBlock = *lastScannedBlock - overlapBlocks + 1
	}
	if fromBlock < 0 {
		fromBlock = 0
	}
	if fromBlock > latestBlock {
		fromBlock = latestBlock
	}
	return fromBlock, latestBlock
}

// ResolvePriority 根据接收方类型和美元价值判断 S7 信号优先级。
func ResolvePriority(recipientType string, usdValue *float64) string {
	if recipientType == "dex" || recipientType == "pool" {
		return "high"
	}
	if usdValue != nil && *usdValue >= 1_000_000 {
		return "high"
	}
	return "medium"
}

// ScoreSellSignal 根据接收方类型和美元价值计算 S7 信号分数。
func ScoreSellSignal(recipientType string, usdValue *float64) float64 {
	base := map[string]float64{"dex": 92, "pool": 90, "cex": 82}[recipientType]
	if base == 0 {
		base = 75
	}
	if usdValue == nil {
		return base
	}
	switch {
	case *usdValue >= 5_000_000:
		base += 6
	case *usdValue >= 1_000_000:
		base += 4
	case *usdValue >= 100_000:
		base += 2
	}
	if base > 99 {
		base = 99
	}
	return math.Round(base*100) / 100
}

// SlugTag 将接收方名称转换为适合 tags_json 的小写下划线标签。
func SlugTag(text string) string {
	var builder strings.Builder
	lastUnderscore := false
	for _, ch := range text {
		if unicode.IsLetter(ch) || unicode.IsDigit(ch) {
			builder.WriteRune(unicode.ToLower(ch))
			lastUnderscore = false
		} else if !lastUnderscore {
			builder.WriteRune('_')
			lastUnderscore = true
		}
	}
	out := strings.Trim(builder.String(), "_")
	if out == "" {
		return "unknown"
	}
	return out
}

// decodeHexString 将 ABI 字符串 payload 的十六进制内容转为普通字符串。
func decodeHexString(raw string) string {
	bytes, err := hex.DecodeString(raw)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(strings.TrimRight(string(bytes), "\x00"))
}
