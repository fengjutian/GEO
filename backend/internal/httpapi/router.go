package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/fengjutian/GEO/backend/internal/service"
)

type API struct {
	service *service.Service
}

func NewRouter(svc *service.Service, allowedOrigins map[string]bool) *gin.Engine {
	api := &API{service: svc}
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

func (a *API) handleHealth(c *gin.Context) {
	if err := a.service.Health(c.Request.Context()); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "error", "database": "unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "database": "ok"})
}
