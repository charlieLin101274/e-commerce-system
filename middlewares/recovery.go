package middlewares

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/linenxing/e-commerce-system/base/logger"
	"github.com/linenxing/e-commerce-system/base/response"
)

func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		logger.FromContext(c.Request.Context()).
			Error().
			Interface("panic", recovered).
			Msg("panic recovered")
		response.Error(c, http.StatusInternalServerError, "internal_error", "an internal error occurred")
	})
}
