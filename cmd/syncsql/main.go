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
		slog.Error("sync sql stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "", "path to YAML config file")
	sqlPath := flag.String("file", "sql/t_medicated_food.sql", "path to SQL file")
	flag.Parse()

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

	count, err := importer.SyncMedicatedFoodSQL(ctx, db, *sqlPath)
	if err != nil {
		return fmt.Errorf("sync medicated food sql: %w", err)
	}

	logger.Info("sql synced", "file", *sqlPath, "rows", count)
	return nil
}
