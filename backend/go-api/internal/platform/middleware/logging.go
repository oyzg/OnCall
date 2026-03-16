package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oyzg/OnCall/backend/go-api/pkg/logger"
	"github.com/oyzg/OnCall/backend/go-api/pkg/utils"
)

func RequestLogger(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		log.Info("http request",
			"request_id", utils.RequestIDFromContext(c.Request.Context()),
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration", time.Since(start),
		)
	}
}
