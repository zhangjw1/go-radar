package scanners

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

func NewHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxyRaw := strings.TrimSpace(os.Getenv("SELECTIVE_PROXY_URL")); proxyRaw != "" {
		if proxyURL, err := url.Parse(proxyRaw); err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	} else if !EnvBool("HTTP_TRUST_ENV", false) {
		transport.Proxy = nil
	}
	return &http.Client{Timeout: time.Duration(EnvFloat("GMGN_TIMEOUT_SECONDS", 6)) * time.Second, Transport: transport}
}

func GetJSON(ctx context.Context, client *http.Client, rawURL string, params url.Values, target any) error {
	if params != nil {
		rawURL += "?" + params.Encode()
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	request.Header.Set("Accept", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("GET %s returned %s", rawURL, response.Status)
	}
	return json.NewDecoder(response.Body).Decode(target)
}

func EnvBool(key string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	return raw == "1" || strings.EqualFold(raw, "true") || strings.EqualFold(raw, "yes") || strings.EqualFold(raw, "on")
}

func EnvInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func EnvFloat(key string, fallback float64) float64 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	return value
}

func ParseFloat(raw string) float64 {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0
	}
	return value
}
