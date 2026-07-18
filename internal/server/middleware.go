package server

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/suapapa/mon64/internal/metrics"
)

func requestLogger(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		route := c.FullPath()
		if route == "" {
			route = c.Request.URL.Path
		}
		log.Info("http request",
			"method", c.Request.Method,
			"path", route,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
			"client", c.ClientIP(),
		)
	}
}

func metricsMiddleware(reg *metrics.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		route := c.FullPath()
		if route == "" {
			route = c.Request.URL.Path
		}
		reg.ObserveHTTP(c.Request.Method, route, strconv.Itoa(c.Writer.Status()))
	}
}

func recoveryLogger(log *slog.Logger) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, err any) {
		log.Error("panic recovered", "err", err, "path", c.Request.URL.Path)
		c.AbortWithStatus(http.StatusInternalServerError)
	})
}
