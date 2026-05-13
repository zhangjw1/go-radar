package insider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

type RPCClient struct {
	endpoint string
	client   *http.Client
	nextID   int64
}

func NewRPCClient(endpoint string) *RPCClient {
	return &RPCClient{
		endpoint: strings.TrimSpace(endpoint),
		client:   &http.Client{Timeout: 30 * time.Second},
		nextID:   1,
	}
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id"`
	Method  string `json:"method"`
	Params  []any  `json:"params,omitempty"`
}

type rpcResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (r *RPCClient) call(ctx context.Context, method string, params []any, out any) error {
	if r.endpoint == "" {
		return fmt.Errorf("solana rpc url is not configured")
	}
	body, err := json.Marshal(rpcRequest{JSONRPC: "2.0", ID: r.nextID, Method: method, Params: params})
	if err != nil {
		return err
	}
	r.nextID++
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("rpc status=%d body=%s", resp.StatusCode, string(raw))
	}
	var decoded rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return err
	}
	if decoded.Error != nil {
		return fmt.Errorf("rpc %s failed: %s", method, decoded.Error.Message)
	}
	return json.Unmarshal(decoded.Result, out)
}

type tokenAccountRPCResult struct {
	Value []struct {
		Account struct {
			Data struct {
				Parsed struct {
					Info struct {
						Mint        string `json:"mint"`
						TokenAmount struct {
							Amount         string  `json:"amount"`
							Decimals       int     `json:"decimals"`
							UIAmount       float64 `json:"uiAmount"`
							UIAmountString string  `json:"uiAmountString"`
						} `json:"tokenAmount"`
					} `json:"info"`
				} `json:"parsed"`
			} `json:"data"`
		} `json:"account"`
	} `json:"value"`
}

func (r *RPCClient) TokenAccounts(ctx context.Context, owner string) ([]TokenAccount, error) {
	var result tokenAccountRPCResult
	err := r.call(ctx, "getTokenAccountsByOwner", []any{
		owner,
		map[string]any{"programId": "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"},
		map[string]any{"encoding": "jsonParsed"},
	}, &result)
	if err != nil {
		return nil, err
	}
	accounts := make([]TokenAccount, 0, len(result.Value))
	for _, item := range result.Value {
		info := item.Account.Data.Parsed.Info
		if info.Mint == "" || info.TokenAmount.UIAmount <= 0 {
			continue
		}
		accounts = append(accounts, TokenAccount{
			MintAddress: info.Mint,
			Balance:     info.TokenAmount.UIAmount,
			Decimals:    info.TokenAmount.Decimals,
		})
	}
	return accounts, nil
}

type balanceRPCResult struct {
	Value uint64 `json:"value"`
}

func (r *RPCClient) SOLBalance(ctx context.Context, owner string) (TokenAccount, bool, error) {
	var result balanceRPCResult
	if err := r.call(ctx, "getBalance", []any{owner, map[string]any{"commitment": "finalized"}}, &result); err != nil {
		return TokenAccount{}, false, err
	}
	balance := float64(result.Value) / math.Pow10(9)
	if balance <= 0 {
		return TokenAccount{}, false, nil
	}
	return TokenAccount{MintAddress: NativeSOLMint, TokenName: "SOL", Balance: balance, Decimals: 9}, true, nil
}

type HeliusClient struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

func NewHeliusClient(apiKey, baseURL string) *HeliusClient {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://api.helius.xyz"
	}
	return &HeliusClient{
		apiKey:  strings.TrimSpace(apiKey),
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

type heliusMetadataItem struct {
	Account        string `json:"account"`
	LegacyMetadata *struct {
		Symbol string `json:"symbol"`
		Name   string `json:"name"`
	} `json:"legacyMetadata"`
	OnChainMetadata *struct {
		Metadata *struct {
			Data *struct {
				Symbol string `json:"symbol"`
				Name   string `json:"name"`
			} `json:"data"`
		} `json:"metadata"`
	} `json:"onChainMetadata"`
}

func (h *HeliusClient) TokenNames(ctx context.Context, mints []string) map[string]string {
	names := map[string]string{}
	if h.apiKey == "" || len(mints) == 0 {
		return names
	}
	for i := 0; i < len(mints); i += 100 {
		end := i + 100
		if end > len(mints) {
			end = len(mints)
		}
		payload, _ := json.Marshal(map[string]any{"mintAccounts": mints[i:end]})
		url := fmt.Sprintf("%s/v0/token-metadata?api-key=%s", h.baseURL, h.apiKey)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := h.client.Do(req)
		if err != nil {
			continue
		}
		var items []heliusMetadataItem
		if resp.StatusCode == http.StatusOK {
			_ = json.NewDecoder(resp.Body).Decode(&items)
		}
		resp.Body.Close()
		for _, item := range items {
			name := tokenNameFromMetadata(item)
			if name != "" {
				names[item.Account] = name
			}
		}
	}
	return names
}

func tokenNameFromMetadata(item heliusMetadataItem) string {
	if item.LegacyMetadata != nil {
		if strings.TrimSpace(item.LegacyMetadata.Symbol) != "" {
			return strings.TrimSpace(item.LegacyMetadata.Symbol)
		}
		if strings.TrimSpace(item.LegacyMetadata.Name) != "" {
			return strings.TrimSpace(item.LegacyMetadata.Name)
		}
	}
	if item.OnChainMetadata != nil && item.OnChainMetadata.Metadata != nil && item.OnChainMetadata.Metadata.Data != nil {
		if strings.TrimSpace(item.OnChainMetadata.Metadata.Data.Symbol) != "" {
			return strings.TrimSpace(item.OnChainMetadata.Metadata.Data.Symbol)
		}
		return strings.TrimSpace(item.OnChainMetadata.Metadata.Data.Name)
	}
	return ""
}

type ParsedTransaction struct {
	Signature       string           `json:"signature"`
	Timestamp       int64            `json:"timestamp"`
	TokenTransfers  []TokenTransfer  `json:"tokenTransfers"`
	NativeTransfers []NativeTransfer `json:"nativeTransfers"`
}

type TokenTransfer struct {
	FromUserAccount string  `json:"fromUserAccount"`
	ToUserAccount   string  `json:"toUserAccount"`
	Mint            string  `json:"mint"`
	TokenAmount     float64 `json:"tokenAmount"`
}

type NativeTransfer struct {
	FromUserAccount string `json:"fromUserAccount"`
	ToUserAccount   string `json:"toUserAccount"`
	Amount          int64  `json:"amount"`
}

func (h *HeliusClient) Transactions(ctx context.Context, address, afterSignature string) ([]ParsedTransaction, error) {
	if h.apiKey == "" {
		return nil, nil
	}
	var all []ParsedTransaction
	before := ""
	for {
		url := fmt.Sprintf("%s/v0/addresses/%s/transactions?api-key=%s&limit=100", h.baseURL, address, h.apiKey)
		if before != "" {
			url += "&before=" + before
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return all, err
		}
		resp, err := h.client.Do(req)
		if err != nil {
			return all, err
		}
		if resp.StatusCode != http.StatusOK {
			raw, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return all, fmt.Errorf("helius status=%d body=%s", resp.StatusCode, string(raw))
		}
		var page []ParsedTransaction
		err = json.NewDecoder(resp.Body).Decode(&page)
		resp.Body.Close()
		if err != nil {
			return all, err
		}
		if len(page) == 0 {
			break
		}
		reachedKnown := false
		for _, tx := range page {
			if afterSignature != "" && tx.Signature == afterSignature {
				reachedKnown = true
				break
			}
			all = append(all, tx)
		}
		if reachedKnown || len(page) < 100 {
			break
		}
		before = page[len(page)-1].Signature
		time.Sleep(200 * time.Millisecond)
	}
	return all, nil
}

func ParseTransactions(wallet Wallet, raw []ParsedTransaction) []Transaction {
	txs := make([]Transaction, 0)
	for _, item := range raw {
		for _, transfer := range item.TokenTransfers {
			if transfer.Mint == "" || transfer.TokenAmount == 0 {
				continue
			}
			tx := Transaction{
				WalletID:    wallet.ID,
				Signature:   item.Signature,
				MintAddress: transfer.Mint,
				Amount:      transfer.TokenAmount,
				BlockTime:   time.Unix(item.Timestamp, 0),
			}
			if transfer.ToUserAccount == wallet.Address {
				tx.TxType = "buy"
			} else if transfer.FromUserAccount == wallet.Address {
				tx.TxType = "sell"
			} else {
				continue
			}
			for _, nt := range item.NativeTransfers {
				if nt.FromUserAccount == wallet.Address || nt.ToUserAccount == wallet.Address {
					tx.SolAmount = float64(nt.Amount) / 1_000_000_000
					break
				}
			}
			txs = append(txs, tx)
		}
	}
	return txs
}

type JupiterClient struct {
	client *http.Client
}

func NewJupiterClient() *JupiterClient {
	return &JupiterClient{client: &http.Client{Timeout: 15 * time.Second}}
}

type jupiterPriceItem struct {
	USDPrice float64 `json:"usdPrice"`
	Price    float64 `json:"price"`
}

func (j *JupiterClient) Prices(ctx context.Context, mints []string) (map[string]float64, error) {
	prices := map[string]float64{}
	if len(mints) == 0 {
		return prices, nil
	}
	var lastErr error
	for i := 0; i < len(mints); i += 10 {
		end := i + 10
		if end > len(mints) {
			end = len(mints)
		}
		url := fmt.Sprintf("https://lite-api.jup.ag/price/v3?ids=%s", strings.Join(mints[i:end], ","))
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			lastErr = err
			continue
		}
		resp, err := j.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("jupiter status=%d", resp.StatusCode)
			resp.Body.Close()
			continue
		}
		var data map[string]jupiterPriceItem
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			lastErr = err
			resp.Body.Close()
			continue
		}
		resp.Body.Close()
		for mint, item := range data {
			price := item.USDPrice
			if price == 0 {
				price = item.Price
			}
			if price > 0 {
				prices[mint] = price
			}
		}
		if end < len(mints) {
			time.Sleep(100 * time.Millisecond)
		}
	}
	if len(prices) == 0 && lastErr != nil {
		return prices, lastErr
	}
	return prices, nil
}
