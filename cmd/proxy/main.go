package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/your-org/proxy-api/internal/admin"
	"github.com/your-org/proxy-api/internal/config"
	"github.com/your-org/proxy-api/internal/middleware"
	"github.com/your-org/proxy-api/internal/proxy"
)

func main() {
	configPath := flag.String("config", "./configs/config.yaml", "path to config file")
	metricsAddr := flag.String("metrics-addr", ":9090", "Prometheus metrics listen address")
	adminAddr := flag.String("admin-addr", ":15723", "admin UI listen address")
	flag.Parse()

	// Load configuration with hot-reload support
	cfgMgr, err := config.NewConfigManager(*configPath, nil)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	cfg := cfgMgr.Snapshot()

	log.Printf("Config loaded: %d rules, proxy %s → %s",
		len(cfg.Rules), cfg.Proxy.Listen, cfg.Proxy.Upstream)
	log.Printf("Logging enabled: %v", cfg.Logging.Enabled)

	// Create proxy handler
	handler, err := proxy.NewHandler(cfg)
	if err != nil {
		log.Fatalf("Failed to create proxy handler: %v", err)
	}

	// Set up config hot-reload callback
	cfgMgr.SetOnChange(func(newCfg *config.Config) {
		handler.UpdateConfig(newCfg)
	})

	// Start config file watching
	if err := cfgMgr.StartWatching(); err != nil {
		log.Printf("Warning: config hot-reload disabled: %v", err)
	}

	// Build handler chain: RequestID → Proxy
	chain := middleware.RequestID(handler)

	// Setup main proxy server
	srv := &http.Server{
		Addr:         cfg.Proxy.Listen,
		Handler:      chain,
		ReadTimeout:  time.Duration(cfg.Proxy.TimeoutSec) * time.Second,
		WriteTimeout: time.Duration(cfg.Proxy.TimeoutSec) * time.Second,
	}

	// Start proxy server
	go func() {
		log.Printf("Proxy listening on %s", cfg.Proxy.Listen)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Start metrics server (using handler's custom registry)
	metricsHandler := promhttp.HandlerFor(handler.MetricsRegistry(), promhttp.HandlerOpts{})
	metricsSrv := &http.Server{
		Addr:    *metricsAddr,
		Handler: metricsHandler,
	}
	go func() {
		log.Printf("Metrics listening on %s", *metricsAddr)
		if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Metrics server error: %v", err)
		}
	}()

	// Start admin UI server
	adminHandlers := admin.NewHandlers(cfgMgr, handler)
	adminSrv := admin.NewServer(adminHandlers, *adminAddr)
	go func() {
		if err := adminSrv.Start(); err != nil && err != http.ErrServerClosed {
			log.Printf("Admin server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down...")

	// Stop config watching
	cfgMgr.StopWatching()

	// Close proxy resources
	handler.Close()

	// Shutdown servers
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Proxy shutdown error: %v", err)
	}
	if err := metricsSrv.Shutdown(ctx); err != nil {
		log.Printf("Metrics shutdown error: %v", err)
	}

	fmt.Println("Server exited cleanly")
}
