package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go-radar/internal/config"
	"go-radar/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestHealthReturnsDatabaseStatus(t *testing.T) {
	router := testRouter(t)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/health", nil)

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status mismatch: got %d", recorder.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["database_ok"] != true {
		t.Fatalf("expected database_ok=true, got %#v", payload["database_ok"])
	}
}

func TestSignalsEndpointFiltersAndLimits(t *testing.T) {
	router := testRouter(t)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/signals?source=s5&limit=1", nil)

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status mismatch: got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Items []model.SignalEvent `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(payload.Items))
	}
	if payload.Items[0].Source != "s5" {
		t.Fatalf("expected s5 signal, got %q", payload.Items[0].Source)
	}
}

func TestWatchlistAPIUpsertsItem(t *testing.T) {
	router := testRouter(t)
	recorder := httptest.NewRecorder()
	body := bytes.NewBufferString(`{"chain":"eth","address":"0xabc","symbol":"aaa","name":"AAA","status":"watch","note":"track"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/watchlist", body)
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status mismatch: got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Item model.WatchlistItem `json:"item"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Item.Symbol != "AAA" || payload.Item.Note != "track" {
		t.Fatalf("unexpected watchlist item: %#v", payload.Item)
	}
}

func TestWatchlistPageRendersAddForm(t *testing.T) {
	router := testRouter(t)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/watchlist", nil)

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status mismatch: got %d body=%s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, want := range []string{`加入观察名单`, `method="post" action="/watchlist"`, `name="address"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("watchlist page missing %q", want)
		}
	}
}

func TestSettingsAPIStoresOverride(t *testing.T) {
	router := testRouter(t)
	recorder := httptest.NewRecorder()
	body := bytes.NewBufferString(`{"settings":{"enable_scanner_s5":false,"scan_interval_s5":60}}`)
	request := httptest.NewRequest(http.MethodPost, "/api/settings", body)
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status mismatch: got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Saved map[string]any `json:"saved"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Saved["enable_scanner_s5"] != false {
		t.Fatalf("expected enable_scanner_s5=false, got %#v", payload.Saved["enable_scanner_s5"])
	}
	if payload.Saved["scan_interval_s5"] != float64(60) {
		t.Fatalf("expected scan_interval_s5=60, got %#v", payload.Saved["scan_interval_s5"])
	}
}

func TestTelegramTestRequiresConfiguration(t *testing.T) {
	router := testRouter(t)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/telegram/test", nil)

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status mismatch: got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func testRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.ReleaseMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.TokenProfile{}, &model.TokenSnapshot{}, &model.SignalEvent{}, &model.ScannerRun{}, &model.WatchlistItem{}, &model.AppSetting{}); err != nil {
		t.Fatalf("migrate test schema: %v", err)
	}

	token := model.TokenProfile{Chain: "eth", Address: "0xabc", Symbol: "AAA", Name: "AAA"}
	if err := db.Create(&token).Error; err != nil {
		t.Fatalf("seed token: %v", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := db.Create(&model.SignalEvent{
		TokenID:    &token.ID,
		Source:     "s5",
		Chain:      "eth",
		Address:    "0xabc",
		Symbol:     "AAA",
		SignalType: "momentum",
		Priority:   "medium",
		Score:      42,
		Reason:     "test",
		DedupeKey:  "s5|eth|0xabc|momentum|bucket",
		CreatedAt:  now,
	}).Error; err != nil {
		t.Fatalf("seed signal: %v", err)
	}
	if err := db.Create(&model.ScannerRun{
		Scanner:       "s5",
		StartedAt:     now,
		Status:        "ok",
		SignalCount:   1,
		SnapshotCount: 1,
	}).Error; err != nil {
		t.Fatalf("seed run: %v", err)
	}

	return NewRouter(&config.Settings{
		AppName:         "test radar",
		DatabasePath:    ":memory:",
		Port:            "8080",
		EnableScheduler: false,
	}, db)
}
