package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type API struct {
	db *gorm.DB
}

type siteInput struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	Description string `json:"description"`
}

type scanView struct {
	Scan
	IssueCount     int64 `json:"issueCount"`
	HighIssueCount int64 `json:"highIssueCount"`
}

type dashboardStats struct {
	SiteCount    int   `json:"siteCount"`
	ScanCount    int64 `json:"scanCount"`
	OpenIssues   int64 `json:"openIssues"`
	HighPriority int64 `json:"highPriority"`
	AverageScore int   `json:"averageScore"`
}

type dashboardSite struct {
	Site
	LatestScan   *Scan `json:"latestScan"`
	OpenIssues   int64 `json:"openIssues"`
	HighPriority int64 `json:"highPriority"`
}

type dashboardRecentScan struct {
	Scan
	SiteName string `json:"siteName"`
}

type scanComparison struct {
	BaseScanID      uint      `json:"baseScanId"`
	NewCount        int       `json:"newCount"`
	PersistingCount int       `json:"persistingCount"`
	ResolvedCount   int       `json:"resolvedCount"`
	New             []Finding `json:"new"`
	Resolved        []Finding `json:"resolved"`
}

func newRouter(db *gorm.DB, allowedOrigins map[string]bool) *gin.Engine {
	api := &API{db: db}
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery(), corsMiddleware(allowedOrigins), requestIDMiddleware(), bodyLimitMiddleware(1<<20))
	r.GET("/api/health", api.handleHealth)
	r.GET("/api/dashboard", api.handleDashboard)
	r.GET("/api/sites", api.handleListSites)
	r.POST("/api/sites", api.handleCreateSite)
	r.GET("/api/sites/:id", api.handleGetSite)
	r.PUT("/api/sites/:id", api.handleUpdateSite)
	r.DELETE("/api/sites/:id", api.handleDeleteSite)
	r.POST("/api/sites/:id/scans", api.handleCreateScan)
	r.GET("/api/sites/:id/findings", api.handleSiteFindings)
	r.PATCH("/api/issues/:id", api.handleUpdateIssue)
	r.GET("/api/scans/:id", api.handleGetScan)
	r.GET("/api/scans/:id/report", api.handleHTMLReport)
	r.GET("/api/scans/:id/report.json", api.handleJSONReport)
	return r
}

func corsMiddleware(allowed map[string]bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && allowed[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Max-Age", "600")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.GetHeader("X-Request-ID"))
		if id == "" || len(id) > 128 {
			id = fmt.Sprintf("req_%d", time.Now().UnixNano())
		}
		c.Header("X-Request-ID", id)
		c.Next()
	}
}

func bodyLimitMiddleware(limit int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		}
		c.Next()
	}
}

func abortError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{
		"code":    code,
		"message": message,
		"error":   message, // retained for older clients
	})
}

func parseID(c *gin.Context) (uint, bool) {
	value, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || value == 0 {
		abortError(c, http.StatusBadRequest, "VALIDATION_ERROR", "无效的资源编号")
		return 0, false
	}
	return uint(value), true
}

func (a *API) handleHealth(c *gin.Context) {
	sqlDB, err := a.db.DB()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "error", "database": "unavailable"})
		return
	}
	if err := sqlDB.Ping(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "error", "database": "unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "database": "ok"})
}

func (a *API) handleListSites(c *gin.Context) {
	var sites []Site
	if err := a.db.Order("created_at desc").Find(&sites).Error; err != nil {
		abortError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "无法读取网站列表")
		return
	}
	c.JSON(http.StatusOK, sites)
}

func validateSiteInput(input *siteInput) (string, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.URL = strings.TrimSpace(input.URL)
	input.Description = strings.TrimSpace(input.Description)
	if input.Name == "" || input.URL == "" {
		return "", fmt.Errorf("网站名称和地址不能为空")
	}
	if len([]rune(input.Name)) > 160 {
		return "", fmt.Errorf("网站名称不能超过 160 个字符")
	}
	if len([]rune(input.Description)) > 4000 {
		return "", fmt.Errorf("备注不能超过 4000 个字符")
	}
	normalized, err := normalizeTarget(input.URL)
	if err != nil {
		return "", err
	}
	return normalized, nil
}

func (a *API) handleCreateSite(c *gin.Context) {
	var input siteInput
	if err := c.ShouldBindJSON(&input); err != nil {
		abortError(c, http.StatusBadRequest, "VALIDATION_ERROR", "请求内容不是合法的 JSON")
		return
	}
	normalized, err := validateSiteInput(&input)
	if err != nil {
		abortError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	site := Site{Name: input.Name, URL: normalized, Description: input.Description}
	if err := a.db.Create(&site).Error; err != nil {
		abortError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "创建网站失败")
		return
	}
	c.JSON(http.StatusCreated, site)
}

func (a *API) handleGetSite(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var site Site
	if err := a.db.First(&site, id).Error; err != nil {
		abortError(c, http.StatusNotFound, "RESOURCE_NOT_FOUND", "网站不存在")
		return
	}
	var scans []Scan
	if err := a.db.Where("site_id = ?", site.ID).Order("id desc").Limit(20).Find(&scans).Error; err != nil {
		abortError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "无法读取扫描记录")
		return
	}
	views := make([]scanView, 0, len(scans))
	for _, scan := range scans {
		view := scanView{Scan: scan}
		a.db.Model(&Finding{}).Where("scan_id = ?", scan.ID).Count(&view.IssueCount)
		a.db.Model(&Finding{}).Where("scan_id = ? AND severity IN ?", scan.ID, []string{"critical", "high"}).Count(&view.HighIssueCount)
		views = append(views, view)
	}
	response := gin.H{"site": site, "scans": views}
	if len(views) > 0 {
		response["latestScan"] = views[0]
	}
	c.JSON(http.StatusOK, response)
}

func (a *API) handleUpdateSite(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var site Site
	if err := a.db.First(&site, id).Error; err != nil {
		abortError(c, http.StatusNotFound, "RESOURCE_NOT_FOUND", "网站不存在")
		return
	}
	var input siteInput
	if err := c.ShouldBindJSON(&input); err != nil {
		abortError(c, http.StatusBadRequest, "VALIDATION_ERROR", "请求内容不是合法的 JSON")
		return
	}
	normalized, err := validateSiteInput(&input)
	if err != nil {
		abortError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	updates := map[string]any{"name": input.Name, "url": normalized, "description": input.Description}
	if err := a.db.Model(&site).Updates(updates).Error; err != nil {
		abortError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "更新网站失败")
		return
	}
	a.db.First(&site, site.ID)
	c.JSON(http.StatusOK, site)
}

func (a *API) handleDeleteSite(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var site Site
	if err := a.db.First(&site, id).Error; err != nil {
		abortError(c, http.StatusNotFound, "RESOURCE_NOT_FOUND", "网站不存在")
		return
	}
	err := a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("site_id = ?", site.ID).Delete(&IssueEvent{}).Error; err != nil {
			return err
		}
		if err := tx.Where("site_id = ?", site.ID).Delete(&Finding{}).Error; err != nil {
			return err
		}
		if err := tx.Where("site_id = ?", site.ID).Delete(&Scan{}).Error; err != nil {
			return err
		}
		return tx.Delete(&site).Error
	})
	if err != nil {
		abortError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "删除网站失败，数据未被修改")
		return
	}
	c.Status(http.StatusNoContent)
}

func (a *API) handleDashboard(c *gin.Context) {
	var sites []Site
	if err := a.db.Order("created_at desc").Find(&sites).Error; err != nil {
		abortError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "无法读取工作台数据")
		return
	}
	var scanCount int64
	a.db.Model(&Scan{}).Count(&scanCount)
	stats := dashboardStats{SiteCount: len(sites), ScanCount: scanCount}
	sumScore, scoredSites := 0, 0
	items := make([]dashboardSite, 0, len(sites))
	for _, site := range sites {
		item := dashboardSite{Site: site}
		if scan := latestScan(a.db, site.ID, ""); scan != nil {
			item.LatestScan = scan
		}
		if scan := latestScan(a.db, site.ID, ScanStatusCompleted); scan != nil {
			a.db.Model(&Finding{}).Where("scan_id = ? AND status IN ?", scan.ID, activeIssueStatuses()).Count(&item.OpenIssues)
			a.db.Model(&Finding{}).Where("scan_id = ? AND status IN ? AND severity IN ?", scan.ID, activeIssueStatuses(), []string{"critical", "high"}).Count(&item.HighPriority)
			stats.OpenIssues += item.OpenIssues
			stats.HighPriority += item.HighPriority
			sumScore += scan.Score
			scoredSites++
		}
		items = append(items, item)
	}
	if scoredSites > 0 {
		stats.AverageScore = int(float64(sumScore)/float64(scoredSites) + 0.5)
	}
	var scans []Scan
	a.db.Order("id desc").Limit(8).Find(&scans)
	names := make(map[uint]string, len(sites))
	for _, site := range sites {
		names[site.ID] = site.Name
	}
	recent := make([]dashboardRecentScan, 0, len(scans))
	for _, scan := range scans {
		recent = append(recent, dashboardRecentScan{Scan: scan, SiteName: names[scan.SiteID]})
	}
	c.JSON(http.StatusOK, gin.H{"stats": stats, "sites": items, "recentScans": recent})
}

func latestScan(db *gorm.DB, siteID uint, status string) *Scan {
	query := db.Where("site_id = ?", siteID)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var scan Scan
	if err := query.Order("id desc").First(&scan).Error; err != nil {
		return nil
	}
	return &scan
}

func activeIssueStatuses() []string {
	return []string{IssueStatusOpen, IssueStatusInProgress}
}

func (a *API) handleCreateScan(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var site Site
	if err := a.db.First(&site, id).Error; err != nil {
		abortError(c, http.StatusNotFound, "RESOURCE_NOT_FOUND", "网站不存在")
		return
	}
	var running int64
	a.db.Model(&Scan{}).Where("site_id = ? AND status = ? AND started_at > ?", site.ID, ScanStatusRunning, time.Now().Add(-5*time.Minute)).Count(&running)
	if running > 0 {
		abortError(c, http.StatusConflict, "SCAN_IN_PROGRESS", "该网站已有扫描正在执行，请稍后重试")
		return
	}
	scan := Scan{
		SiteID:       site.ID,
		Status:       ScanStatusRunning,
		TargetURL:    site.URL,
		StartedAt:    time.Now(),
		PageCount:    0,
		RuleVersion:  RuleVersion,
		ScoreVersion: ScoreVersion,
	}
	if err := a.db.Create(&scan).Error; err != nil {
		abortError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "无法创建扫描任务")
		return
	}

	result := ScanWebsite(site.URL)
	finished := time.Now()
	scan.FinishedAt = &finished
	scan.Status = result.Status
	if scan.Status == "" {
		scan.Status = ScanStatusFailed
	}
	scan.Score = result.Score
	scan.Summary = result.Summary
	scan.ErrorCode = result.ErrorCode
	scan.ErrorMessage = result.ErrorMessage
	scan.FinalURL = result.FinalURL
	scan.HTTPStatus = result.HTTPStatus
	if scan.Status == ScanStatusCompleted {
		scan.PageCount = 1
	}

	var persisted []Finding
	err := a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&scan).Error; err != nil {
			return err
		}
		var err error
		persisted, err = persistFindings(tx, site, scan.ID, result.Findings, finished)
		return err
	})
	if err != nil {
		a.db.Model(&scan).Updates(map[string]any{
			"status":        ScanStatusFailed,
			"finished_at":   finished,
			"error_code":    "PERSIST_FAILED",
			"error_message": "扫描结果保存失败",
		})
		abortError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "扫描已完成，但结果保存失败")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"scan": scan, "findings": persisted})
}

func persistFindings(tx *gorm.DB, site Site, scanID uint, findings []RuleFinding, now time.Time) ([]Finding, error) {
	persisted := make([]Finding, 0, len(findings))
	for _, item := range findings {
		fingerprint := fingerprintFor(site.ID, site.URL, item.RuleCode)
		status := IssueStatusOpen
		firstSeen := now
		reopened := false
		var previous Finding
		err := tx.Where("site_id = ? AND fingerprint = ?", site.ID, fingerprint).Order("id desc").First(&previous).Error
		if err == nil {
			status = previous.Status
			if status == "" {
				status = IssueStatusOpen
			}
			if !previous.FirstSeenAt.IsZero() {
				firstSeen = previous.FirstSeenAt
			} else if !previous.CreatedAt.IsZero() {
				firstSeen = previous.CreatedAt
			}
			if status == IssueStatusResolved {
				status = IssueStatusOpen
				reopened = true
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		finding := Finding{
			SiteID:         site.ID,
			ScanID:         scanID,
			Fingerprint:    fingerprint,
			Severity:       item.Severity,
			RuleCode:       item.RuleCode,
			Title:          item.Title,
			URL:            site.URL,
			Evidence:       item.Evidence,
			Recommendation: item.Recommendation,
			ScorePenalty:   item.ScorePenalty,
			Status:         status,
			FirstSeenAt:    firstSeen,
			LastSeenAt:     now,
		}
		if err := tx.Create(&finding).Error; err != nil {
			return nil, err
		}
		if reopened {
			if err := tx.Model(&Finding{}).Where("site_id = ? AND fingerprint = ? AND id <> ?", site.ID, fingerprint, finding.ID).Update("status", IssueStatusOpen).Error; err != nil {
				return nil, err
			}
			if err := tx.Create(&IssueEvent{
				SiteID:      site.ID,
				FindingID:   finding.ID,
				Fingerprint: fingerprint,
				RuleCode:    item.RuleCode,
				FromStatus:  IssueStatusResolved,
				ToStatus:    IssueStatusOpen,
				Note:        "重新扫描仍检测到该问题，已自动重新打开。",
			}).Error; err != nil {
				return nil, err
			}
		}
		persisted = append(persisted, finding)
	}
	return persisted, nil
}

func (a *API) handleSiteFindings(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var site Site
	if err := a.db.First(&site, id).Error; err != nil {
		abortError(c, http.StatusNotFound, "RESOURCE_NOT_FOUND", "网站不存在")
		return
	}
	var scanID uint
	if raw := c.Query("scanId"); raw != "" {
		parsed, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || parsed == 0 {
			abortError(c, http.StatusBadRequest, "VALIDATION_ERROR", "无效的扫描编号")
			return
		}
		var scan Scan
		if err := a.db.Where("id = ? AND site_id = ?", parsed, site.ID).First(&scan).Error; err != nil {
			abortError(c, http.StatusNotFound, "RESOURCE_NOT_FOUND", "扫描记录不存在")
			return
		}
		scanID = scan.ID
	} else if scan := latestScan(a.db, site.ID, ScanStatusCompleted); scan != nil {
		scanID = scan.ID
	} else {
		c.JSON(http.StatusOK, []Finding{})
		return
	}

	query := a.db.Where("site_id = ? AND scan_id = ?", site.ID, scanID)
	if severity := strings.TrimSpace(c.Query("severity")); severity != "" {
		if severity != "critical" && severity != "high" && severity != "medium" && severity != "low" {
			abortError(c, http.StatusBadRequest, "VALIDATION_ERROR", "无效的严重级别")
			return
		}
		query = query.Where("severity = ?", severity)
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		if !validIssueStatus(status) {
			abortError(c, http.StatusBadRequest, "VALIDATION_ERROR", "无效的问题状态")
			return
		}
		query = query.Where("status = ?", status)
	}
	if keyword := strings.TrimSpace(c.Query("q")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("(title LIKE ? OR rule_code LIKE ? OR evidence LIKE ?)", like, like, like)
	}
	var findings []Finding
	if err := query.Order("FIELD(severity, 'critical', 'high', 'medium', 'low')").Order("id asc").Find(&findings).Error; err != nil {
		abortError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "无法读取问题列表")
		return
	}
	c.JSON(http.StatusOK, findings)
}

type issueUpdateInput struct {
	Status string `json:"status"`
	Note   string `json:"note"`
}

func validIssueStatus(status string) bool {
	switch status {
	case IssueStatusOpen, IssueStatusInProgress, IssueStatusResolved, IssueStatusIgnored:
		return true
	default:
		return false
	}
}

func (a *API) handleUpdateIssue(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var input issueUpdateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		abortError(c, http.StatusBadRequest, "VALIDATION_ERROR", "请求内容不是合法的 JSON")
		return
	}
	input.Status = strings.TrimSpace(input.Status)
	input.Note = strings.TrimSpace(input.Note)
	if !validIssueStatus(input.Status) {
		abortError(c, http.StatusBadRequest, "VALIDATION_ERROR", "状态必须为 open、in_progress、resolved 或 ignored")
		return
	}
	if len([]rune(input.Note)) > 1000 {
		abortError(c, http.StatusBadRequest, "VALIDATION_ERROR", "备注不能超过 1000 个字符")
		return
	}
	var finding Finding
	if err := a.db.First(&finding, id).Error; err != nil {
		abortError(c, http.StatusNotFound, "RESOURCE_NOT_FOUND", "问题记录不存在")
		return
	}
	if finding.Status == input.Status {
		c.JSON(http.StatusOK, finding)
		return
	}
	from := finding.Status
	if from == "" {
		from = IssueStatusOpen
	}
	err := a.db.Transaction(func(tx *gorm.DB) error {
		query := tx.Model(&Finding{}).Where("site_id = ? AND fingerprint = ?", finding.SiteID, finding.Fingerprint)
		if finding.Fingerprint == "" {
			query = tx.Model(&Finding{}).Where("id = ?", finding.ID)
		}
		if err := query.Updates(map[string]any{"status": input.Status, "updated_at": time.Now()}).Error; err != nil {
			return err
		}
		return tx.Create(&IssueEvent{
			SiteID:      finding.SiteID,
			FindingID:   finding.ID,
			Fingerprint: finding.Fingerprint,
			RuleCode:    finding.RuleCode,
			FromStatus:  from,
			ToStatus:    input.Status,
			Note:        input.Note,
		}).Error
	})
	if err != nil {
		abortError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "更新问题状态失败")
		return
	}
	a.db.First(&finding, finding.ID)
	c.JSON(http.StatusOK, finding)
}

func (a *API) handleGetScan(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var scan Scan
	if err := a.db.First(&scan, id).Error; err != nil {
		abortError(c, http.StatusNotFound, "RESOURCE_NOT_FOUND", "扫描记录不存在")
		return
	}
	var site Site
	if err := a.db.First(&site, scan.SiteID).Error; err != nil {
		abortError(c, http.StatusNotFound, "RESOURCE_NOT_FOUND", "扫描关联的网站不存在")
		return
	}
	var findings []Finding
	if err := a.db.Where("scan_id = ?", scan.ID).Order("FIELD(severity, 'critical', 'high', 'medium', 'low')").Order("id asc").Find(&findings).Error; err != nil {
		abortError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "无法读取扫描问题")
		return
	}
	comparison := compareScanFindings(a.db, scan, findings)
	c.JSON(http.StatusOK, gin.H{
		"scan":           scan,
		"site":           site,
		"findings":       findings,
		"comparison":     comparison,
		"severityCounts": countBySeverity(findings),
		"statusCounts":   countByStatus(findings),
	})
}

func compareScanFindings(db *gorm.DB, scan Scan, current []Finding) scanComparison {
	comparison := scanComparison{New: []Finding{}, Resolved: []Finding{}}
	if scan.Status != ScanStatusCompleted {
		return comparison
	}
	var previous Scan
	err := db.Where("site_id = ? AND id < ? AND status = ?", scan.SiteID, scan.ID, ScanStatusCompleted).Order("id desc").First(&previous).Error
	if err != nil {
		comparison.New = append(comparison.New, current...)
		comparison.NewCount = len(current)
		return comparison
	}
	comparison.BaseScanID = previous.ID
	var previousFindings []Finding
	db.Where("scan_id = ?", previous.ID).Find(&previousFindings)
	currentByFingerprint := make(map[string]Finding, len(current))
	for _, finding := range current {
		currentByFingerprint[finding.Fingerprint] = finding
	}
	previousByFingerprint := make(map[string]Finding, len(previousFindings))
	for _, finding := range previousFindings {
		previousByFingerprint[finding.Fingerprint] = finding
		if _, exists := currentByFingerprint[finding.Fingerprint]; !exists {
			comparison.Resolved = append(comparison.Resolved, finding)
		}
	}
	for _, finding := range current {
		if _, exists := previousByFingerprint[finding.Fingerprint]; !exists {
			comparison.New = append(comparison.New, finding)
		}
	}
	comparison.NewCount = len(comparison.New)
	comparison.ResolvedCount = len(comparison.Resolved)
	comparison.PersistingCount = len(current) - comparison.NewCount
	return comparison
}

func countBySeverity(findings []Finding) map[string]int {
	counts := map[string]int{"critical": 0, "high": 0, "medium": 0, "low": 0}
	for _, finding := range findings {
		counts[finding.Severity]++
	}
	return counts
}

func countByStatus(findings []Finding) map[string]int {
	counts := map[string]int{IssueStatusOpen: 0, IssueStatusInProgress: 0, IssueStatusResolved: 0, IssueStatusIgnored: 0}
	for _, finding := range findings {
		counts[finding.Status]++
	}
	return counts
}

func (a *API) loadReport(c *gin.Context) (Site, Scan, []Finding, bool) {
	id, ok := parseID(c)
	if !ok {
		return Site{}, Scan{}, nil, false
	}
	var scan Scan
	if err := a.db.First(&scan, id).Error; err != nil {
		abortError(c, http.StatusNotFound, "RESOURCE_NOT_FOUND", "扫描记录不存在")
		return Site{}, Scan{}, nil, false
	}
	var site Site
	if err := a.db.First(&site, scan.SiteID).Error; err != nil {
		abortError(c, http.StatusNotFound, "RESOURCE_NOT_FOUND", "扫描关联的网站不存在")
		return Site{}, Scan{}, nil, false
	}
	var findings []Finding
	if err := a.db.Where("scan_id = ?", scan.ID).Order("FIELD(severity, 'critical', 'high', 'medium', 'low')").Order("id asc").Find(&findings).Error; err != nil {
		abortError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "无法读取扫描问题")
		return Site{}, Scan{}, nil, false
	}
	return site, scan, findings, true
}

func (a *API) handleHTMLReport(c *gin.Context) {
	site, scan, findings, ok := a.loadReport(c)
	if !ok {
		return
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Header("Cache-Control", "no-store")
	c.String(http.StatusOK, renderReport(site, scan, findings))
}

func (a *API) handleJSONReport(c *gin.Context) {
	site, scan, findings, ok := a.loadReport(c)
	if !ok {
		return
	}
	document := buildReportDocument(site, scan, findings)
	payload, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		abortError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "报告序列化失败")
		return
	}
	c.Header("Content-Type", "application/json; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"geopilot-scan-%d.json\"", scan.ID))
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "application/json; charset=utf-8", payload)
}

func failInterruptedScans(db *gorm.DB) {
	now := time.Now()
	result := db.Model(&Scan{}).Where("status = ?", ScanStatusRunning).Updates(map[string]any{
		"status":        ScanStatusFailed,
		"finished_at":   now,
		"error_code":    "INTERRUPTED",
		"error_message": "服务重启前扫描仍在执行，已标记为失败，请重新运行诊断。",
	})
	if result.Error != nil {
		log.Printf("mark interrupted scans: %v", result.Error)
	}
}

func backfillData(db *gorm.DB) {
	var scans []Scan
	db.Where("target_url = '' OR rule_version = '' OR score_version = ''").Find(&scans)
	siteURLs := map[uint]string{}
	for _, scan := range scans {
		updates := map[string]any{"rule_version": RuleVersion, "score_version": ScoreVersion}
		if scan.TargetURL == "" {
			if _, exists := siteURLs[scan.SiteID]; !exists {
				var site Site
				if db.First(&site, scan.SiteID).Error == nil {
					siteURLs[scan.SiteID] = site.URL
				}
			}
			if value := siteURLs[scan.SiteID]; value != "" {
				updates["target_url"] = value
			}
		}
		db.Model(&Scan{}).Where("id = ?", scan.ID).Updates(updates)
	}

	var findings []Finding
	db.Where("fingerprint = '' OR first_seen_at IS NULL OR last_seen_at IS NULL OR score_penalty = 0").Find(&findings)
	for _, finding := range findings {
		first := finding.FirstSeenAt
		if first.IsZero() {
			first = finding.CreatedAt
		}
		last := finding.LastSeenAt
		if last.IsZero() {
			last = finding.CreatedAt
		}
		status := finding.Status
		if status == "" {
			status = IssueStatusOpen
		}
		penalty := finding.ScorePenalty
		if penalty == 0 {
			penalty = penaltyFor(finding.Severity)
		}
		db.Model(&Finding{}).Where("id = ?", finding.ID).Updates(map[string]any{
			"fingerprint":   fingerprintFor(finding.SiteID, finding.URL, finding.RuleCode),
			"first_seen_at": first,
			"last_seen_at":  last,
			"status":        status,
			"score_penalty": penalty,
		})
	}
}
