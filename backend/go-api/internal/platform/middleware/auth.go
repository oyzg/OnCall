package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/oyzg/OnCall/backend/go-api/internal/auth/api"
	"github.com/oyzg/OnCall/backend/go-api/internal/auth/application"
	appErrors "github.com/oyzg/OnCall/backend/go-api/pkg/errors"
	"github.com/oyzg/OnCall/backend/go-api/pkg/response"
	"github.com/oyzg/OnCall/backend/go-api/pkg/utils"
)

func Auth(authService *application.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			writeUnauthorized(c)
			return
		}

		token, ok := strings.CutPrefix(authHeader, "Bearer ")
		if !ok || strings.TrimSpace(token) == "" {
			writeUnauthorized(c)
			return
		}

		user, err := authService.ParseUser(strings.TrimSpace(token))
		if err != nil {
			writeUnauthorized(c)
			return
		}

		api.SetCurrentUser(c, user)
		c.Next()
	}
}

func writeUnauthorized(c *gin.Context) {
	requestID := utils.RequestIDFromContext(c.Request.Context())
	response.Failure(c.Writer, appErrors.ErrUnauthorized.HTTPStatus, requestID, appErrors.ErrUnauthorized.Code, appErrors.ErrUnauthorized.Message)
	c.Abort()
}
