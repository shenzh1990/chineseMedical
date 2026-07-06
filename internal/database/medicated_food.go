package database

import (
	"context"
	"fmt"

	"chinese-medical/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

func EnsureMedicatedFoodSchema(ctx context.Context, db *pgxpool.Pool) error {
	var exists bool
	if err := db.QueryRow(ctx, `SELECT to_regclass('t_medicated_food') IS NOT NULL`).Scan(&exists); err != nil {
		return fmt.Errorf("check medicated food table: %w", err)
	}
	if !exists {
		return nil
	}

	if _, err := db.Exec(ctx, `
		ALTER TABLE t_medicated_food
		ADD COLUMN IF NOT EXISTS category TEXT NOT NULL DEFAULT '药食同源'
	`); err != nil {
		return fmt.Errorf("add medicated food category: %w", err)
	}

	if _, err := db.Exec(ctx, `
		UPDATE t_medicated_food
		SET category = $1
		WHERE category IS NULL OR btrim(category) = ''
	`, model.DefaultFoodCategory); err != nil {
		return fmt.Errorf("backfill medicated food category: %w", err)
	}

	return nil
}
