package middlewares

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/linenxing/e-commerce-system/base/logger"
	"github.com/rs/zerolog"
)

func RequestLogger(baseLogger zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		startedAt := time.Now()
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.NewString()
		}

		requestLogger := baseLogger.With().
			Str("request_id", requestID).
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Logger()

		c.Header("X-Request-ID", requestID)
		c.Request = c.Request.WithContext(logger.WithContext(c.Request.Context(), requestLogger))
		c.Next()

		requestLogger.Info().
			Int("status", c.Writer.Status()).
			Int64("duration_ms", time.Since(startedAt).Milliseconds()).
			Msg("http request completed")
	}
}
