package httpapi

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/fengjutian/GEO/backend/internal/service"
)

func abortError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{
		"code":    code,
		"message": message,
		"error":   message, // retained for older clients
	})
}

func writeServiceError(c *gin.Context, err error) {
	code := service.CodeOf(err)
	message := service.MessageOf(err)
	switch service.KindOf(err) {
	case service.KindValidation:
		abortError(c, http.StatusBadRequest, code, message)
	case service.KindNotFound:
		abortError(c, http.StatusNotFound, code, message)
	case service.KindConflict:
		abortError(c, http.StatusConflict, code, message)
	default:
		log.Printf("request failed: %v", err)
		abortError(c, http.StatusInternalServerError, code, message)
	}
}

func parseID(c *gin.Context) (uint, bool) {
	value, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || value == 0 {
		abortError(c, http.StatusBadRequest, "VALIDATION_ERROR", "无效的资源编号")
		return 0, false
	}
	return uint(value), true
}
