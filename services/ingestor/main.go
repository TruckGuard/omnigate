package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/omnigate/services/ingestor/src/api/handlers"
	"github.com/omnigate/services/ingestor/src/api/middleware"
	"github.com/omnigate/services/ingestor/src/pkg/telemetry"
	"github.com/omnigate/services/ingestor/src/repository"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

func main() {
	logger := telemetry.NewLogger("omnigate-ingestor")
	slog.SetDefault(logger)

	if err := telemetry.Init("omnigate-ingestor"); err != nil {
		logger.Error("otel init failed", "error", err)
		os.Exit(1)
	}
	defer telemetry.Shutdown(context.Background())

	// Initialize Valkey
	valkeyAddr := os.Getenv("VALKEY_ADDR")
	if valkeyAddr == "" {
		valkeyAddr = os.Getenv("REDIS_ADDR")
	}
	repository.InitRedis(valkeyAddr)

	// Initialize Garage/MinIO storage
	repository.InitMinio(
		os.Getenv("STORAGE_ENDPOINT"),
		os.Getenv("STORAGE_ACCESS_KEY"),
		os.Getenv("STORAGE_SECRET_KEY"),
	)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.Logger())
	r.Use(otelgin.Middleware("omnigate-ingestor"))

	// Health check
	r.Match([]string{"GET", "HEAD"}, "/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Ingestion endpoint
	r.POST("/ingest/event", handlers.HandleIngest)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{Addr: ":" + port, Handler: r}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
	}
}
