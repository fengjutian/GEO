package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/fengjutian/GEO/backend/internal/service"
)

type siteInput struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	Description string `json:"description"`
}

func (a *API) handleListSites(c *gin.Context) {
	sites, err := a.service.ListSites(c.Request.Context())
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, sites)
}

func (a *API) handleCreateSite(c *gin.Context) {
	var input siteInput
	if err := c.ShouldBindJSON(&input); err != nil {
		abortError(c, http.StatusBadRequest, "VALIDATION_ERROR", "请求内容不是合法的 JSON")
		return
	}
	site, err := a.service.CreateSite(c.Request.Context(), service.SiteInput{
		Name:        input.Name,
		URL:         input.URL,
		Description: input.Description,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, site)
}

func (a *API) handleGetSite(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	detail, err := a.service.GetSiteDetail(c.Request.Context(), id)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (a *API) handleUpdateSite(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var input siteInput
	if err := c.ShouldBindJSON(&input); err != nil {
		abortError(c, http.StatusBadRequest, "VALIDATION_ERROR", "请求内容不是合法的 JSON")
		return
	}
	site, err := a.service.UpdateSite(c.Request.Context(), id, service.SiteInput{
		Name:        input.Name,
		URL:         input.URL,
		Description: input.Description,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, site)
}

func (a *API) handleDeleteSite(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := a.service.DeleteSite(c.Request.Context(), id); err != nil {
		writeServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
