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
	"github.com/omnigate/services/auth/src/api/handlers"
	"github.com/omnigate/services/auth/src/api/middleware"
	"github.com/omnigate/services/auth/src/pkg/telemetry"
	"github.com/omnigate/services/auth/src/repository"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

func main() {
	logger := telemetry.NewLogger("omnigate-auth")
	slog.SetDefault(logger)

	if err := telemetry.Init("omnigate-auth"); err != nil {
		logger.Error("otel init failed", "error", err)
		os.Exit(1)
	}
	defer telemetry.Shutdown(context.Background())

	repository.InitDB(os.Getenv("DATABASE_URL"))
	valkeyAddr := os.Getenv("VALKEY_ADDR")
	if valkeyAddr == "" {
		valkeyAddr = os.Getenv("REDIS_ADDR")
	}
	repository.InitRedis(valkeyAddr)

	seedData()

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.Logger())
	r.Use(otelgin.Middleware("omnigate-auth"))

	r.POST("/login", handlers.HandleLogin)
	r.GET("/validate", handlers.HandleValidate)
	r.POST("/logout", handlers.HandleLogout)
	r.GET("/sessions", handlers.HandleListSessions)
	r.POST("/sessions/revoke", handlers.HandleRevokeSession)
	r.POST("/sessions/revoke-all", handlers.HandleRevokeAllSessions)
	r.Match([]string{"GET", "HEAD"}, "/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })

	r.POST("/register", handlers.HandleRegister)
	r.GET("/hierarchy", handlers.HandleGetPermissionHierarchy)
	r.POST("/change-password", handlers.HandleChangePassword)

	admin := r.Group("/admin")
	{
		// Users
		admin.GET("/users", handlers.HandleListUsers)
		admin.GET("/users/:id", handlers.HandleGetUser)
		admin.PUT("/users/:id/role", middleware.RequirePermission("manage:users"), handlers.HandleUpdateUserRole)
		admin.DELETE("/users/:id", middleware.RequirePermission("manage:users"), handlers.HandleDeleteUser)
		admin.POST("/users/:id/reset-password", middleware.RequirePermission("manage:users"), handlers.HandleAdminResetPassword)

		// Roles
		admin.GET("/roles", middleware.RequirePermission("read:roles"), handlers.HandleListRoles)
		admin.POST("/roles", middleware.RequirePermission("manage:roles"), handlers.HandleCreateRole)
		admin.PUT("/roles/:id", middleware.RequirePermission("manage:roles"), handlers.HandleUpdateRole)
		admin.DELETE("/roles/:id", middleware.RequirePermission("manage:roles"), handlers.HandleDeleteRole)
		admin.POST("/roles/:id/permissions", middleware.RequirePermission("manage:roles"), handlers.HandleAssignPermissionsToRole)

		// API Keys (IoT)
		admin.GET("/keys", middleware.RequirePermission("read:keys"), handlers.HandleListKeys)
		admin.POST("/keys", middleware.RequirePermission("manage:keys"), handlers.HandleCreateKeyWithPerms)
		admin.DELETE("/keys/:id", middleware.RequirePermission("manage:keys"), handlers.HandleDeleteKey)
		admin.PUT("/keys/:id/permissions", middleware.RequirePermission("manage:keys"), handlers.HandleAssignPermissionsToKey)
		admin.PUT("/keys/:id", middleware.RequirePermission("manage:keys"), handlers.HandleUpdateKey)

		admin.GET("/permissions", handlers.HandleListPermissions)
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
