package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"chinese-medical/internal/config"
	"chinese-medical/internal/database"
	"chinese-medical/internal/importer"
)

func main() {
	if err := run(); err != nil {
		slog.Error("import article stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "", "path to YAML config file")
	htmlPath := flag.String("file", "", "path to saved WeChat article HTML file")
	flag.Parse()

	if *htmlPath == "" {
		return fmt.Errorf("-file is required")
	}

	var (
		cfg config.Config
		err error
	)
	if *configPath != "" {
		cfg, err = config.LoadFile(*configPath)
	} else {
		cfg, err = config.Load()
	}
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))
	slog.SetDefault(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := database.NewPostgresPool(ctx, cfg.Database)
	if err != nil {
		return fmt.Errorf("connect postgres: %w", err)
	}
	defer db.Close()

	result, err := importer.SyncPatentFoodHTML(ctx, db, *htmlPath)
	if err != nil {
		return fmt.Errorf("sync patent food html: %w", err)
	}

	logger.Info("article imported", "file", *htmlPath, "parsed", result.Parsed, "inserted", result.Inserted, "skipped", result.Skipped)
	return nil
}
