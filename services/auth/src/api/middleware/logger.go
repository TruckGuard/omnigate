package middleware

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		c.Next()

		end := time.Now()
		latency := end.Sub(start)
		clientIP := c.ClientIP()
		method := c.Request.Method
		statusCode := c.Writer.Status()
		errorMessage := c.Errors.ByType(gin.ErrorTypePrivate).String()

		if raw != "" {
			path = path + "?" + raw
		}

		attrs := []any{
			slog.Int("status", statusCode),
			slog.String("method", method),
			slog.String("path", path),
			slog.String("ip", clientIP),
			slog.Duration("latency", latency),
			slog.String("user_agent", c.Request.UserAgent()),
		}

		if errorMessage != "" {
			attrs = append(attrs, slog.String("error", errorMessage))
		}

		msg := fmt.Sprintf("HTTP Request: %s", path)
		if statusCode >= 500 {
			slog.Error(msg, attrs...)
		} else if statusCode >= 400 {
			slog.Warn(msg, attrs...)
		} else {
			slog.Info(msg, attrs...)
		}
	}
}
