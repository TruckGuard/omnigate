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
	"github.com/omnigate/services/core/src/api/handlers"
	"github.com/omnigate/services/core/src/api/middleware"
	"github.com/omnigate/services/core/src/pkg/telemetry"
	"github.com/omnigate/services/core/src/repository"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

func main() {
	logger := telemetry.NewLogger("omnigate-core")
	slog.SetDefault(logger)

	if err := telemetry.Init("omnigate-core"); err != nil {
		logger.Error("otel init failed", "error", err)
		os.Exit(1)
	}
	defer telemetry.Shutdown(context.Background())

	repository.InitDB(os.Getenv("DATABASE_URL"))

	if ep := os.Getenv("STORAGE_ENDPOINT"); ep != "" {
		bucket := os.Getenv("STORAGE_BUCKET")
		if bucket == "" {
			bucket = "omnigate-data"
		}
		repository.InitStorage(ep, os.Getenv("STORAGE_ACCESS_KEY"), os.Getenv("STORAGE_SECRET_KEY"), bucket)
	}

	valkeyAddr := os.Getenv("VALKEY_ADDR")
	if valkeyAddr == "" {
		valkeyAddr = os.Getenv("REDIS_ADDR")
	}
	repository.InitRedis(valkeyAddr)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.Logger())
	r.Use(otelgin.Middleware("omnigate-core"))

	r.Match([]string{"GET", "HEAD"}, "/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	api := r.Group("/api/v1")
	{
		// Events
		api.POST("/events", handlers.HandleCreateEvent)
		api.GET("/events", handlers.HandleListEvents)
		api.GET("/events/latest", handlers.HandleGetLatestEventForSource)
		api.GET("/events/:id", handlers.HandleGetEvent)
		api.GET("/events/:id/raw", handlers.HandleGetEventRaw)
		api.DELETE("/events/:id", handlers.HandleDeleteEvent)

		// Transactions
		api.GET("/transactions", handlers.HandleListTransactions)
		api.GET("/transactions/history", handlers.HandleVehicleHistory)
		api.GET("/transactions/:id", handlers.HandleGetTransaction)
		api.POST("/transactions", handlers.HandleCreateTransaction)
		api.POST("/transactions/:id/close", handlers.HandleCloseTransaction)
		api.PUT("/transactions/:id", handlers.HandleUpdateTransaction)
		api.DELETE("/transactions/:id", handlers.HandleDeleteTransaction)

		// Device Configs
		api.GET("/configs/devices", handlers.HandleListDeviceConfigs)
		api.GET("/configs/devices/:source_id", handlers.HandleGetDeviceConfig)
		api.POST("/configs/devices", handlers.HandleCreateDeviceConfig)
		api.PUT("/configs/devices/:id", handlers.HandleUpdateDeviceConfig)
		api.POST("/configs/devices/:id/trigger", handlers.HandleTriggerDevice)
		api.DELETE("/configs/devices/:id", handlers.HandleDeleteDeviceConfig)

		// Event Types
		api.GET("/types", handlers.HandleListEventTypes)
		api.GET("/types/:id", handlers.HandleGetEventType)
		api.POST("/types", handlers.HandleCreateEventType)
		api.PUT("/types/:id", handlers.HandleUpdateEventType)
		api.DELETE("/types/:id", handlers.HandleDeleteEventType)

		// Gates
		api.GET("/gates", handlers.HandleListGates)
		api.GET("/gates/:id", handlers.HandleGetGate)
		api.POST("/gates", handlers.HandleCreateGate)
		api.PUT("/gates/:id", handlers.HandleUpdateGate)
		api.DELETE("/gates/:id", handlers.HandleDeleteGate)
		api.PUT("/gates/:id/settings", handlers.HandleUpdateGateSettings)
		api.GET("/gates/:id/stats", handlers.HandleGetGateStats)

		// User Profiles (?auth_id=<uint> on GET /profiles for lookup by auth ID)
		api.GET("/profiles", handlers.HandleListUserProfiles)
		api.GET("/profiles/:id", handlers.HandleGetUserProfile)
		api.POST("/profiles", handlers.HandleCreateUserProfile)
		api.PUT("/profiles/:id", handlers.HandleUpdateUserProfile)
		api.DELETE("/profiles/:id", handlers.HandleDeleteUserProfile)
	}

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
