package http

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"go-radar/internal/config"
	"go-radar/internal/insider"
	"go-radar/internal/model"
	"go-radar/internal/scheduler"
	radartelegram "go-radar/internal/telegram"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

//go:embed templates/*.html
var templateFiles embed.FS

const defaultLimit = 100
const maxLimit = 500

// Server 持有 HTTP 层处理请求需要的共享依赖。
type Server struct {
	settings     *config.Settings     // settings 是启动配置，用于页面展示和健康检查。
	db           *gorm.DB             // db 是查询页面和 API 数据的数据库连接。
	scheduler    *scheduler.Scheduler // scheduler 用于页面展示任务状态；为空时表示未接入调度器。
	templates    *template.Template   // templates 是编译后的嵌入式 HTML 模板集合。
	insiderStore *insider.Store
	insiderSvc   *insider.Service
}

// SidebarGroup 是侧边栏中的一组导航。
type SidebarGroup struct {
	Title string        // Title 是分组标题。
	Items []SidebarItem // Items 是分组下的导航项。
}

// SidebarItem 是侧边栏中的一个导航链接。
type SidebarItem struct {
	Label    string // Label 是链接展示名。
	Href     string // Href 是跳转地址。
	Key      string // Key 是用于判断激活状态的稳定标识。
	Subtitle string // Subtitle 是导航项的辅助说明。
}

// PageData 是所有页面模板共享的基础数据。
type PageData struct {
	AppName       string         // AppName 是应用名。
	ActiveNav     string         // ActiveNav 是当前激活导航 key。
	SidebarGroups []SidebarGroup // SidebarGroups 是页面侧边栏分组。
}

// SignalFilters 表示 signals/pushes 页面和 API 支持的筛选条件。
type SignalFilters struct {
	Source        string // Source 按扫描器来源筛选。
	Chain         string // Chain 按链或市场筛选。
	Symbol        string // Symbol 按展示符号或地址模糊筛选。
	SignalType    string // SignalType 按信号类型筛选。
	Priority      string // Priority 按优先级筛选。
	TimeRange     string // TimeRange 按最近时间窗口筛选。
	WatchlistOnly bool   // WatchlistOnly 表示只看观察名单 token。
}

// FilterOptions 是页面下拉框可选项集合。
type FilterOptions struct {
	Sources     []string `json:"sources"`     // Sources 是来源下拉选项。
	Chains      []string `json:"chains"`      // Chains 是链下拉选项。
	SignalTypes []string `json:"signalTypes"` // SignalTypes 是信号类型下拉选项。
	Priorities  []string `json:"priorities"`  // Priorities 是优先级下拉选项。
	TimeRanges  []string `json:"timeRanges"`  // TimeRanges 是时间范围下拉选项。
}

// DashboardData 是监控总览页的数据模型。
type DashboardData struct {
	Page                 PageData            // Page 是页面公共数据。
	HighSignals          []model.SignalEvent // HighSignals 是最近高优先级信号。
	ResonanceSignals     []model.SignalEvent // ResonanceSignals 是最近跨来源共振信号。
	LatestPushes         []model.SignalEvent // LatestPushes 是最近已推送信号。
	RecentRuns           []model.ScannerRun  // RecentRuns 是最近扫描任务运行记录。
	JobErrors            []model.ScannerRun  // JobErrors 是最近失败任务。
	JobWarnings          []JobWarning        // JobWarnings 是任务 metadata 中的非致命告警。
	SourceCounts         map[string]int64    // SourceCounts 是各来源最近信号数量。
	WatchlistHitCount    int64               // WatchlistHitCount 是观察名单命中的信号数量。
	ActiveWatchlistCount int64               // ActiveWatchlistCount 是 active 状态观察名单数量。
}

// SignalsData 是信号列表页的数据模型。
type SignalsData struct {
	Page          PageData            // Page 是页面公共数据。
	Signals       []model.SignalEvent // Signals 是当前筛选后的信号列表。
	Filters       SignalFilters       // Filters 是当前筛选条件。
	FilterOptions FilterOptions       // FilterOptions 是筛选下拉框选项。
}

// JobsData 是任务状态页的数据模型。
type JobsData struct {
	Page PageData // Page 是页面公共数据。
	Rows []JobRow // Rows 是每个扫描器的任务状态行。
}

// PushesData 是推送记录页的数据模型。
type PushesData struct {
	Page          PageData            // Page 是页面公共数据。
	Pushes        []model.SignalEvent // Pushes 是当前筛选后的已推送信号。
	Filters       SignalFilters       // Filters 是当前筛选条件。
	FilterOptions FilterOptions       // FilterOptions 是筛选下拉框选项。
}

// RadarData 是单个雷达源详情页的数据模型。
type RadarData struct {
	Page         PageData            // Page 是页面公共数据。
	Source       string              // Source 是当前雷达源编号。
	Meta         RadarMeta           // Meta 是当前雷达源的展示说明。
	RecentHigh   []model.SignalEvent // RecentHigh 是该来源最近高优先级信号。
	Signals      []model.SignalEvent // Signals 是该来源最近信号。
	Runs         []model.ScannerRun  // Runs 是该来源最近任务记录。
	TypeCounts   []TypeCount         // TypeCounts 是该来源按 signal_type 统计的数量。
	PushCount24H int64               // PushCount24H 是 24 小时内推送数量。
	LatestPush   *model.SignalEvent  // LatestPush 是该来源最近一次推送。
	LatestErrors []model.ScannerRun  // LatestErrors 是该来源最近失败记录。
}

// TokenData 是 token 详情页的数据模型。
type TokenData struct {
	Page        PageData              // Page 是页面公共数据。
	Token       model.TokenProfile    // Token 是 token 基础资料。
	Snapshots   []model.TokenSnapshot // Snapshots 是最近行情快照。
	Signals     []model.SignalEvent   // Signals 是最近相关信号。
	WatchItem   *model.WatchlistItem  // WatchItem 是观察名单记录；为空表示未关注。
	Tags        []string              // Tags 是解析后的叙事标签。
	SocialLinks map[string]string     // SocialLinks 是解析后的社交链接。
}

// WatchlistData 是观察名单页的数据模型。
type WatchlistData struct {
	Page  PageData              // Page 是页面公共数据。
	Items []model.WatchlistItem // Items 是观察名单条目。
}

// SettingsData 是运行设置页的数据模型。
type SettingsData struct {
	Page        PageData     // Page 是页面公共数据。
	Rows        []SettingRow // Rows 是可编辑设置项。
	SaveStatus  string       // SaveStatus 是保存操作状态。
	SaveMessage string       // SaveMessage 是保存操作提示。
	TestStatus  string       // TestStatus 是 Telegram 测试状态。
	TestMessage string       // TestMessage 是 Telegram 测试提示。
}

// SettingRow 是运行设置页中的单个配置项。
type SettingRow struct {
	Key      string // Key 是设置项名称。
	Value    string // Value 是当前有效值。
	Default  string // Default 是默认值。
	Override string // Override 是来自 settings 表的覆盖值。
	IsBool   bool   // IsBool 表示该设置是否为布尔开关。
}

// JobRow 是任务状态页中的单个扫描器状态。
type JobRow struct {
	Scanner         string            // Scanner 是扫描器编号。
	Label           string            // Label 是展示名称。
	Summary         string            // Summary 是扫描器摘要。
	Focus           string            // Focus 是扫描器关注重点。
	Implemented     bool              // Implemented 表示 Go 版是否已实现该扫描器。
	IntervalSeconds int               // IntervalSeconds 是当前扫描间隔。
	LastRun         *model.ScannerRun // LastRun 是最近一次运行记录。
}

// JobWarning 是从 scanner_runs.metadata_json 中解析出的任务告警。
type JobWarning struct {
	Scanner   string   // Scanner 是扫描器编号。
	StartedAt string   // StartedAt 是告警所属任务开始时间。
	Warnings  []string // Warnings 是非致命告警列表。
}

// RadarMeta 是单个雷达源的页面描述。
type RadarMeta struct {
	Label   string // Label 是雷达源展示名。
	Summary string // Summary 是雷达源摘要。
	Focus   string // Focus 是雷达源关注点。
}

// TypeCount 是按信号类型聚合后的计数。
type TypeCount struct {
	Type  string // Type 是 signal_type。
	Count int64  // Count 是该类型数量。
}

// NewRouter 创建只包含 HTTP 路由的 Gin engine。
func NewRouter(settings *config.Settings, db *gorm.DB) *gin.Engine {
	return NewRouterWithScheduler(settings, db, nil)
}

// NewRouterWithScheduler 创建 Gin engine，并注册页面、API 和调度器状态路由。
func NewRouterWithScheduler(settings *config.Settings, db *gorm.DB, goScheduler *scheduler.Scheduler) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	server := &Server{
		settings:     settings,
		db:           db,
		scheduler:    goScheduler,
		insiderStore: insider.NewStore(db),
		templates: template.Must(template.New("").Funcs(template.FuncMap{
			"sourceLabel":     sourceLabel,
			"chainLabel":      chainLabel,
			"signalTypeLabel": signalTypeLabel,
			"priorityClass":   priorityClass,
			"priorityLabel":   priorityLabel,
			"statusLabel":     statusLabel,
			"warningCount":    warningCount,
			"firstWarning":    firstWarning,
			"formatScore":     formatScore,
			"formatTime":      formatTime,
			"sumCounts":       sumCounts,
			"sourceOrder":     sourceOrder,
		}).ParseFS(templateFiles, "templates/*.html")),
	}
	server.insiderSvc = insider.NewService(server.insiderStore)

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	router.GET("/health", server.health)
	router.GET("/api/signals", server.apiSignals)
	router.GET("/api/pushes", server.apiPushes)
	router.GET("/api/jobs", server.apiJobs)
	router.GET("/api/watchlist", server.apiWatchlist)
	router.POST("/api/watchlist", server.apiWatchlistUpsert)
	router.GET("/api/settings", server.apiSettings)
	router.POST("/api/settings", server.apiSettingsUpdate)
	router.POST("/api/telegram/test", server.apiTelegramTest)
	router.GET("/api/tokens/:chain/:address", server.apiToken)
	router.GET("/api/insider/wallets", server.apiInsiderWallets)
	router.POST("/api/insider/wallets", server.apiInsiderCreateWallet)
	router.GET("/api/insider/wallets/:id", server.apiInsiderWallet)
	router.PUT("/api/insider/wallets/:id", server.apiInsiderUpdateWallet)
	router.DELETE("/api/insider/wallets/:id", server.apiInsiderDeleteWallet)
	router.GET("/api/insider/wallets/:id/portfolio", server.apiInsiderPortfolio)
	router.GET("/api/insider/wallets/:id/transactions", server.apiInsiderTransactions)
	router.GET("/api/insider/wallets/:id/analytics", server.apiInsiderAnalytics)
	router.POST("/api/insider/sync/trigger", server.apiInsiderSync)
	router.GET("/api/insider/sync/status", server.apiInsiderStatus)
	router.GET("/api/insider/alerts/rules", server.apiInsiderRules)
	router.POST("/api/insider/alerts/rules", server.apiInsiderSaveRule)
	router.PUT("/api/insider/alerts/rules/:id", server.apiInsiderSaveRule)
	router.GET("/api/insider/alerts/history", server.apiInsiderHistory)
	router.GET("/api/insider/alerts/channels", server.apiInsiderChannels)
	router.POST("/api/insider/alerts/channels", server.apiInsiderSaveChannel)
	router.PUT("/api/insider/alerts/channels/:id", server.apiInsiderSaveChannel)
	router.GET("/radar-api/signals", server.apiSignals)
	router.GET("/radar-api/pushes", server.apiPushes)
	router.GET("/radar-api/jobs", server.apiJobs)
	router.GET("/radar-api/watchlist", server.apiWatchlist)
	router.POST("/radar-api/watchlist", server.apiWatchlistUpsert)
	router.GET("/radar-api/settings", server.apiSettings)
	router.POST("/radar-api/settings", server.apiSettingsUpdate)
	router.POST("/radar-api/telegram/test", server.apiTelegramTest)
	router.GET("/radar-api/tokens/:chain/:address", server.apiToken)
	router.GET("/radar-api/insider/wallets", server.apiInsiderWallets)
	router.POST("/radar-api/insider/wallets", server.apiInsiderCreateWallet)
	router.GET("/radar-api/insider/wallets/:id", server.apiInsiderWallet)
	router.PUT("/radar-api/insider/wallets/:id", server.apiInsiderUpdateWallet)
	router.DELETE("/radar-api/insider/wallets/:id", server.apiInsiderDeleteWallet)
	router.GET("/radar-api/insider/wallets/:id/portfolio", server.apiInsiderPortfolio)
	router.GET("/radar-api/insider/wallets/:id/transactions", server.apiInsiderTransactions)
	router.GET("/radar-api/insider/wallets/:id/analytics", server.apiInsiderAnalytics)
	router.POST("/radar-api/insider/sync/trigger", server.apiInsiderSync)
	router.GET("/radar-api/insider/sync/status", server.apiInsiderStatus)
	router.GET("/radar-api/insider/alerts/rules", server.apiInsiderRules)
	router.POST("/radar-api/insider/alerts/rules", server.apiInsiderSaveRule)
	router.PUT("/radar-api/insider/alerts/rules/:id", server.apiInsiderSaveRule)
	router.GET("/radar-api/insider/alerts/history", server.apiInsiderHistory)
	router.GET("/radar-api/insider/alerts/channels", server.apiInsiderChannels)
	router.POST("/radar-api/insider/alerts/channels", server.apiInsiderSaveChannel)
	router.PUT("/radar-api/insider/alerts/channels/:id", server.apiInsiderSaveChannel)
	router.GET("/dashboard", server.dashboard)
	router.GET("/signals", server.signalsPage)
	router.GET("/pushes", server.pushesPage)
	router.GET("/radar/:source", server.radarPage)
	router.GET("/token/:chain/:address", server.tokenPage)
	router.GET("/watchlist", server.watchlistPage)
	router.POST("/watchlist", server.watchlistSubmit)
	router.GET("/jobs", server.jobsPage)
	router.GET("/settings", server.settingsPage)
	router.POST("/settings", server.settingsSubmit)
	router.POST("/telegram/test", server.telegramTestPage)
	router.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/dashboard")
	})
	return router
}

func (s *Server) health(c *gin.Context) {
	sqlDB, err := s.db.DB()
	dbOK := err == nil && sqlDB.Ping() == nil
	response := gin.H{
		"status":            "ok",
		"service":           "go-radar",
		"app_name":          s.settings.AppName,
		"time":              time.Now().UTC().Format(time.RFC3339),
		"database_ok":       dbOK,
		"database_path":     s.settings.DatabasePath,
		"scheduler_ok":      s.scheduler == nil || !s.scheduler.Enabled() || dbOK,
		"scheduler_enabled": s.scheduler != nil && s.scheduler.Enabled(),
	}
	if s.scheduler != nil {
		jobs := []gin.H{}
		for _, spec := range s.scheduler.Specs() {
			jobs = append(jobs, gin.H{
				"id":               spec.Name,
				"interval_seconds": spec.IntervalSeconds,
			})
		}
		response["jobs"] = jobs
	}
	if !dbOK {
		response["status"] = "degraded"
	}
	c.JSON(http.StatusOK, response)
}

func (s *Server) apiSignals(c *gin.Context) {
	query := applySignalFilters(s.db.Model(&model.SignalEvent{}), filtersFromQuery(c))
	page, pageSize := parsePagination(c)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	query = query.Order("created_at desc").Limit(pageSize).Offset((page - 1) * pageSize)

	var signals []model.SignalEvent
	if err := query.Find(&signals).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"items":    signals,
		"total":    total,
		"current":  page,
		"pageSize": pageSize,
		"filters":  s.filterOptions(),
	})
}

func (s *Server) apiPushes(c *gin.Context) {
	query := applySignalFilters(s.db.Model(&model.SignalEvent{}), filtersFromQuery(c))
	query = query.Where("pushed_at IS NOT NULL").Order("pushed_at desc").Limit(parseLimit(c.Query("limit")))

	var pushes []model.SignalEvent
	if err := query.Find(&pushes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": pushes})
}

func (s *Server) apiJobs(c *gin.Context) {
	limit := parseLimit(c.DefaultQuery("limit", "30"))
	var runs []model.ScannerRun
	if err := s.db.Order("started_at desc").Limit(limit).Find(&runs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": runs})
}

func (s *Server) apiWatchlist(c *gin.Context) {
	var items []model.WatchlistItem
	if err := s.db.Order("updated_at desc").Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (s *Server) apiWatchlistUpsert(c *gin.Context) {
	var payload watchlistPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item, err := s.upsertWatchlist(payload)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"item": item})
}

func (s *Server) apiSettings(c *gin.Context) {
	overrides, err := s.settingsMap()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"settings":  s.publicSettingsPayload(overrides),
		"overrides": maskSecretOverrides(overrides),
	})
}

func (s *Server) apiSettingsUpdate(c *gin.Context) {
	var payload struct {
		Settings map[string]any `json:"settings"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	saved, err := s.upsertSettings(payload.Settings)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"saved": saved})
}

func (s *Server) apiTelegramTest(c *gin.Context) {
	if err := s.sendTelegramTest(c.Request.Context()); err != nil {
		status := http.StatusBadRequest
		if !errors.Is(err, radartelegram.ErrDisabled) {
			status = http.StatusBadGateway
		}
		c.JSON(status, gin.H{"ok": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) apiToken(c *gin.Context) {
	chain := strings.ToLower(strings.TrimSpace(c.Param("chain")))
	address := strings.ToLower(strings.TrimSpace(c.Param("address")))

	var token model.TokenProfile
	err := s.db.Where("chain = ? AND lower(address) = ?", chain, address).First(&token).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "token not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var snapshots []model.TokenSnapshot
	if err := s.db.Where("token_id = ?", token.ID).Order("created_at desc").Limit(20).Find(&snapshots).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var signals []model.SignalEvent
	if err := s.db.Where("chain = ? AND address = ?", token.Chain, token.Address).Order("created_at desc").Limit(20).Find(&signals).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var watchItem *model.WatchlistItem
	var item model.WatchlistItem
	watchErr := s.db.Where("chain = ? AND address = ?", token.Chain, token.Address).First(&item).Error
	if watchErr == nil {
		watchItem = &item
	} else if !errors.Is(watchErr, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": watchErr.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":      token,
		"snapshots":  snapshots,
		"signals":    signals,
		"watch_item": watchItem,
	})
}

func (s *Server) dashboard(c *gin.Context) {
	data := DashboardData{Page: s.page("dashboard")}

	if err := s.db.Where("priority IN ?", []string{"high", "medium"}).Order("created_at desc").Limit(15).Find(&data.HighSignals).Error; err != nil {
		renderError(c, err)
		return
	}
	if err := s.db.Where("signal_type = ?", "resonance").Order("created_at desc").Limit(10).Find(&data.ResonanceSignals).Error; err != nil {
		renderError(c, err)
		return
	}
	if err := s.db.Where("pushed_at IS NOT NULL").Order("pushed_at desc").Limit(8).Find(&data.LatestPushes).Error; err != nil {
		renderError(c, err)
		return
	}
	if err := s.db.Order("started_at desc").Limit(8).Find(&data.RecentRuns).Error; err != nil {
		renderError(c, err)
		return
	}
	if err := s.db.Where("status = ?", "error").Order("started_at desc").Limit(6).Find(&data.JobErrors).Error; err != nil {
		renderError(c, err)
		return
	}
	data.JobWarnings = s.recentJobWarnings(10)

	data.SourceCounts = s.sourceCounts(24)
	data.WatchlistHitCount = s.watchlistHitCount(24)
	data.ActiveWatchlistCount = s.countWatchlist()
	s.render(c, "dashboard.html", data)
}

func (s *Server) signalsPage(c *gin.Context) {
	filters := filtersFromQuery(c)
	query := applySignalFilters(s.db.Model(&model.SignalEvent{}), filters)
	query = query.Order("created_at desc").Limit(200)

	var signals []model.SignalEvent
	if err := query.Find(&signals).Error; err != nil {
		renderError(c, err)
		return
	}

	s.render(c, "signals.html", SignalsData{
		Page:          s.page("signals"),
		Signals:       signals,
		Filters:       filters,
		FilterOptions: s.filterOptions(),
	})
}

func (s *Server) pushesPage(c *gin.Context) {
	filters := filtersFromQuery(c)
	query := applySignalFilters(s.db.Model(&model.SignalEvent{}), filters)
	query = query.Where("pushed_at IS NOT NULL").Order("pushed_at desc").Limit(200)

	var pushes []model.SignalEvent
	if err := query.Find(&pushes).Error; err != nil {
		renderError(c, err)
		return
	}

	s.render(c, "pushes.html", PushesData{
		Page:          s.page("pushes"),
		Pushes:        pushes,
		Filters:       filters,
		FilterOptions: s.filterOptions(),
	})
}

func (s *Server) radarPage(c *gin.Context) {
	source := strings.TrimSpace(c.Param("source"))
	meta, ok := radarMeta(source)
	if !ok {
		c.String(http.StatusNotFound, "unknown radar source")
		return
	}

	data := RadarData{Page: s.page("radar_" + source), Source: source, Meta: meta}
	if err := s.db.Where("source = ?", source).Order("created_at desc").Limit(80).Find(&data.Signals).Error; err != nil {
		renderError(c, err)
		return
	}
	for _, signal := range data.Signals {
		if signal.Priority == "high" || signal.Priority == "medium" {
			data.RecentHigh = append(data.RecentHigh, signal)
			if len(data.RecentHigh) >= 12 {
				break
			}
		}
	}
	if err := s.db.Where("scanner = ?", source).Order("started_at desc").Limit(8).Find(&data.Runs).Error; err != nil {
		renderError(c, err)
		return
	}
	if err := s.db.Model(&model.SignalEvent{}).
		Select("signal_type as type, count(id) as count").
		Where("source = ? AND created_at >= ?", source, cutoffForRange("24h")).
		Group("signal_type").
		Order("count desc, signal_type").
		Scan(&data.TypeCounts).Error; err != nil {
		renderError(c, err)
		return
	}
	_ = s.db.Model(&model.SignalEvent{}).
		Where("source = ? AND pushed_at IS NOT NULL AND created_at >= ?", source, cutoffForRange("24h")).
		Count(&data.PushCount24H).Error

	var latestPush model.SignalEvent
	if err := s.db.Where("source = ? AND pushed_at IS NOT NULL", source).Order("pushed_at desc").First(&latestPush).Error; err == nil {
		data.LatestPush = &latestPush
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		renderError(c, err)
		return
	}
	if err := s.db.Where("scanner = ? AND status = ?", source, "error").Order("started_at desc").Limit(5).Find(&data.LatestErrors).Error; err != nil {
		renderError(c, err)
		return
	}

	s.render(c, "radar.html", data)
}

func (s *Server) tokenPage(c *gin.Context) {
	chain := strings.ToLower(strings.TrimSpace(c.Param("chain")))
	address := strings.ToLower(strings.TrimSpace(c.Param("address")))

	var token model.TokenProfile
	err := s.db.Where("chain = ? AND lower(address) = ?", chain, address).First(&token).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.String(http.StatusNotFound, "token not found")
		return
	}
	if err != nil {
		renderError(c, err)
		return
	}

	data := TokenData{Page: s.page(""), Token: token}
	if err := s.db.Where("token_id = ?", token.ID).Order("created_at desc").Limit(20).Find(&data.Snapshots).Error; err != nil {
		renderError(c, err)
		return
	}
	if err := s.db.Where("chain = ? AND address = ?", token.Chain, token.Address).Order("created_at desc").Limit(20).Find(&data.Signals).Error; err != nil {
		renderError(c, err)
		return
	}
	var watchItem model.WatchlistItem
	watchErr := s.db.Where("chain = ? AND address = ?", token.Chain, token.Address).First(&watchItem).Error
	if watchErr == nil {
		data.WatchItem = &watchItem
	} else if !errors.Is(watchErr, gorm.ErrRecordNotFound) {
		renderError(c, watchErr)
		return
	}
	data.Tags = parseJSONStringSlice(token.NarrativeTagsJSON)
	data.SocialLinks = parseJSONStringMap(token.SocialLinksJSON)
	s.render(c, "token.html", data)
}

func (s *Server) watchlistPage(c *gin.Context) {
	var items []model.WatchlistItem
	if err := s.db.Order("updated_at desc").Find(&items).Error; err != nil {
		renderError(c, err)
		return
	}
	s.render(c, "watchlist.html", WatchlistData{Page: s.page("watchlist"), Items: items})
}

func (s *Server) watchlistSubmit(c *gin.Context) {
	payload := watchlistPayload{
		Chain:   c.PostForm("chain"),
		Address: c.PostForm("address"),
		Symbol:  c.PostForm("symbol"),
		Name:    c.PostForm("name"),
		Status:  firstNonEmpty(c.PostForm("status"), "watch"),
		Note:    c.PostForm("note"),
	}
	if _, err := s.upsertWatchlist(payload); err != nil {
		renderError(c, err)
		return
	}
	target := c.GetHeader("Referer")
	if target == "" {
		target = "/watchlist"
	}
	c.Redirect(http.StatusSeeOther, target)
}

func (s *Server) jobsPage(c *gin.Context) {
	var runs []model.ScannerRun
	if err := s.db.Order("started_at desc").Limit(50).Find(&runs).Error; err != nil {
		renderError(c, err)
		return
	}
	lastRun := make(map[string]*model.ScannerRun)
	for i := range runs {
		run := runs[i]
		if _, ok := lastRun[run.Scanner]; !ok {
			lastRun[run.Scanner] = &run
		}
	}

	specRows := scannerSpecs
	if s.scheduler != nil {
		specRows = make([]JobRow, 0, len(scannerSpecs))
		for _, spec := range s.scheduler.Specs() {
			meta, _ := radarMeta(spec.Name)
			specRows = append(specRows, JobRow{
				Scanner:         spec.Name,
				Label:           meta.Label,
				Summary:         meta.Summary,
				Focus:           meta.Focus,
				Implemented:     true,
				IntervalSeconds: spec.IntervalSeconds,
			})
		}
	}

	rows := make([]JobRow, 0, len(specRows))
	for _, spec := range specRows {
		rows = append(rows, JobRow{
			Scanner:         spec.Scanner,
			Label:           spec.Label,
			Summary:         spec.Summary,
			Focus:           spec.Focus,
			Implemented:     spec.Implemented,
			IntervalSeconds: spec.IntervalSeconds,
			LastRun:         lastRun[spec.Scanner],
		})
	}

	s.render(c, "jobs.html", JobsData{Page: s.page("jobs"), Rows: rows})
}

func (s *Server) settingsPage(c *gin.Context) {
	rows, err := s.settingRows()
	if err != nil {
		renderError(c, err)
		return
	}
	s.render(c, "settings.html", SettingsData{
		Page:        s.page("settings"),
		Rows:        rows,
		SaveStatus:  c.Query("save_status"),
		SaveMessage: c.Query("save_message"),
		TestStatus:  c.Query("tg_test_status"),
		TestMessage: c.Query("tg_test_message"),
	})
}

func (s *Server) settingsSubmit(c *gin.Context) {
	updates := make(map[string]any)
	for _, key := range visibleSettingKeys {
		raw := c.PostForm(key)
		defaultValue := defaultSettingValue(key)
		updates[key] = coerceSettingValue(raw, defaultValue)
	}
	if _, err := s.upsertSettings(updates); err != nil {
		renderError(c, err)
		return
	}
	c.Redirect(http.StatusSeeOther, "/settings?save_status=ok&save_message=settings_saved")
}

func (s *Server) telegramTestPage(c *gin.Context) {
	if err := s.sendTelegramTest(c.Request.Context()); err != nil {
		c.Redirect(http.StatusSeeOther, "/settings?tg_test_status=error&tg_test_message="+urlQueryEscape(err.Error()))
		return
	}
	c.Redirect(http.StatusSeeOther, "/settings?tg_test_status=ok&tg_test_message=telegram_test_sent")
}

func (s *Server) render(c *gin.Context, name string, data any) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(c.Writer, name, data); err != nil {
		renderError(c, err)
	}
}

func renderError(c *gin.Context, err error) {
	c.String(http.StatusInternalServerError, err.Error())
}

func (s *Server) page(activeNav string) PageData {
	return PageData{
		AppName:       s.settings.AppName,
		ActiveNav:     activeNav,
		SidebarGroups: sidebarGroups(),
	}
}

func filtersFromQuery(c *gin.Context) SignalFilters {
	timeRange := strings.TrimSpace(c.DefaultQuery("time_range", "24h"))
	if timeRange == "" {
		timeRange = "24h"
	}
	return SignalFilters{
		Source:        strings.TrimSpace(c.Query("source")),
		Chain:         strings.TrimSpace(c.Query("chain")),
		Symbol:        strings.TrimSpace(c.Query("symbol")),
		SignalType:    strings.TrimSpace(c.Query("signal_type")),
		Priority:      strings.TrimSpace(c.Query("priority")),
		TimeRange:     timeRange,
		WatchlistOnly: strings.EqualFold(c.Query("watchlist_only"), "true") || strings.EqualFold(c.Query("watchlist_only"), "on"),
	}
}

func applySignalFilters(query *gorm.DB, filters SignalFilters) *gorm.DB {
	query = query.Where("created_at >= ?", cutoffForRange(filters.TimeRange))
	if filters.Source != "" {
		query = query.Where("source = ?", filters.Source)
	}
	if filters.Chain != "" {
		query = query.Where("chain = ?", filters.Chain)
	}
	if filters.Symbol != "" {
		like := "%" + filters.Symbol + "%"
		query = query.Where("symbol LIKE ? OR address LIKE ?", like, like)
	}
	if filters.SignalType != "" {
		query = query.Where("signal_type = ?", filters.SignalType)
	}
	if filters.Priority != "" {
		query = query.Where("priority = ?", filters.Priority)
	}
	if filters.WatchlistOnly {
		query = query.Joins("JOIN watchlist ON watchlist.chain = signals.chain AND watchlist.address = signals.address")
	}
	return query
}

func cutoffForRange(value string) string {
	hours := 24
	switch value {
	case "1h":
		hours = 1
	case "6h":
		hours = 6
	case "7d":
		hours = 24 * 7
	}
	return time.Now().UTC().Add(-time.Duration(hours) * time.Hour).Format(time.RFC3339Nano)
}

func (s *Server) sourceCounts(hours int) map[string]int64 {
	type row struct {
		Source string // Source 是 signals.source 分组值。
		Count  int64  // Count 是该来源在时间窗口内的信号数量。
	}
	var rows []row
	counts := make(map[string]int64)
	err := s.db.Model(&model.SignalEvent{}).
		Select("source, count(id) as count").
		Where("created_at >= ?", time.Now().UTC().Add(-time.Duration(hours)*time.Hour).Format(time.RFC3339Nano)).
		Group("source").
		Scan(&rows).Error
	if err != nil {
		return counts
	}
	for _, row := range rows {
		counts[row.Source] = row.Count
	}
	return counts
}

func (s *Server) watchlistHitCount(hours int) int64 {
	var count int64
	_ = s.db.Model(&model.SignalEvent{}).
		Joins("JOIN watchlist ON watchlist.chain = signals.chain AND watchlist.address = signals.address").
		Where("signals.created_at >= ?", time.Now().UTC().Add(-time.Duration(hours)*time.Hour).Format(time.RFC3339Nano)).
		Count(&count).Error
	return count
}

func (s *Server) countWatchlist() int64 {
	var count int64
	_ = s.db.Model(&model.WatchlistItem{}).Count(&count).Error
	return count
}

func (s *Server) recentJobWarnings(limit int) []JobWarning {
	var runs []model.ScannerRun
	if err := s.db.Order("started_at desc").Limit(limit).Find(&runs).Error; err != nil {
		return nil
	}
	warnings := []JobWarning{}
	for _, run := range runs {
		row := JobWarning{Scanner: run.Scanner, StartedAt: run.StartedAt}
		row.Warnings = firstNStrings(runWarnings(run), 3)
		if len(row.Warnings) > 0 {
			warnings = append(warnings, row)
		}
	}
	return warnings
}

func runWarnings(value any) []string {
	var raw string
	switch run := value.(type) {
	case model.ScannerRun:
		raw = run.MetadataJSON
	case *model.ScannerRun:
		if run == nil {
			return nil
		}
		raw = run.MetadataJSON
	default:
		return nil
	}
	var metadata map[string]any
	if json.Unmarshal([]byte(raw), &metadata) != nil {
		return nil
	}
	rawWarnings, ok := metadata["warnings"].([]any)
	if !ok {
		return nil
	}
	warnings := make([]string, 0, len(rawWarnings))
	for _, item := range rawWarnings {
		text := strings.TrimSpace(toString(item))
		if text != "" {
			warnings = append(warnings, text)
		}
	}
	return warnings
}

func warningCount(value any) int {
	return len(runWarnings(value))
}

func firstWarning(value any) string {
	warnings := runWarnings(value)
	if len(warnings) == 0 {
		return ""
	}
	return warnings[0]
}

func firstNStrings(values []string, n int) []string {
	if len(values) <= n {
		return values
	}
	return values[:n]
}

func (s *Server) filterOptions() FilterOptions {
	var sources []string
	var chains []string
	var signalTypes []string
	_ = s.db.Model(&model.SignalEvent{}).Distinct().Order("source").Pluck("source", &sources).Error
	_ = s.db.Model(&model.SignalEvent{}).Distinct().Order("chain").Pluck("chain", &chains).Error
	_ = s.db.Model(&model.SignalEvent{}).Distinct().Order("signal_type").Pluck("signal_type", &signalTypes).Error
	return FilterOptions{
		Sources:     mergeOptions(sourceOrder(), sources),
		Chains:      mergeOptions([]string{"binance_perp", "binance_alpha", "ethereum", "eth", "bsc", "base", "sol", "solana"}, chains),
		SignalTypes: mergeOptions(defaultSignalTypes(), signalTypes),
		Priorities:  []string{"high", "medium", "low"},
		TimeRanges:  []string{"1h", "6h", "24h", "7d"},
	}
}

func (s *Server) upsertWatchlist(payload watchlistPayload) (model.WatchlistItem, error) {
	chain := strings.ToLower(strings.TrimSpace(payload.Chain))
	address := strings.ToLower(strings.TrimSpace(payload.Address))
	if chain == "" || address == "" {
		return model.WatchlistItem{}, errors.New("chain and address are required")
	}
	status := firstNonEmpty(strings.TrimSpace(payload.Status), "watch")
	now := time.Now().UTC().Format(time.RFC3339Nano)

	var item model.WatchlistItem
	err := s.db.Where("chain = ? AND address = ?", chain, address).First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		item = model.WatchlistItem{
			Chain:     chain,
			Address:   address,
			Symbol:    strings.ToUpper(strings.TrimSpace(payload.Symbol)),
			Name:      strings.TrimSpace(payload.Name),
			Status:    status,
			Note:      strings.TrimSpace(payload.Note),
			UpdatedAt: now,
		}
		return item, s.db.Create(&item).Error
	}
	if err != nil {
		return model.WatchlistItem{}, err
	}
	if strings.TrimSpace(payload.Symbol) != "" {
		item.Symbol = strings.ToUpper(strings.TrimSpace(payload.Symbol))
	}
	if strings.TrimSpace(payload.Name) != "" {
		item.Name = strings.TrimSpace(payload.Name)
	}
	item.Status = status
	item.Note = strings.TrimSpace(payload.Note)
	item.UpdatedAt = now
	return item, s.db.Save(&item).Error
}

func (s *Server) settingRows() ([]SettingRow, error) {
	overrides, err := s.settingsMap()
	if err != nil {
		return nil, err
	}
	rows := make([]SettingRow, 0, len(visibleSettingKeys))
	for _, key := range visibleSettingKeys {
		defaultValue := defaultSettingValue(key)
		effective := defaultValue
		overrideText := "-"
		if override, ok := overrides[key]; ok {
			effective = override
			overrideText = displaySettingValue(override, key)
		}
		rows = append(rows, SettingRow{
			Key:      key,
			Value:    displaySettingValue(effective, key),
			Default:  displaySettingValue(defaultValue, key),
			Override: overrideText,
			IsBool:   isBoolDefault(defaultValue),
		})
	}
	return rows, nil
}

func (s *Server) settingsMap() (map[string]any, error) {
	var rows []model.AppSetting
	if err := s.db.Order("key").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[string]any, len(rows))
	for _, row := range rows {
		var value any
		if err := json.Unmarshal([]byte(row.ValueJSON), &value); err != nil {
			value = row.ValueJSON
		}
		result[row.Key] = value
	}
	return result, nil
}

func (s *Server) upsertSettings(settings map[string]any) (map[string]any, error) {
	saved := make(map[string]any)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for key, value := range settings {
		if !isOverridableSetting(key) {
			continue
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		row := model.AppSetting{Key: key, ValueJSON: string(encoded), UpdatedAt: now}
		if err := s.db.Save(&row).Error; err != nil {
			return nil, err
		}
		saved[key] = value
	}
	return saved, nil
}

func (s *Server) publicSettingsPayload(overrides map[string]any) map[string]any {
	payload := make(map[string]any)
	for _, key := range visibleSettingKeys {
		value := defaultSettingValue(key)
		if override, ok := overrides[key]; ok {
			value = override
		}
		if key == "tg_bot_token" || key == "gmgn_cookie" {
			payload[key] = maskSecret(displaySettingValue(value, key))
		} else {
			payload[key] = value
		}
	}
	return payload
}

func (s *Server) sendTelegramTest(ctx context.Context) error {
	settings, err := s.effectiveTelegramSettings()
	if err != nil {
		return err
	}
	notifier, err := radartelegram.New(settings)
	if err != nil {
		return err
	}
	return notifier.SendText(ctx, "Web3 Online Radar Telegram test from Go Radar")
}

func (s *Server) effectiveTelegramSettings() (radartelegram.Settings, error) {
	overrides, err := s.settingsMap()
	if err != nil {
		return radartelegram.Settings{}, err
	}
	proxyURL := effectiveString(overrides, "tg_proxy_url")
	if proxyURL == "" {
		proxyURL = effectiveString(overrides, "selective_proxy_url")
	}
	return radartelegram.Settings{
		BotToken: effectiveString(overrides, "tg_bot_token"),
		ChatID:   effectiveString(overrides, "tg_chat_id"),
		ProxyURL: proxyURL,
		TrustEnv: effectiveBool(overrides, "http_trust_env"),
	}, nil
}

func parseLimit(raw string) int {
	limit, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || limit <= 0 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}

func parsePagination(c *gin.Context) (int, int) {
	page, err := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("page", c.DefaultQuery("current", "1"))))
	if err != nil || page <= 0 {
		page = 1
	}

	pageSize := parseLimit(c.DefaultQuery("pageSize", c.Query("limit")))
	return page, pageSize
}

func sidebarGroups() []SidebarGroup {
	return []SidebarGroup{
		{
			Title: "总览",
			Items: []SidebarItem{
				{Label: "Dashboard", Href: "/dashboard", Key: "dashboard"},
				{Label: "信号列表", Href: "/signals", Key: "signals"},
				{Label: "TG 推送", Href: "/pushes", Key: "pushes"},
				{Label: "观察名单", Href: "/watchlist", Key: "watchlist"},
			},
		},
		{
			Title: "雷达模块",
			Items: []SidebarItem{
				{Label: "S1 币安公告", Href: "/radar/s1", Key: "radar_s1"},
				{Label: "S2 费率翻转", Href: "/radar/s2", Key: "radar_s2"},
				{Label: "S3 热度确认", Href: "/radar/s3", Key: "radar_s3"},
				{Label: "S5 链上发现", Href: "/radar/s5", Key: "radar_s5"},
				{Label: "S7 Vitalik Sell", Href: "/radar/s7", Key: "radar_s7"},
			},
		},
		{
			Title: "系统",
			Items: []SidebarItem{
				{Label: "任务状态", Href: "/jobs", Key: "jobs"},
				{Label: "运行设置", Href: "/settings", Key: "settings"},
			},
		},
	}
}

func radarMeta(source string) (RadarMeta, bool) {
	meta := map[string]RadarMeta{
		"s1":     {Label: "S1 币安公告", Summary: "监控 Binance 公告、Alpha、空投和上所预期，偏事件驱动。", Focus: "公告催化 / 事件预期"},
		"s2":     {Label: "S2 费率翻转", Summary: "监控费率由正转负且 OI 持续上升的合约环境，偏逼空观察。", Focus: "费率翻转 / OI 增长"},
		"s3":     {Label: "S3 热度确认", Summary: "监控热度、负费率、持仓量变化，偏合约市场资金确认。", Focus: "热度 / 费率 / OI"},
		"s5":     {Label: "S5 链上发现", Summary: "监控链上新币、叙事币、FLAP 支撑和连续动量，偏早期发现。", Focus: "链上新币 / 叙事 / FLAP"},
		"s7":     {Label: "S7 Vitalik Sell", Summary: "监控 Vitalik 地址 ERC-20 转出，识别 DEX、CEX 或 LP 路径。", Focus: "DEX / CEX / LP 转出"},
		"system": {Label: "系统共振", Summary: "多个雷达来源同时命中同一标的时生成的增强信号。", Focus: "跨源共振"},
	}
	value, ok := meta[source]
	return value, ok
}

func sourceOrder() []string {
	return []string{"s1", "s2", "s3", "s5", "s7"}
}

func defaultSignalTypes() []string {
	return []string{
		"vitalik_sell",
		"heat",
		"heat_plus_oi",
		"heat_plus_negative_funding",
		"oi_anomaly",
		"momentum",
		"narrative_tagged",
		"flap_support",
		"resonance",
		"funding_flip_oi_rising",
		"alpha_discovery",
		"listing",
		"airdrop",
		"alpha",
	}
}

func mergeOptions(defaults []string, values []string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(defaults)+len(values))
	for _, value := range append(defaults, values...) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func sourceLabel(source string) string {
	labels := map[string]string{
		"s1":     "S1 币安公告",
		"s2":     "S2 费率翻转",
		"s3":     "S3 热度确认",
		"s5":     "S5 链上发现",
		"s7":     "S7 Vitalik Sell",
		"system": "系统共振",
	}
	if label, ok := labels[source]; ok {
		return label
	}
	return source
}

func chainLabel(chain string) string {
	labels := map[string]string{
		"binance_perp":  "Binance Perp",
		"binance_alpha": "币安公告",
		"eth":           "Ethereum",
		"ethereum":      "Ethereum",
		"bsc":           "BSC",
		"base":          "Base",
		"sol":           "Solana",
		"solana":        "Solana",
	}
	if label, ok := labels[chain]; ok {
		return label
	}
	return chain
}

func signalTypeLabel(signalType string) string {
	labels := map[string]string{
		"vitalik_sell":               "Vitalik 卖出",
		"heat":                       "热度",
		"heat_plus_oi":               "热度 + OI",
		"heat_plus_negative_funding": "热度 + 负费率",
		"oi_anomaly":                 "OI 异动",
		"momentum":                   "连续动量",
		"narrative_tagged":           "叙事命中",
		"flap_support":               "FLAP 支撑",
		"resonance":                  "跨源共振",
		"funding_flip_oi_rising":     "费率翻转 + OI",
		"alpha_discovery":            "币安公告发现",
		"listing":                    "正式上币",
		"airdrop":                    "HODLer 空投",
		"alpha":                      "币安 Alpha",
	}
	if label, ok := labels[signalType]; ok {
		return label
	}
	return signalType
}

func priorityClass(priority string) string {
	switch priority {
	case "high":
		return "priority-high"
	case "medium":
		return "priority-medium"
	case "low":
		return "priority-low"
	default:
		return ""
	}
}

func priorityLabel(priority string) string {
	labels := map[string]string{
		"high":   "高",
		"medium": "中",
		"low":    "低",
	}
	if label, ok := labels[priority]; ok {
		return label
	}
	return priority
}

func statusLabel(status string) string {
	labels := map[string]string{
		"ok":      "正常",
		"warning": "告警",
		"error":   "错误",
		"skipped": "跳过",
		"running": "运行中",
		"watch":   "观察",
		"active":  "活跃",
		"paused":  "暂停",
	}
	if label, ok := labels[status]; ok {
		return label
	}
	return status
}

func formatScore(score float64) string {
	return strconv.FormatFloat(score, 'f', 1, 64)
}

func formatTime(value any) string {
	var raw string
	switch typed := value.(type) {
	case string:
		raw = typed
	case *string:
		if typed == nil {
			return "-"
		}
		raw = *typed
	default:
		raw = toString(typed)
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "-"
	}
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return raw
	}
	return parsed.In(time.FixedZone("CST", 8*3600)).Format("01-02 15:04")
}

func sumCounts(counts map[string]int64) int64 {
	var total int64
	for _, count := range counts {
		total += count
	}
	return total
}

func parseJSONStringSlice(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" {
		return nil
	}
	var items []string
	if json.Unmarshal([]byte(raw), &items) == nil {
		return items
	}
	return nil
}

func parseJSONStringMap(raw string) map[string]string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "{}" {
		return nil
	}
	result := make(map[string]string)
	if json.Unmarshal([]byte(raw), &result) == nil {
		return result
	}
	return nil
}

func urlQueryEscape(value string) string {
	replacer := strings.NewReplacer(" ", "+", "\n", "+", "\r", "+", "&", "%26", "=", "%3D", "?", "%3F")
	return replacer.Replace(value)
}

type watchlistPayload struct {
	Chain   string `json:"chain"`   // Chain 是观察名单 token 所在链。
	Address string `json:"address"` // Address 是观察名单 token 地址。
	Symbol  string `json:"symbol"`  // Symbol 是观察名单 token 符号。
	Name    string `json:"name"`    // Name 是观察名单 token 名称。
	Status  string `json:"status"`  // Status 是观察名单状态。
	Note    string `json:"note"`    // Note 是用户备注。
}

func defaultSettingValue(key string) any {
	switch key {
	case "http_trust_env", "enable_scanner_s7", "enable_scanner_s5", "enable_scanner_s3", "enable_scanner_s2", "enable_scanner_s1", "enable_binance_square", "enable_insider_monitor":
		return envBool(keyToEnv(key), false)
	case "scan_interval_insider":
		return envInt("SCAN_INTERVAL_INSIDER", 300)
	case "scan_interval_s7":
		return envInt("SCAN_INTERVAL_S7", 20)
	case "scan_interval_s5":
		return envInt("SCAN_INTERVAL_S5", 120)
	case "scan_interval_s3":
		return envInt("SCAN_INTERVAL_S3", 300)
	case "scan_interval_s2":
		return envInt("SCAN_INTERVAL_S2", 120)
	case "scan_interval_s1":
		return envInt("SCAN_INTERVAL_S1", 30)
	case "gmgn_retries":
		return envInt("GMGN_RETRIES", 0)
	case "s7_min_notify_usd":
		return envInt("S7_MIN_NOTIFY_USD", 0)
	case "s5_momentum_consecutive_up":
		return envInt("S5_MOMENTUM_CONSECUTIVE_UP", 3)
	case "s5_momentum_medium_quota":
		return envInt("S5_MOMENTUM_MEDIUM_QUOTA", 1)
	case "s3_top_volume_limit":
		return envInt("S3_TOP_VOLUME_LIMIT", 100)
	case "s3_volume_lookback_limit":
		return envInt("S3_VOLUME_LOOKBACK_LIMIT", 80)
	case "signal_time_bucket_minutes":
		return envInt("SIGNAL_TIME_BUCKET_MINUTES", 30)
	case "token_push_cooldown_minutes":
		return envInt("TOKEN_PUSH_COOLDOWN_MINUTES", 180)
	case "watchlist_cooldown_minutes":
		return envInt("WATCHLIST_COOLDOWN_MINUTES", 30)
	case "s3_digest_cooldown_minutes":
		return envInt("S3_DIGEST_COOLDOWN_MINUTES", 10)
	case "resonance_lookback_minutes":
		return envInt("RESONANCE_LOOKBACK_MINUTES", 360)
	case "insider_monitor_engine":
		return firstNonEmpty(os.Getenv("INSIDER_MONITOR_ENGINE"), "service")
	case "gmgn_timeout_seconds":
		return envFloat("GMGN_TIMEOUT_SECONDS", 6)
	case "s5_min_gain_pct":
		return envFloat("S5_MIN_GAIN_PCT", 5)
	case "s5_min_mc":
		return envFloat("S5_MIN_MC", 1000)
	case "s5_max_mc":
		return envFloat("S5_MAX_MC", 10000000)
	case "s5_min_liq":
		return envFloat("S5_MIN_LIQ", 500)
	case "s3_vol_surge_mult":
		return envFloat("S3_VOL_SURGE_MULT", 2.5)
	case "s3_min_vol_usd":
		return envFloat("S3_MIN_VOL_USD", 20000000)
	case "s3_min_oi_delta_pct":
		return envFloat("S3_MIN_OI_DELTA_PCT", 3)
	case "s3_min_oi_usd":
		return envFloat("S3_MIN_OI_USD", 2000000)
	case "selective_proxy_domains":
		return os.Getenv("SELECTIVE_PROXY_DOMAINS")
	default:
		return os.Getenv(keyToEnv(key))
	}
}

func keyToEnv(key string) string {
	return strings.ToUpper(key)
}

func envBool(key string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	return raw == "1" || strings.EqualFold(raw, "true") || strings.EqualFold(raw, "yes") || strings.EqualFold(raw, "on")
}

func envInt(key string, fallback int) int {
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

func envFloat(key string, fallback float64) float64 {
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

func coerceSettingValue(raw string, defaultValue any) any {
	switch defaultValue.(type) {
	case bool:
		return raw == "1" || strings.EqualFold(raw, "true") || strings.EqualFold(raw, "yes") || strings.EqualFold(raw, "on")
	case int:
		value, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil {
			return defaultValue
		}
		return value
	case float64:
		value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		if err != nil {
			return defaultValue
		}
		return value
	default:
		return raw
	}
}

func displaySettingValue(value any, key string) string {
	if key == "tg_bot_token" || key == "gmgn_cookie" {
		return maskSecret(toString(value))
	}
	return toString(value)
}

func effectiveString(overrides map[string]any, key string) string {
	if override, ok := overrides[key]; ok {
		return toString(override)
	}
	return toString(defaultSettingValue(key))
}

func effectiveBool(overrides map[string]any, key string) bool {
	if override, ok := overrides[key]; ok {
		switch value := override.(type) {
		case bool:
			return value
		case string:
			return value == "1" || strings.EqualFold(value, "true") || strings.EqualFold(value, "yes") || strings.EqualFold(value, "on")
		}
	}
	defaultValue, ok := defaultSettingValue(key).(bool)
	return ok && defaultValue
}

func toString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case bool:
		if v {
			return "true"
		}
		return "false"
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, toString(item))
		}
		return strings.Join(parts, ",")
	default:
		return strings.TrimSpace(strings.Trim(fmtAny(v), `"`))
	}
}

func fmtAny(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func maskSecret(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return "***"
}

func maskSecretOverrides(overrides map[string]any) map[string]any {
	result := make(map[string]any, len(overrides))
	for key, value := range overrides {
		if key == "gmgn_cookie" || key == "tg_bot_token" {
			result[key] = maskSecret(toString(value))
		} else {
			result[key] = value
		}
	}
	return result
}

func isBoolDefault(value any) bool {
	_, ok := value.(bool)
	return ok
}

func isOverridableSetting(key string) bool {
	for _, item := range overridableSettingKeys {
		if item == key {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

var visibleSettingKeys = []string{
	"http_trust_env",
	"selective_proxy_url",
	"selective_proxy_domains",
	"tg_proxy_url",
	"s7_eth_rpc_url",
	"enable_scanner_s7",
	"enable_scanner_s5",
	"enable_scanner_s3",
	"enable_scanner_s2",
	"enable_scanner_s1",
	"enable_insider_monitor",
	"enable_binance_square",
	"scan_interval_insider",
	"scan_interval_s7",
	"scan_interval_s5",
	"scan_interval_s3",
	"scan_interval_s2",
	"scan_interval_s1",
	"gmgn_timeout_seconds",
	"gmgn_retries",
	"s7_min_notify_usd",
	"s5_momentum_consecutive_up",
	"s5_momentum_medium_quota",
	"s5_min_gain_pct",
	"s3_vol_surge_mult",
	"s3_min_vol_usd",
	"s3_min_oi_delta_pct",
	"s3_min_oi_usd",
	"signal_time_bucket_minutes",
	"token_push_cooldown_minutes",
	"watchlist_cooldown_minutes",
	"s3_digest_cooldown_minutes",
	"resonance_lookback_minutes",
	"insider_monitor_engine",
}

var overridableSettingKeys = append([]string{
	"tg_bot_token",
	"tg_chat_id",
	"gmgn_cookie",
}, visibleSettingKeys...)

var scannerSpecs = []JobRow{
	{Scanner: "s1", Label: "S1 Binance Alpha", Summary: "Binance announcement and Alpha event discovery.", Focus: "announcement catalyst", Implemented: true, IntervalSeconds: 30},
	{Scanner: "s2", Label: "S2 OI / Funding", Summary: "Funding flips and open interest expansion.", Focus: "funding / OI", Implemented: true, IntervalSeconds: 120},
	{Scanner: "s3", Label: "S3 Heat", Summary: "Heat, volume, funding and OI confirmation.", Focus: "heat / volume / OI", Implemented: true, IntervalSeconds: 300},
	{Scanner: "s5", Label: "S5 On-chain", Summary: "New token, narrative and on-chain momentum discovery.", Focus: "on-chain early discovery", Implemented: true, IntervalSeconds: 120},
	{Scanner: "s7", Label: "S7 Vitalik Sell", Summary: "Vitalik outbound token transfer monitor.", Focus: "DEX / CEX / LP outflow", Implemented: true, IntervalSeconds: 20},
}
