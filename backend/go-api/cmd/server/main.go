package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/oyzg/OnCall/backend/go-api/internal/platform/httpserver"
	"github.com/oyzg/OnCall/backend/go-api/pkg/config"
	"github.com/oyzg/OnCall/backend/go-api/pkg/logger"
)

func main() {
	cfg := config.Load()
	appLogger := logger.New(cfg.App.LogLevel)

	if cfg.App.Env == "prod" {
		gin.SetMode(gin.ReleaseMode)
	}

	server := &http.Server{
		Addr:         cfg.HTTPAddress(),
		Handler:      httpserver.New(cfg, appLogger),
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
		IdleTimeout:  cfg.HTTP.IdleTimeout,
	}

	appLogger.Info("go api starting",
		"addr", cfg.HTTPAddress(),
		"env", cfg.App.Env,
	)

	log.Fatal(server.ListenAndServe())
}
