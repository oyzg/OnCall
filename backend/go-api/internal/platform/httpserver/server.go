package httpserver

import (
	"github.com/gin-gonic/gin"
	retrievalApp "github.com/oyzg/OnCall/backend/go-api/internal/ai/retrieval"
	retrievalAPI "github.com/oyzg/OnCall/backend/go-api/internal/ai/retrieval/api"
	alertAPI "github.com/oyzg/OnCall/backend/go-api/internal/alert/api"
	alertApp "github.com/oyzg/OnCall/backend/go-api/internal/alert/application"
	authAPI "github.com/oyzg/OnCall/backend/go-api/internal/auth/api"
	authApp "github.com/oyzg/OnCall/backend/go-api/internal/auth/application"
	knowledgeAPI "github.com/oyzg/OnCall/backend/go-api/internal/knowledge/api"
	knowledgeApp "github.com/oyzg/OnCall/backend/go-api/internal/knowledge/application"
	"github.com/oyzg/OnCall/backend/go-api/internal/platform/middleware"
	"github.com/oyzg/OnCall/backend/go-api/internal/platform/observability"
	sessionAPI "github.com/oyzg/OnCall/backend/go-api/internal/session/api"
	sessionApp "github.com/oyzg/OnCall/backend/go-api/internal/session/application"
	toolAPI "github.com/oyzg/OnCall/backend/go-api/internal/tool/api"
	toolApp "github.com/oyzg/OnCall/backend/go-api/internal/tool/application"
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
	knowledgeService := knowledgeApp.NewService()
	knowledgeHandler := knowledgeAPI.NewHandler(knowledgeService)
	retrievalService := retrievalApp.NewService(knowledgeService)
	retrievalHandler := retrievalAPI.NewHandler(retrievalService)
	sessionService := sessionApp.NewService()
	sessionHandler := sessionAPI.NewHandler(sessionService, retrievalService)
	alertService := alertApp.NewService(sessionService)
	alertService.EnsureSeeded()
	alertHandler := alertAPI.NewHandler(alertService)
	toolService := toolApp.NewService(alertService, retrievalService, sessionService, knowledgeService)
	toolHandler := toolAPI.NewHandler(toolService)

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

	sessionGroup := router.Group("/api/v1/sessions")
	sessionGroup.Use(middleware.Auth(authService))
	sessionGroup.GET("", sessionHandler.ListSessions)
	sessionGroup.POST("", sessionHandler.CreateSession)
	sessionGroup.GET("/:sessionID/messages", sessionHandler.ListMessages)
	sessionGroup.POST("/:sessionID/messages/stream", sessionHandler.StreamMessage)
	sessionGroup.DELETE("/:sessionID", sessionHandler.DeleteSession)

	knowledgeGroup := router.Group("/api/v1/knowledge/documents")
	knowledgeGroup.Use(middleware.Auth(authService))
	knowledgeGroup.GET("", knowledgeHandler.ListDocuments)
	knowledgeGroup.POST("", knowledgeHandler.UploadDocument)
	knowledgeGroup.GET("/:documentID", knowledgeHandler.GetDocument)
	knowledgeGroup.DELETE("/:documentID", knowledgeHandler.DeleteDocument)
	knowledgeGroup.POST("/:documentID/reprocess", knowledgeHandler.RetryDocument)

	ragGroup := router.Group("/api/v1/rag")
	ragGroup.Use(middleware.Auth(authService))
	ragGroup.POST("/retrieve", retrievalHandler.Retrieve)

	router.POST("/api/v1/alerts/ingest", alertHandler.Ingest)

	alertGroup := router.Group("/api/v1/alerts")
	alertGroup.Use(middleware.Auth(authService))
	alertGroup.GET("", alertHandler.ListAlerts)
	alertGroup.GET("/:alertID", alertHandler.GetDetail)
	alertGroup.POST("/:alertID/status", alertHandler.UpdateStatus)
	alertGroup.POST("/:alertID/session", alertHandler.LinkSession)

	toolGroup := router.Group("/api/v1/tools")
	toolGroup.Use(middleware.Auth(authService))
	toolGroup.GET("", toolHandler.ListTools)
	toolGroup.GET("/logs", toolHandler.ListLogs)
	toolGroup.POST("/:toolName/call", toolHandler.CallTool)
}
