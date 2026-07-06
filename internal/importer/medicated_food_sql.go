package importer

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"chinese-medical/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const medicatedFoodSchema = `
CREATE TABLE IF NOT EXISTS t_medicated_food (
    id BIGINT PRIMARY KEY,
    category TEXT NOT NULL DEFAULT '药食同源',
    name TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT '',
    food TEXT NOT NULL DEFAULT '',
    method TEXT NOT NULL DEFAULT '',
    effect TEXT NOT NULL DEFAULT '',
    create_by TEXT,
    create_time TIMESTAMPTZ,
    update_by TEXT,
    update_time TIMESTAMPTZ
);
`

func SyncMedicatedFoodSQL(ctx context.Context, db *pgxpool.Pool, path string) (int, error) {
	items, err := ParseMedicatedFoodSQL(path)
	if err != nil {
		return 0, err
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	exists, err := medicatedFoodTableExists(ctx, tx)
	if err != nil {
		return 0, fmt.Errorf("check t_medicated_food: %w", err)
	}
	if !exists {
		if _, err := tx.Exec(ctx, medicatedFoodSchema); err != nil {
			return 0, fmt.Errorf("create t_medicated_food: %w", err)
		}
	}
	if err := ensureMedicatedFoodCategory(ctx, tx); err != nil {
		return 0, err
	}

	batch := &pgx.Batch{}
	for _, item := range items {
		item.Category = model.NormalizeFoodCategory(item.Category)
		batch.Queue(`
			INSERT INTO t_medicated_food (id, category, name, source, food, method, effect)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (id) DO UPDATE SET
				category = EXCLUDED.category,
				name = EXCLUDED.name,
				source = EXCLUDED.source,
				food = EXCLUDED.food,
				method = EXCLUDED.method,
				effect = EXCLUDED.effect
		`, item.ID, item.Category, item.Name, item.Source, item.Food, item.Method, item.Effect)
	}

	results := tx.SendBatch(ctx, batch)
	for range items {
		if _, err := results.Exec(); err != nil {
			_ = results.Close()
			return 0, fmt.Errorf("upsert medicated food: %w", err)
		}
	}
	if err := results.Close(); err != nil {
		return 0, fmt.Errorf("close batch: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit transaction: %w", err)
	}

	return len(items), nil
}

func medicatedFoodTableExists(ctx context.Context, tx pgx.Tx) (bool, error) {
	var exists bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = current_schema()
				AND table_name = 't_medicated_food'
		)
	`).Scan(&exists); err != nil {
		return false, fmt.Errorf("query table existence: %w", err)
	}
	return exists, nil
}

func ensureMedicatedFoodCategory(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx, `
		ALTER TABLE t_medicated_food
		ADD COLUMN IF NOT EXISTS category TEXT NOT NULL DEFAULT '药食同源'
	`); err != nil {
		return fmt.Errorf("add medicated food category: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE t_medicated_food
		SET category = $1
		WHERE category IS NULL OR btrim(category) = ''
	`, model.DefaultFoodCategory); err != nil {
		return fmt.Errorf("backfill medicated food category: %w", err)
	}
	return nil
}

func ParseMedicatedFoodSQL(path string) ([]model.MedicatedFood, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open sql file %q: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), 1024*1024)

	items := []model.MedicatedFood{}
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		fields, err := parseInsertValues(line)
		if err != nil {
			return nil, fmt.Errorf("parse line %d: %w", lineNumber, err)
		}
		if len(fields) < 6 {
			return nil, fmt.Errorf("parse line %d: expected at least 6 fields, got %d", lineNumber, len(fields))
		}

		id, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse line %d id: %w", lineNumber, err)
		}

		items = append(items, model.MedicatedFood{
			ID:       id,
			Category: model.DefaultFoodCategory,
			Name:     fields[1],
			Source:   fields[2],
			Food:     fields[3],
			Method:   fields[4],
			Effect:   fields[5],
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan sql file: %w", err)
	}

	return items, nil
}

func parseInsertValues(line string) ([]string, error) {
	start := strings.Index(line, "VALUES (")
	if start < 0 {
		return nil, fmt.Errorf("missing VALUES clause")
	}

	values := strings.TrimSpace(line[start+len("VALUES "):])
	values = strings.TrimSuffix(values, ";")
	if !strings.HasPrefix(values, "(") || !strings.HasSuffix(values, ")") {
		return nil, fmt.Errorf("invalid VALUES tuple")
	}

	values = strings.TrimPrefix(strings.TrimSuffix(values, ")"), "(")
	fields := []string{}
	var current strings.Builder
	inString := false

	for i := 0; i < len(values); i++ {
		ch := values[i]
		switch ch {
		case '\'':
			if inString && i+1 < len(values) && values[i+1] == '\'' {
				current.WriteByte('\'')
				i++
				continue
			}
			inString = !inString
		case ',':
			if !inString {
				fields = append(fields, cleanSQLValue(current.String()))
				current.Reset()
				continue
			}
			current.WriteByte(ch)
		case '\\':
			if inString && i+1 < len(values) {
				i++
				current.WriteByte(values[i])
				continue
			}
			current.WriteByte(ch)
		default:
			current.WriteByte(ch)
		}
	}

	if inString {
		return nil, fmt.Errorf("unterminated string")
	}
	fields = append(fields, cleanSQLValue(current.String()))

	return fields, nil
}

func cleanSQLValue(value string) string {
	value = strings.TrimSpace(value)
	if strings.EqualFold(value, "NULL") {
		return ""
	}
	return value
}
