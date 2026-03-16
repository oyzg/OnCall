package middleware

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gin-gonic/gin"
	"github.com/oyzg/OnCall/backend/go-api/pkg/utils"
)

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-Id")
		if requestID == "" {
			requestID = randomID()
		}

		ctx := utils.WithRequestID(c.Request.Context(), requestID)
		c.Request = c.Request.WithContext(ctx)
		c.Writer.Header().Set("X-Request-Id", requestID)
		c.Next()
	}
}

func randomID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "request-id-fallback"
	}
	return hex.EncodeToString(buf)
}
