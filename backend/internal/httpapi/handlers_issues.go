package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type issueUpdateInput struct {
	Status string `json:"status"`
	Note   string `json:"note"`
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
	finding, err := a.service.UpdateIssue(c.Request.Context(), id, input.Status, input.Note)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, finding)
}
