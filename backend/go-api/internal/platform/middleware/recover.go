package middleware

import (
	"github.com/gin-gonic/gin"
	appErrors "github.com/oyzg/OnCall/backend/go-api/pkg/errors"
	"github.com/oyzg/OnCall/backend/go-api/pkg/logger"
	"github.com/oyzg/OnCall/backend/go-api/pkg/response"
	"github.com/oyzg/OnCall/backend/go-api/pkg/utils"
)

func Recover(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				requestID := utils.RequestIDFromContext(c.Request.Context())
				log.Error("request panic",
					"panic", rec,
					"request_id", requestID,
					"path", c.Request.URL.Path,
				)
				response.Failure(c.Writer, appErrors.ErrInternal.HTTPStatus, requestID, appErrors.ErrInternal.Code, appErrors.ErrInternal.Message)
				c.Abort()
			}
		}()

		c.Next()
	}
}
