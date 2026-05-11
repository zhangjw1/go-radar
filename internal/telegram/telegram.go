package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Settings 保存 Telegram 推送所需配置。
type Settings struct {
	BotToken string // BotToken 是 Telegram Bot API token。
	ChatID   string // ChatID 是接收消息的群组或用户 ID。
	ProxyURL string // ProxyURL 是显式代理地址，优先级高于环境变量代理。
	TrustEnv bool   // TrustEnv 控制是否允许 http.Client 使用系统环境变量代理。
}

// Notifier 封装 Telegram sendMessage 调用。
type Notifier struct {
	settings Settings     // settings 是构建 Notifier 时固定下来的推送配置。
	client   *http.Client // client 是带超时和代理设置的 HTTP 客户端。
}

// New 创建 Telegram 推送器，并根据配置设置代理行为。
func New(settings Settings) (*Notifier, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if settings.ProxyURL != "" {
		proxyURL, err := url.Parse(settings.ProxyURL)
		if err != nil {
			return nil, fmt.Errorf("parse telegram proxy URL: %w", err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	} else if !settings.TrustEnv {
		transport.Proxy = nil
	}

	return &Notifier{
		settings: settings,
		client:   &http.Client{Timeout: 10 * time.Second, Transport: transport},
	}, nil
}

// Enabled 判断当前配置是否足够发送 Telegram 消息。
func (n *Notifier) Enabled() bool {
	return strings.TrimSpace(n.settings.BotToken) != "" && strings.TrimSpace(n.settings.ChatID) != ""
}

// SendText 发送一条 HTML parse mode 的 Telegram 文本消息。
func (n *Notifier) SendText(ctx context.Context, text string) error {
	if !n.Enabled() {
		return ErrDisabled
	}
	payload := map[string]any{
		"chat_id":    n.settings.ChatID,
		"text":       text,
		"parse_mode": "HTML",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", n.settings.BotToken)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := n.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram sendMessage returned %s", response.Status)
	}
	return nil
}

// ErrDisabled 表示 Telegram 缺少 bot token 或 chat id，当前不可发送。
var ErrDisabled = fmt.Errorf("telegram is not configured")
