package httpserver

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	aiAnalyzer "github.com/oyzg/OnCall/backend/go-api/internal/ai/analyzer"
	"github.com/oyzg/OnCall/backend/go-api/internal/ai/eino"
	"github.com/oyzg/OnCall/backend/go-api/internal/ai/gateway"
	retrievalApp "github.com/oyzg/OnCall/backend/go-api/internal/ai/retrieval"
	retrievalAPI "github.com/oyzg/OnCall/backend/go-api/internal/ai/retrieval/api"
	alertAPI "github.com/oyzg/OnCall/backend/go-api/internal/alert/api"
	alertApp "github.com/oyzg/OnCall/backend/go-api/internal/alert/application"
	alertInfra "github.com/oyzg/OnCall/backend/go-api/internal/alert/infrastructure"
	auditAPI "github.com/oyzg/OnCall/backend/go-api/internal/audit/api"
	auditApp "github.com/oyzg/OnCall/backend/go-api/internal/audit/application"
	auditInfra "github.com/oyzg/OnCall/backend/go-api/internal/audit/infrastructure"
	authAPI "github.com/oyzg/OnCall/backend/go-api/internal/auth/api"
	authApp "github.com/oyzg/OnCall/backend/go-api/internal/auth/application"
	authInfra "github.com/oyzg/OnCall/backend/go-api/internal/auth/infrastructure"
	knowledgeAPI "github.com/oyzg/OnCall/backend/go-api/internal/knowledge/api"
	knowledgeApp "github.com/oyzg/OnCall/backend/go-api/internal/knowledge/application"
	knowledgeInfra "github.com/oyzg/OnCall/backend/go-api/internal/knowledge/infrastructure"
	"github.com/oyzg/OnCall/backend/go-api/internal/platform/db"
	"github.com/oyzg/OnCall/backend/go-api/internal/platform/middleware"
	"github.com/oyzg/OnCall/backend/go-api/internal/platform/observability"
	sessionAPI "github.com/oyzg/OnCall/backend/go-api/internal/session/api"
	sessionApp "github.com/oyzg/OnCall/backend/go-api/internal/session/application"
	sessionInfra "github.com/oyzg/OnCall/backend/go-api/internal/session/infrastructure"
	toolAPI "github.com/oyzg/OnCall/backend/go-api/internal/tool/api"
	toolApp "github.com/oyzg/OnCall/backend/go-api/internal/tool/application"
	toolInfra "github.com/oyzg/OnCall/backend/go-api/internal/tool/infrastructure"
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
	auditService := auditApp.NewService()
	aiClient := gateway.NewHTTPClient(cfg.AI)
	sessionService := sessionApp.NewService()
	knowledgeService := knowledgeApp.NewService()
	alertAnalyzer := aiAnalyzer.NewService(eino.NewStubOrchestrator(aiClient))
	alertService := alertApp.NewService(sessionService, alertAnalyzer)
	var toolRepo toolApp.Repository
	if cfg.MySQL.Enabled {
		gdb, err := db.Open(db.Config{
			Driver:      cfg.MySQL.Driver,
			DSN:         cfg.MySQL.DSN,
			PingTimeout: cfg.MySQL.PingTimeout,
		})
		if err != nil {
			panic(fmt.Errorf("open persistence db: %w", err))
		}
		if cfg.MySQL.AutoMigrate {
			if err := db.AutoMigrate(gdb); err != nil {
				panic(fmt.Errorf("auto migrate persistence db: %w", err))
			}
		}

		authService = authApp.NewServiceWithRepository(cfg.Auth, authInfra.NewMySQLRepository(gdb))
		if err := authService.EnsureSeeded(); err != nil {
			panic(fmt.Errorf("seed auth users: %w", err))
		}
		sessionService = sessionApp.NewServiceWithRepository(sessionInfra.NewMySQLRepository(gdb))
		knowledgeService = knowledgeApp.NewServiceWithRepository(knowledgeInfra.NewMySQLRepository(gdb))
		alertService = alertApp.NewServiceWithRepository(sessionService, alertAnalyzer, alertInfra.NewMySQLRepository(gdb))
		auditService = auditApp.NewServiceWithRepository(auditInfra.NewMySQLRepository(gdb))
		toolRepo = toolInfra.NewMySQLRepository(gdb)
	}
	authHandler := authAPI.NewHandler(authService, auditService)
	auditHandler := auditAPI.NewHandler(auditService)
	knowledgeService.SetIndexer(knowledgeIndexer{client: aiClient})
	knowledgeHandler := knowledgeAPI.NewHandler(knowledgeService, auditService)
	retrievalService := retrievalApp.NewService(knowledgeService)
	retrievalService.SetRemoteRetriever(aiClient)
	retrievalHandler := retrievalAPI.NewHandler(retrievalService)
	sessionHandler := sessionAPI.NewHandler(sessionService, retrievalService, auditService)
	alertService.EnsureSeeded()
	alertHandler := alertAPI.NewHandler(alertService, auditService)
	var toolService *toolApp.Service
	if toolRepo != nil {
		toolService = toolApp.NewServiceWithRepository(alertService, retrievalService, sessionService, knowledgeService, toolRepo)
	} else {
		toolService = toolApp.NewService(alertService, retrievalService, sessionService, knowledgeService)
	}
	toolHandler := toolAPI.NewHandler(toolService, auditService)

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
		report, err := aiClient.Health(c.Request.Context())
		if err == nil {
			response.Success(c.Writer, 200, utils.RequestIDFromContext(c.Request.Context()), report)
			return
		}

		response.Success(c.Writer, 200, utils.RequestIDFromContext(c.Request.Context()), map[string]any{
			"service": "python-ai",
			"status":  "unavailable",
			"detail":  err.Error(),
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
	alertGroup.GET("/stats", alertHandler.Stats)
	alertGroup.GET("/:alertID", alertHandler.GetDetail)
	alertGroup.POST("/:alertID/status", alertHandler.UpdateStatus)
	alertGroup.POST("/:alertID/session", alertHandler.LinkSession)
	alertGroup.POST("/:alertID/analyze", alertHandler.Analyze)

	toolGroup := router.Group("/api/v1/tools")
	toolGroup.Use(middleware.Auth(authService))
	toolGroup.GET("", toolHandler.ListTools)
	toolGroup.GET("/logs", toolHandler.ListLogs)
	toolGroup.POST("/:toolName/call", toolHandler.CallTool)

	auditGroup := router.Group("/api/v1/audit")
	auditGroup.Use(middleware.Auth(authService))
	auditGroup.GET("/logs", auditHandler.ListLogs)
	auditGroup.GET("/stats", auditHandler.Stats)
}

type knowledgeIndexer struct {
	client *gateway.HTTPClient
}

func (k knowledgeIndexer) IndexDocument(ctx context.Context, request knowledgeApp.IndexRequest) (knowledgeApp.IndexResult, error) {
	if k.client == nil {
		return knowledgeApp.IndexResult{}, nil
	}

	chunks := make([]gateway.RAGIndexChunk, 0, len(request.Chunks))
	for _, chunk := range request.Chunks {
		chunks = append(chunks, gateway.RAGIndexChunk{
			DocumentID:    chunk.DocumentID,
			DocumentTitle: chunk.DocumentTitle,
			Category:      chunk.Category,
			Index:         chunk.Index,
			Content:       chunk.Content,
		})
	}

	result, err := k.client.IndexKnowledge(ctx, gateway.RAGIndexRequest{
		UserID:        request.UserID,
		DocumentID:    request.DocumentID,
		DocumentTitle: request.DocumentTitle,
		Category:      request.Category,
		Chunks:        chunks,
	})
	if err != nil {
		return knowledgeApp.IndexResult{}, err
	}

	return knowledgeApp.IndexResult{
		Status:           result.Status,
		EmbeddingBackend: result.EmbeddingBackend,
		VectorBackend:    result.VectorBackend,
		LexicalBackend:   result.LexicalBackend,
	}, nil
}

func (k knowledgeIndexer) DeleteDocument(ctx context.Context, documentID string) error {
	if k.client == nil {
		return nil
	}

	_, err := k.client.DeleteKnowledge(ctx, gateway.RAGDeleteRequest{DocumentID: documentID})
	return err
}
