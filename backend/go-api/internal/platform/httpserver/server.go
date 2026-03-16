package httpserver

import (
	"github.com/gin-gonic/gin"
	"github.com/oyzg/OnCall/backend/go-api/internal/platform/middleware"
	"github.com/oyzg/OnCall/backend/go-api/internal/platform/observability"
	"github.com/oyzg/OnCall/backend/go-api/pkg/config"
	"github.com/oyzg/OnCall/backend/go-api/pkg/logger"
	"github.com/oyzg/OnCall/backend/go-api/pkg/response"
	"github.com/oyzg/OnCall/backend/go-api/pkg/utils"
)

func New(cfg config.Config, log *logger.Logger) *gin.Engine {
	router := gin.New()
	router.Use(
		middleware.RequestID(),
		middleware.Recover(log),
		middleware.RequestLogger(log),
	)
	registerRoutes(router, cfg)
	return router
}

func registerRoutes(router *gin.Engine, cfg config.Config) {
	router.GET("/", func(c *gin.Context) {
		response.Success(c.Writer, 200, utils.RequestIDFromContext(c.Request.Context()), map[string]string{
			"message": "AI OnCall Go API base server is ready",
		})
	})

	router.GET("/healthz", func(c *gin.Context) {
		report := observability.BuildHealthReport(cfg)
		response.Success(c.Writer, 200, utils.RequestIDFromContext(c.Request.Context()), report)
	})

	router.GET("/proxy/python-ai/healthz", func(c *gin.Context) {
		report := observability.BuildHealthReport(cfg)
		for _, component := range report.Components {
			if component.Name == "python_ai_grpc" {
				response.Success(c.Writer, 200, utils.RequestIDFromContext(c.Request.Context()), map[string]any{
					"service": "python-ai",
					"status":  component.Status,
					"detail":  component.Error,
				})
				return
			}
		}

		response.Success(c.Writer, 200, utils.RequestIDFromContext(c.Request.Context()), map[string]any{
			"service": "python-ai",
			"status":  "unknown",
		})
	})
}
