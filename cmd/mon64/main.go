package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/suapapa/mon64/internal/config"
	"github.com/suapapa/mon64/internal/export/pixoo"
	"github.com/suapapa/mon64/internal/export/prometheus"
	"github.com/suapapa/mon64/internal/metrics"
	"github.com/suapapa/mon64/internal/server"
	"github.com/suapapa/mon64/internal/store"
)

func main() {
	configPath := flag.String("config", "configs/example.yaml", "path to YAML config")
	dummy := flag.Bool("dummy", false, "generate dummy metrics without connecting to Prometheus endpoints")
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(log)

	loadConfig := config.Load
	if *dummy {
		loadConfig = config.LoadDummy
	}

	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Error("config load failed", "err", err)
		os.Exit(1)
	}

	reg := metrics.NewRegistry()
	st := store.New(cfg)
	if *dummy {
		st = store.NewDummy(cfg)
		log.Info("dummy mode enabled")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go st.Start(ctx)

	if len(cfg.Exports.Pixoo64) > 0 {
		badgeNames := make([]string, len(cfg.Exports.Pixoo64))
		for i, exp := range cfg.Exports.Pixoo64 {
			badgeNames[i] = exp.Badge
		}
		go func() {
			pixooExporter, err := pixoo.New(st, badgeNames, log)
			if err != nil {
				log.Error("Pixoo64 exporter disabled", "err", err)
				return
			}
			pixooExporter.Run(ctx)
		}()
	}

	if len(cfg.Exports.Prometheuses) > 0 {
		go func() {
			promExporter, err := prometheus.New(st, cfg.Exports.Prometheuses, log)
			if err != nil {
				log.Error("Prometheus exports disabled", "err", err)
				return
			}
			promExporter.Run(ctx)
		}()
	}

	var listen atomic.Value
	listen.Store(cfg.Listen)

	reload := func() error {
		newCfg, err := loadConfig(*configPath)
		if err != nil {
			return err
		}
		if newCfg.Listen != listen.Load().(string) {
			log.Warn("listen address change ignored; restart required",
				"current", listen.Load(), "new", newCfg.Listen)
		}
		if err := st.Reload(newCfg); err != nil {
			return err
		}
		return nil
	}

	go func() {
		err := config.Watch(
			ctx,
			*configPath,
			reload,
			log,
		)
		if err != nil && ctx.Err() == nil {
			log.Error("config watcher stopped", "err", err)
		}
	}()

	sighup := make(chan os.Signal, 1)
	signal.Notify(sighup, syscall.SIGHUP)
	go func() {
		for range sighup {
			if err := reload(); err != nil {
				log.Error("SIGHUP reload failed", "err", err)
				continue
			}
			log.Info("config reloaded", "reason", "SIGHUP")
		}
	}()

	srv, err := server.New(st, log, reg)
	if err != nil {
		log.Error("server init failed", "err", err)
		os.Exit(1)
	}

	httpServer := &http.Server{
		Addr:    cfg.Listen,
		Handler: srv.Engine(),
	}

	go func() {
		log.Info("listening", "addr", cfg.Listen)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("listen failed", "err", err)
			os.Exit(1)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Info("shutting down")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown failed", "err", err)
	}
}
