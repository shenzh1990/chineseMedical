package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"chinese-medical/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MedicatedFoodRepository struct {
	db *pgxpool.Pool
}

func NewMedicatedFoodRepository(db *pgxpool.Pool) MedicatedFoodRepository {
	return MedicatedFoodRepository{db: db}
}

func (r MedicatedFoodRepository) Get(ctx context.Context, id int64) (model.MedicatedFood, error) {
	var item model.MedicatedFood
	if err := r.db.QueryRow(ctx, `
		SELECT id, COALESCE(NULLIF(btrim(category), ''), '药食同源'), name, source, food, method, effect
		FROM t_medicated_food
		WHERE id = $1
	`, id).Scan(&item.ID, &item.Category, &item.Name, &item.Source, &item.Food, &item.Method, &item.Effect); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.MedicatedFood{}, fmt.Errorf("medicated food %d not found: %w", id, err)
		}
		return model.MedicatedFood{}, fmt.Errorf("get medicated food: %w", err)
	}
	return item, nil
}

func (r MedicatedFoodRepository) List(ctx context.Context, query, category string, limit, offset int) ([]model.MedicatedFood, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	query = strings.TrimSpace(query)
	category = strings.TrimSpace(category)
	args := []any{}
	whereParts := []string{}
	if query != "" {
		args = append(args, "%"+query+"%")
		whereParts = append(whereParts, fmt.Sprintf("(name ILIKE $%d OR COALESCE(NULLIF(btrim(category), ''), '药食同源') ILIKE $%d OR source ILIKE $%d OR food ILIKE $%d OR effect ILIKE $%d)", len(args), len(args), len(args), len(args), len(args)))
	}
	if category != "" {
		args = append(args, category)
		whereParts = append(whereParts, fmt.Sprintf("COALESCE(NULLIF(btrim(category), ''), '药食同源') = $%d", len(args)))
	}

	where := ""
	if len(whereParts) > 0 {
		where = "WHERE " + strings.Join(whereParts, " AND ")
	}

	countSQL := "SELECT COUNT(*) FROM t_medicated_food " + where
	var total int64
	if err := r.db.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count medicated foods: %w", err)
	}

	limitArg := len(args) + 1
	offsetArg := len(args) + 2
	args = append(args, limit, offset)

	listSQL := fmt.Sprintf(`
		SELECT id, COALESCE(NULLIF(btrim(category), ''), '药食同源'), name, source, food, method, effect
		FROM t_medicated_food
		%s
		ORDER BY COALESCE(NULLIF(btrim(category), ''), '药食同源'), id
		LIMIT $%d OFFSET $%d
	`, where, limitArg, offsetArg)

	rows, err := r.db.Query(ctx, listSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query medicated foods: %w", err)
	}
	defer rows.Close()

	items := make([]model.MedicatedFood, 0, limit)
	for rows.Next() {
		var item model.MedicatedFood
		if err := rows.Scan(&item.ID, &item.Category, &item.Name, &item.Source, &item.Food, &item.Method, &item.Effect); err != nil {
			return nil, 0, fmt.Errorf("scan medicated food: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate medicated foods: %w", err)
	}

	return items, total, nil
}

func (r MedicatedFoodRepository) Categories(ctx context.Context) ([]model.FoodCategorySummary, error) {
	rows, err := r.db.Query(ctx, `
		SELECT COALESCE(NULLIF(btrim(category), ''), '药食同源') AS category, COUNT(*)
		FROM t_medicated_food
		GROUP BY COALESCE(NULLIF(btrim(category), ''), '药食同源')
		ORDER BY category
	`)
	if err != nil {
		return nil, fmt.Errorf("query medicated food categories: %w", err)
	}
	defer rows.Close()

	categories := []model.FoodCategorySummary{}
	for rows.Next() {
		var category model.FoodCategorySummary
		if err := rows.Scan(&category.Name, &category.Count); err != nil {
			return nil, fmt.Errorf("scan medicated food category: %w", err)
		}
		categories = append(categories, category)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate medicated food categories: %w", err)
	}

	return categories, nil
}

func (r MedicatedFoodRepository) Related(ctx context.Context, keywords []string, category string, limit int) ([]model.MedicatedFood, error) {
	if limit <= 0 {
		limit = 5
	}
	category = strings.TrimSpace(category)

	args := []any{}
	whereParts := []string{}
	keywordWhereParts := []string{}
	if category != "" {
		args = append(args, category)
		whereParts = append(whereParts, fmt.Sprintf("COALESCE(NULLIF(btrim(category), ''), '药食同源') = $%d", len(args)))
	}

	scoreParts := []string{}
	for _, keyword := range keywords {
		keyword = strings.TrimSpace(keyword)
		if keyword == "" {
			continue
		}
		args = append(args, "%"+keyword+"%")
		argIndex := len(args)
		keywordWhereParts = append(keywordWhereParts, fmt.Sprintf("(name ILIKE $%d OR source ILIKE $%d OR food ILIKE $%d OR effect ILIKE $%d)", argIndex, argIndex, argIndex, argIndex))
		scoreParts = append(scoreParts, fmt.Sprintf(`
			(CASE WHEN name ILIKE $%d THEN 8 ELSE 0 END) +
			(CASE WHEN effect ILIKE $%d THEN 5 ELSE 0 END) +
			(CASE WHEN food ILIKE $%d THEN 3 ELSE 0 END) +
			(CASE WHEN source ILIKE $%d THEN 1 ELSE 0 END)
		`, argIndex, argIndex, argIndex, argIndex))
		if len(scoreParts) >= 8 {
			break
		}
	}
	if len(scoreParts) == 0 {
		return nil, nil
	}
	whereParts = append(whereParts, "("+strings.Join(keywordWhereParts, " OR ")+")")

	where := "WHERE " + strings.Join(whereParts, " AND ")
	scoreSQL := strings.Join(scoreParts, " + ")
	limitArg := len(args) + 1
	args = append(args, limit)

	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT id, COALESCE(NULLIF(btrim(category), ''), '药食同源'), name, source, food, method, effect
		FROM t_medicated_food
		%s
		ORDER BY (%s) DESC, id
		LIMIT $%d
	`, where, scoreSQL, limitArg), args...)
	if err != nil {
		return nil, fmt.Errorf("query related medicated foods: %w", err)
	}
	defer rows.Close()

	items := []model.MedicatedFood{}
	for rows.Next() {
		var item model.MedicatedFood
		if err := rows.Scan(&item.ID, &item.Category, &item.Name, &item.Source, &item.Food, &item.Method, &item.Effect); err != nil {
			return nil, fmt.Errorf("scan related medicated food: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate related medicated foods: %w", err)
	}

	return items, nil
}

func (r MedicatedFoodRepository) Create(ctx context.Context, item model.MedicatedFood) (model.MedicatedFood, error) {
	item.Category = model.NormalizeFoodCategory(item.Category)
	if item.ID > 0 {
		if err := r.db.QueryRow(ctx, `
			INSERT INTO t_medicated_food (id, category, name, source, food, method, effect)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING id
		`, item.ID, item.Category, item.Name, item.Source, item.Food, item.Method, item.Effect).Scan(&item.ID); err != nil {
			return model.MedicatedFood{}, fmt.Errorf("create medicated food: %w", err)
		}
		return item, nil
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return model.MedicatedFood{}, fmt.Errorf("begin create medicated food: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('t_medicated_food_next_id'))`); err != nil {
		return model.MedicatedFood{}, fmt.Errorf("lock medicated food id sequence: %w", err)
	}

	if err := tx.QueryRow(ctx, `
		WITH next_id AS (
			SELECT COALESCE(MAX(id), 0) + 1 AS id
			FROM t_medicated_food
		)
		INSERT INTO t_medicated_food (id, category, name, source, food, method, effect)
		SELECT next_id.id, $1, $2, $3, $4, $5, $6
		FROM next_id
		RETURNING id
	`, item.Category, item.Name, item.Source, item.Food, item.Method, item.Effect).Scan(&item.ID); err != nil {
		return model.MedicatedFood{}, fmt.Errorf("create medicated food: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return model.MedicatedFood{}, fmt.Errorf("commit create medicated food: %w", err)
	}

	return item, nil
}
