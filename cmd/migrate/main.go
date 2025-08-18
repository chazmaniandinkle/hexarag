package main

import (
	"context"
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/username/hexarag/internal/adapters/storage/sqlite"
	"github.com/username/hexarag/pkg/config"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Running database migrations for: %s", cfg.Database.Path)

	// Ensure database directory exists
	err = os.MkdirAll(filepath.Dir(cfg.Database.Path), 0755)
	if err != nil {
		log.Fatalf("Failed to create database directory: %v", err)
	}

	// Initialize storage adapter
	storage, err := sqlite.NewAdapter(cfg.Database.Path, cfg.Database.MigrationsPath)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer storage.Close()

	// Run migrations
	ctx := context.Background()
	if err := storage.Migrate(ctx); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	log.Println("Migrations completed successfully")
}
