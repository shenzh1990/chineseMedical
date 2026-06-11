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
		SELECT id, name, source, food, method, effect
		FROM t_medicated_food
		WHERE id = $1
	`, id).Scan(&item.ID, &item.Name, &item.Source, &item.Food, &item.Method, &item.Effect); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.MedicatedFood{}, fmt.Errorf("medicated food %d not found: %w", id, err)
		}
		return model.MedicatedFood{}, fmt.Errorf("get medicated food: %w", err)
	}
	return item, nil
}

func (r MedicatedFoodRepository) List(ctx context.Context, query string, limit, offset int) ([]model.MedicatedFood, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	query = strings.TrimSpace(query)
	args := []any{}
	where := ""
	if query != "" {
		args = append(args, "%"+query+"%")
		where = "WHERE name ILIKE $1 OR source ILIKE $1 OR food ILIKE $1 OR effect ILIKE $1"
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
		SELECT id, name, source, food, method, effect
		FROM t_medicated_food
		%s
		ORDER BY id
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
		if err := rows.Scan(&item.ID, &item.Name, &item.Source, &item.Food, &item.Method, &item.Effect); err != nil {
			return nil, 0, fmt.Errorf("scan medicated food: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate medicated foods: %w", err)
	}

	return items, total, nil
}
