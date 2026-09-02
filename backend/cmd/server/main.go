package main

import (
	"context"
	"crypto/tls"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"golang.org/x/crypto/acme/autocert"

	"videocms/backend/internal/api"
	"videocms/backend/internal/config"
	"videocms/backend/internal/db"
)

func main() {
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		log.Fatalf("run migrations: %v", err)
	}
	if err := db.SeedAdmin(ctx, pool, cfg.AdminUsername, cfg.AdminPassword); err != nil {
		log.Fatalf("seed admin: %v", err)
	}

	app, err := api.New(cfg, pool)
	if err != nil {
		log.Fatalf("init api: %v", err)
	}
	app.StartFileWatcher(ctx)
	app.StartShareCleanup(ctx)
	app.StartDownloadWorker(ctx)
	app.StartMaintenance(ctx)
	app.StartDLNA(ctx)
	app.StartRecorder(ctx)
	app.StartTranscoder(ctx)

	tlsConfig := (*tls.Config)(nil)
	if len(cfg.AutoTLSDomains) > 0 {
		certDir := filepath.Join(cfg.DataDir, "certs")
		if err := os.MkdirAll(certDir, 0o755); err != nil {
			log.Fatalf("create cert cache dir: %v", err)
		}
		m := &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(cfg.AutoTLSDomains...),
			Cache:      autocert.DirCache(certDir),
		}
		go func() {
			log.Printf("autocert ACME challenge listener on :80 for %v", cfg.AutoTLSDomains)
			if err := http.ListenAndServe(":80", m.HTTPHandler(nil)); err != nil && err != http.ErrServerClosed {
				log.Printf("autocert :80 listener: %v", err)
			}
		}()
		tlsConfig = &tls.Config{
			GetCertificate: m.GetCertificate,
			MinVersion:     tls.VersionTLS12,
		}
	} else if cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
		if err != nil {
			log.Fatalf("load TLS certificate: %v", err)
		}
		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}
	}

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           app.Routes(),
		TLSConfig:         tlsConfig,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		mode := "http"
		if tlsConfig != nil {
			mode = "https"
		}
		log.Printf("videocms server listening on %s (%s)", cfg.Addr, mode)
		var err error
		if tlsConfig != nil {
			err = srv.ListenAndServeTLS("", "")
		} else {
			err = srv.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
