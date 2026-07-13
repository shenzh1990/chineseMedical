package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func EnsureSkillHubSchema(ctx context.Context, db *pgxpool.Pool) error {
	if _, err := db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS ai_skills (
			id BIGSERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			slug TEXT NOT NULL UNIQUE,
			category TEXT NOT NULL DEFAULT '通用',
			description TEXT NOT NULL DEFAULT '',
			instruction TEXT NOT NULL DEFAULT '',
			tags TEXT NOT NULL DEFAULT '',
			enabled BOOLEAN NOT NULL DEFAULT TRUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`); err != nil {
		return fmt.Errorf("create ai_skills table: %w", err)
	}

	if _, err := db.Exec(ctx, `
		CREATE INDEX IF NOT EXISTS idx_ai_skills_slug ON ai_skills(slug);
		CREATE INDEX IF NOT EXISTS idx_ai_skills_category ON ai_skills(category);
		CREATE INDEX IF NOT EXISTS idx_ai_skills_enabled ON ai_skills(enabled);
	`); err != nil {
		return fmt.Errorf("create ai_skills indexes: %w", err)
	}

	if _, err := db.Exec(ctx, `
		CREATE OR REPLACE FUNCTION update_ai_skills_updated_at()
		RETURNS TRIGGER AS $$
		BEGIN
			NEW.updated_at = now();
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
	`); err != nil {
		return fmt.Errorf("create ai_skills updated_at function: %w", err)
	}

	if _, err := db.Exec(ctx, `
		DROP TRIGGER IF EXISTS trg_ai_skills_updated_at ON ai_skills;
		CREATE TRIGGER trg_ai_skills_updated_at
		BEFORE UPDATE ON ai_skills
		FOR EACH ROW
		EXECUTE FUNCTION update_ai_skills_updated_at();
	`); err != nil {
		return fmt.Errorf("create ai_skills updated_at trigger: %w", err)
	}

	return nil
}
