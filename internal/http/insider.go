package http

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go-radar/internal/insider"

	"github.com/gin-gonic/gin"
)

func (s *Server) insiderService() (*insider.Service, *insider.Store) {
	return s.insiderSvc, s.insiderStore
}

func (s *Server) selectedInsiderEngine(store *insider.Store) insider.Engine {
	engine := strings.ToLower(strings.TrimSpace(toString(defaultSettingValue("insider_monitor_engine"))))
	if overrides, err := s.settingsMap(); err == nil {
		if value := strings.ToLower(strings.TrimSpace(toString(overrides["insider_monitor_engine"]))); value != "" {
			engine = value
		}
	}
	cfg := insider.LoadEngineConfig()
	if engine == insider.EngineLegacy {
		return insider.NewLegacyEngine(store, cfg)
	}
	return insider.NewServiceEngine(store, cfg)
}

func (s *Server) apiInsiderWallets(c *gin.Context) {
	_, store := s.insiderService()
	wallets, err := store.ListWallets()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": wallets})
}

func (s *Server) apiInsiderCreateWallet(c *gin.Context) {
	var payload struct {
		Address string `json:"address"`
		Label   string `json:"label"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	service, _ := s.insiderService()
	wallet, err := service.CreateWallet(payload.Address, payload.Label)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"item": wallet})
}

func (s *Server) apiInsiderWallet(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	_, store := s.insiderService()
	wallet, err := store.GetWallet(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "wallet not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"item": wallet})
}

func (s *Server) apiInsiderUpdateWallet(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var payload struct {
		Label string `json:"label"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, store := s.insiderService()
	wallet, err := store.UpdateWallet(id, payload.Label)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"item": wallet})
}

func (s *Server) apiInsiderDeleteWallet(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	_, store := s.insiderService()
	if err := store.DeleteWallet(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) apiInsiderPortfolio(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	service, _ := s.insiderService()
	items, err := service.Portfolio(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (s *Server) apiInsiderTransactions(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	limit := parseLimit(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	_, store := s.insiderService()
	items, err := store.Transactions(id, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (s *Server) apiInsiderAnalytics(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	service, _ := s.insiderService()
	item, err := service.Analytics(id, c.DefaultQuery("period", "all"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"item": item})
}

func (s *Server) apiInsiderSync(c *gin.Context) {
	service, store := s.insiderService()
	engine := s.selectedInsiderEngine(store)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		_ = service.RunSync(ctx, engine)
	}()
	c.JSON(http.StatusOK, gin.H{"ok": true, "engine": engine.Name()})
}

func (s *Server) apiInsiderStatus(c *gin.Context) {
	service, store := s.insiderService()
	engine := s.selectedInsiderEngine(store)
	c.JSON(http.StatusOK, gin.H{"item": service.Status(engine.Name())})
}

func (s *Server) apiInsiderRules(c *gin.Context) {
	_, store := s.insiderService()
	rules, err := store.ListRules()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rules})
}

func (s *Server) apiInsiderSaveRule(c *gin.Context) {
	var payload struct {
		ID         int64   `json:"id"`
		WalletID   *int64  `json:"wallet_id"`
		RuleType   string  `json:"rule_type"`
		Threshold  float64 `json:"threshold"`
		ChannelIDs []int64 `json:"channel_ids"`
		Enabled    *bool   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	enabled := true
	if payload.Enabled != nil {
		enabled = *payload.Enabled
	}
	rule := &insider.AlertRule{
		ID:         payload.ID,
		WalletID:   payload.WalletID,
		RuleType:   payload.RuleType,
		Threshold:  payload.Threshold,
		ChannelIDs: payload.ChannelIDs,
		Enabled:    enabled,
	}
	_, store := s.insiderService()
	if err := store.SaveRule(rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"item": rule})
}

func (s *Server) apiInsiderHistory(c *gin.Context) {
	limit := parseLimit(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	_, store := s.insiderService()
	items, err := store.ListHistory(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (s *Server) apiInsiderChannels(c *gin.Context) {
	_, store := s.insiderService()
	items, err := store.ListChannels()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (s *Server) apiInsiderSaveChannel(c *gin.Context) {
	var payload insider.NotificationChannel
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, store := s.insiderService()
	if err := store.SaveChannel(&payload); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"item": payload})
}
