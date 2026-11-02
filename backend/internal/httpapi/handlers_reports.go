package httpapi

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (a *API) handleHTMLReport(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	document, err := a.service.HTMLReport(c.Request.Context(), id)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Header("Cache-Control", "no-store")
	c.String(http.StatusOK, document)
}

func (a *API) handleJSONReport(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	payload, err := a.service.JSONReport(c.Request.Context(), id)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.Header("Content-Type", "application/json; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"geopilot-scan-%d.json\"", id))
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "application/json; charset=utf-8", payload)
}
