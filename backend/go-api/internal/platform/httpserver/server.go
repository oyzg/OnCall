package httpserver

import (
	"github.com/gin-gonic/gin"
	authAPI "github.com/oyzg/OnCall/backend/go-api/internal/auth/api"
	authApp "github.com/oyzg/OnCall/backend/go-api/internal/auth/application"
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
		middleware.CORS(),
		middleware.RequestID(),
		middleware.Recover(log),
		middleware.RequestLogger(log),
	)
	registerRoutes(router, cfg)
	return router
}

func registerRoutes(router *gin.Engine, cfg config.Config) {
	authService := authApp.NewService(cfg.Auth)
	authHandler := authAPI.NewHandler(authService)

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

	authGroup := router.Group("/api/v1/auth")
	authGroup.POST("/login", authHandler.Login)
	authGroup.GET("/me", middleware.Auth(authService), authHandler.Me)
}
