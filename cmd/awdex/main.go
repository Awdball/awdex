package main

import (
	"context"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/awdball/awdex/internal/config"
	"github.com/awdball/awdex/internal/db"
	"github.com/awdball/awdex/internal/indexer"
	"github.com/awdball/awdex/internal/server"
	"github.com/awdball/awdex/internal/thumb"
	"github.com/awdball/awdex/web"
)

var version = "1.1.0"

func main() {
	if version != "" {
		server.Version = version
	}
	log.Printf("awdex v%s - modern high-performance file indexer", server.Version)

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

	database, err := db.Open(cfg)
	if err != nil {
		log.Fatalf("failed to initialize database (%s): %v", cfg.DatabaseURL, err)
	}
	defer database.Close()

	thumbs := thumb.NewManager(cfg)
	idx := indexer.New(cfg, database, thumbs)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	idx.Start(ctx)

	var webFS fs.FS
	webFS = web.DistFS

	srv := server.New(cfg, database, thumbs, idx, webFS)

	// Graceful shutdown channel
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := srv.Start(); err != nil && err.Error() != "http: Server closed" {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-stopChan
	log.Println("shutting down awdex server gracefully...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("error during server shutdown: %v", err)
	}
	log.Println("awdex stopped cleanly.")
}
