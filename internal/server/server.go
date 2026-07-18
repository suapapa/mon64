package server

import (
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/suapapa/mon64/internal/exporter"
	"github.com/suapapa/mon64/internal/metrics"
	"github.com/suapapa/mon64/internal/store"
	"github.com/suapapa/mon64/web"
)

// Server serves HTTP API and dashboard via Gin.
type Server struct {
	store   *store.Store
	engine  *gin.Engine
	metrics *metrics.Registry
	indexT  *template.Template
}

// New builds Gin route handlers.
func New(st *store.Store, log *slog.Logger, reg *metrics.Registry) (*Server, error) {
	if reg == nil {
		reg = metrics.Global
	}
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(recoveryLogger(log))
	r.Use(requestLogger(log))
	r.Use(metricsMiddleware(reg))

	static, err := fs.Sub(web.FS, "static")
	if err != nil {
		return nil, err
	}

	funcs := template.FuncMap{
		"deref": func(p *float64) float64 {
			if p == nil {
				return 0
			}
			return *p
		},
	}
	indexT, err := template.New("index.html").Funcs(funcs).ParseFS(web.FS, "index.html")
	if err != nil {
		return nil, err
	}

	s := &Server{store: st, engine: r, metrics: reg, indexT: indexT}
	r.GET("/healthz", s.handleHealthz)
	r.GET("/metrics", s.handleMetrics)
	r.GET("/api/v1/nodes", s.handleNodesJSON)
	r.GET("/api/v1/nodes.yaml", s.handleNodesYAML)
	r.GET("/api/v1/events", s.handleEvents)
	r.GET("/api/v1/badge", s.handleBadgeStack)
	r.GET("/api/v1/badge/:name", s.handleBadge)
	r.GET("/", s.handleIndex)
	r.StaticFS("/static", http.FS(static))

	return s, nil
}

// Engine returns the Gin engine (for tests and http.Server).
func (s *Server) Engine() *gin.Engine {
	return s.engine
}

func (s *Server) handleHealthz(c *gin.Context) {
	c.String(http.StatusOK, "ok")
}

func (s *Server) handleMetrics(c *gin.Context) {
	stats := s.store.ScrapeStats()
	c.Header("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	s.metrics.WritePrometheus(c.Writer, stats)
}

func (s *Server) handleNodesJSON(c *gin.Context) {
	snap := s.store.Snapshot()
	data, err := exporter.JSON(snap)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", data)
}

func (s *Server) handleNodesYAML(c *gin.Context) {
	snap := s.store.Snapshot()
	data, err := exporter.YAML(snap)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.Data(http.StatusOK, "application/yaml; charset=utf-8", data)
}

func (s *Server) handleBadge(c *gin.Context) {
	name := strings.TrimSuffix(c.Param("name"), ".png")
	if name == "" {
		c.Status(http.StatusNotFound)
		return
	}
	node, ok := s.store.NodeByName(name)
	if !ok {
		c.Status(http.StatusNotFound)
		return
	}
	data, err := exporter.BadgePNG(node)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.Data(http.StatusOK, "image/png", data)
}

func (s *Server) handleBadgeStack(c *gin.Context) {
	data, err := exporter.BadgeStackPNG(s.store.Snapshot().Nodes)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.Data(http.StatusOK, "image/png", data)
}

func (s *Server) handleIndex(c *gin.Context) {
	snap := s.store.Snapshot()
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := s.indexT.Execute(c.Writer, snap); err != nil {
		c.String(http.StatusInternalServerError, err.Error())
	}
}

func (s *Server) handleEvents(c *gin.Context) {
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.String(http.StatusInternalServerError, "streaming unsupported")
		return
	}

	events, unsubscribe := s.store.Subscribe()
	defer unsubscribe()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	fmt.Fprintf(c.Writer, ": connected\n\n")
	flusher.Flush()

	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-events:
			fmt.Fprintf(c.Writer, "event: update\ndata: {}\n\n")
			flusher.Flush()
		case <-heartbeat.C:
			fmt.Fprintf(c.Writer, ": heartbeat\n\n")
			flusher.Flush()
		}
	}
}
