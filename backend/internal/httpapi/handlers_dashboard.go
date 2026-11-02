package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (a *API) handleDashboard(c *gin.Context) {
	data, err := a.service.Dashboard(c.Request.Context())
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, data)
}
