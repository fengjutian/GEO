package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/fengjutian/GEO/backend/internal/service"
)

func (a *API) handleCreateScan(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	scan, err := a.service.EnqueueScan(c.Request.Context(), id)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.Header("Location", "/api/scans/"+strconv.FormatUint(uint64(scan.ID), 10))
	c.JSON(http.StatusAccepted, gin.H{"scan": scan})
}

func (a *API) handleGetScan(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	detail, err := a.service.ScanDetail(c.Request.Context(), id)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (a *API) handleSiteFindings(c *gin.Context) {
	siteID, ok := parseID(c)
	if !ok {
		return
	}
	scanID := uint(0)
	if raw := c.Query("scanId"); raw != "" {
		parsed, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || parsed == 0 {
			abortError(c, http.StatusBadRequest, "VALIDATION_ERROR", "无效的扫描编号")
			return
		}
		scanID = uint(parsed)
	}
	filter := service.FindingFilter{Query: strings.TrimSpace(c.Query("q"))}
	if severity := strings.TrimSpace(c.Query("severity")); severity != "" {
		if severity != "critical" && severity != "high" && severity != "medium" && severity != "low" {
			abortError(c, http.StatusBadRequest, "VALIDATION_ERROR", "无效的严重级别")
			return
		}
		filter.Severity = severity
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		if !service.ValidIssueStatus(status) {
			abortError(c, http.StatusBadRequest, "VALIDATION_ERROR", "无效的问题状态")
			return
		}
		filter.Status = status
	}
	findings, err := a.service.SiteFindings(c.Request.Context(), siteID, scanID, filter)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, findings)
}
